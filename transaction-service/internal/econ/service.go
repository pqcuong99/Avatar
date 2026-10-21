// Package econ is the Transaction Service: the only code that changes Xu,
// Diamonds, items, farm plots and pending lines (P0-165, docs/05 §6).
//
// Every command runs as one PostgreSQL transaction:
//
//	BEGIN
//	  lock the character row               -> commands of one character run one at a time
//	  look up (character, request_id)      -> replay the stored outcome, or IDEMPOTENCY_CONFLICT
//	  read the published config version
//	  SAVEPOINT; run the business rule; write bag/wallet/farm rows
//	    business rejection -> ROLLBACK TO SAVEPOINT, store the rejection
//	  INSERT transaction_requests (final outcome) + asset_ledger rows
//	COMMIT
//
// The caller learns the outcome only after COMMIT, so nothing is shown to the
// player before the change is durable (docs/01 §4).
package econ

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/avatar/transaction-service/internal/apperr"
	"example.com/avatar/transaction-service/internal/farm"
	"example.com/avatar/transaction-service/internal/inventory"
)

type Service struct {
	pool    *pgxpool.Pool
	catalog *Catalog
	// Now and Rand are replaceable in tests.
	Now  func() time.Time
	Rand farm.Rand
	// RequireSyncStandby turns on the P0-150 guard: refuse asset writes while
	// no synchronous standby confirms durability.
	RequireSyncStandby bool
}

func New(ctx context.Context, pool *pgxpool.Pool) (*Service, error) {
	cat, err := loadCatalog(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	return &Service{
		pool:    pool,
		catalog: cat,
		Now:     time.Now,
		Rand:    mrand.New(mrand.NewChaCha8(seed())),
	}, nil
}

func seed() [32]byte {
	var s [32]byte
	if _, err := rand.Read(s[:]); err != nil {
		panic(err)
	}
	return s
}

func (s *Service) Catalog() *Catalog { return s.catalog }

func (s *Service) now() time.Time { return s.Now().UTC().Truncate(time.Microsecond) }

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IsUUID(s string) bool { return uuidPattern.MatchString(s) }

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Outcome of a committed command.
type Outcome struct {
	Result   json.RawMessage `json:"result"`
	Replayed bool            `json:"replayed"`
}

type storedResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *apperr.Error   `json:"error,omitempty"`
	Status int             `json:"status,omitempty"`
}

// Execute runs one asset command for a character. fn returns the result to
// store and return, or an *apperr.Error to reject the command.
func (s *Service) Execute(ctx context.Context, characterID, requestID, op string, payload any,
	fn func(w *Work) (any, error)) (*Outcome, error) {
	if !IsUUID(characterID) {
		return nil, apperr.NotFound
	}
	if !requestIDPattern.MatchString(requestID) {
		return nil, apperr.Validation.With(map[string]any{"field": "request_id"})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append([]byte(op+"\x00"), body...))

	var out *Outcome
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		out = nil
		var slotCap int
		err := tx.QueryRow(ctx, `SELECT inventory_slot_cap FROM characters WHERE id = $1 FOR UPDATE`, characterID).Scan(&slotCap)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound
		}
		if err != nil {
			return err
		}

		var prevOp, state string
		var prevDigest []byte
		var stored storedResponse
		err = tx.QueryRow(ctx, `SELECT operation, payload_digest, state, response FROM transaction_requests
			WHERE character_id = $1 AND request_id = $2`, characterID, requestID).Scan(&prevOp, &prevDigest, &state, &stored)
		switch {
		case err == nil:
			if prevOp != op || !bytesEqual(prevDigest, digest[:]) {
				return apperr.IdempotencyConflict
			}
			if state == "rejected" && stored.Error != nil {
				e := *stored.Error
				e.Status = stored.Status
				return &e
			}
			out = &Outcome{Result: canonicalJSON(stored.Result), Replayed: true}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		if err := s.checkDurability(ctx, tx); err != nil {
			return err
		}
		if err := s.checkMaintenance(ctx, tx, op); err != nil {
			return err
		}
		cfg, err := loadPublishedConfig(ctx, tx)
		if err != nil {
			return err
		}

		txID := newUUID()
		w := &Work{ctx: ctx, tx: tx, s: s, Now: s.now(), Config: cfg, CharacterID: characterID, slotCap: slotCap, requestID: requestID, txID: txID}
		sp, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		result, fnErr := fn(w)
		if fnErr == nil {
			fnErr = w.flush()
		}
		var resp storedResponse
		var reqState string
		if fnErr != nil {
			biz, ok := apperr.As(fnErr)
			if !ok {
				return fnErr
			}
			if err := sp.Rollback(ctx); err != nil {
				return err
			}
			w.ledger = nil
			resp = storedResponse{Error: biz, Status: biz.Status}
			reqState = "rejected"
		} else {
			if err := sp.Commit(ctx); err != nil {
				return err
			}
			raw, err := json.Marshal(result)
			if err != nil {
				return err
			}
			resp = storedResponse{Result: raw}
			reqState = "committed"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO transaction_requests
			(id, character_id, request_id, operation, payload_digest, state, response, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			txID, characterID, requestID, op, digest[:], reqState, resp, w.Now); err != nil {
			return err
		}
		if err := w.writeLedger(txID); err != nil {
			return err
		}
		if reqState == "rejected" {
			// Commit the stored rejection, then report it.
			return &rejected{resp.Error}
		}
		out = &Outcome{Result: canonicalJSON(resp.Result)}
		return nil
	})
	var rj *rejected
	if errors.As(err, &rj) {
		return nil, rj.err
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// canonicalJSON gives a fresh result and its replay (read back from JSONB,
// which reorders keys) the same bytes: compact, keys sorted.
func canonicalJSON(raw json.RawMessage) json.RawMessage {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// rejected carries a business rejection out of withTx while still committing
// the transaction that recorded it.
type rejected struct{ err *apperr.Error }

func (r *rejected) Error() string { return r.err.Error() }

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// withTx runs fn in a READ COMMITTED transaction and retries on serialization
// failures and deadlocks. A *rejected result is committed.
func (s *Service) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		err = s.tryTx(ctx, fn)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
			continue
		}
		return err
	}
	return err
}

func (s *Service) tryTx(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			var rj *rejected
			if errors.As(err, &rj) {
				if cerr := tx.Commit(ctx); cerr != nil {
					err = cerr
				}
				return
			}
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// checkDurability implements P0-150 when enabled: with no synchronous standby
// confirming writes, asset changes stop instead of risking lost transactions.
func (s *Service) checkDurability(ctx context.Context, tx pgx.Tx) error {
	if !s.RequireSyncStandby {
		return nil
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pg_stat_replication WHERE sync_state IN ('sync', 'quorum')`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return apperr.AssetWritesDisabled
	}
	return nil
}

// Work is the state of one command inside its transaction.
type Work struct {
	ctx         context.Context
	tx          pgx.Tx
	s           *Service
	Now         time.Time
	Config      *Config
	CharacterID string

	requestID string
	txID      string // id of this command's transaction_requests row
	slotCap   int
	bag       *inventory.Bag
	wallets   map[string]*wallet
	ledger    []ledgerEntry
}

type wallet struct {
	balance int64
	cap     *int64 // NULL in the row: cap comes from config
	changed bool
}

type ledgerEntry struct {
	kind, ref, location, reason string
	delta, after                int64
}

func (w *Work) Tx() pgx.Tx           { return w.tx }
func (w *Work) Ctx() context.Context { return w.ctx }
func (w *Work) Catalog() *Catalog    { return w.s.catalog }
func (w *Work) Rand() farm.Rand      { return w.s.Rand }
func (w *Work) SlotCap() int         { return w.slotCap }

func (w *Work) SetSlotCap(n int) {
	w.slotCap = n
	if w.bag != nil {
		w.bag.Cap = n
	}
}

func (w *Work) Exec(sql string, args ...any) error {
	_, err := w.tx.Exec(w.ctx, sql, args...)
	return err
}

// Bag loads the bag once per command.
func (w *Work) Bag() (*inventory.Bag, error) {
	if w.bag != nil {
		return w.bag, nil
	}
	rows, err := w.tx.Query(w.ctx, `SELECT slot_no, item_code, quantity, COALESCE(item_instance_id::text, '')
		FROM inventory_slots WHERE character_id = $1`, w.CharacterID)
	if err != nil {
		return nil, err
	}
	slots, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (inventory.Slot, error) {
		var s inventory.Slot
		err := r.Scan(&s.No, &s.ItemCode, &s.Quantity, &s.InstanceID)
		return s, err
	})
	if err != nil {
		return nil, err
	}
	w.bag = inventory.New(w.slotCap, slots)
	return w.bag, nil
}

func (w *Work) wallet(currency string) (*wallet, error) {
	if w.wallets == nil {
		w.wallets = map[string]*wallet{}
		rows, err := w.tx.Query(w.ctx, `SELECT currency, balance, wallet_cap FROM character_wallets
			WHERE character_id = $1 FOR UPDATE`, w.CharacterID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var cur string
			wl := &wallet{}
			if err := rows.Scan(&cur, &wl.balance, &wl.cap); err != nil {
				rows.Close()
				return nil, err
			}
			w.wallets[cur] = wl
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	wl, ok := w.wallets[currency]
	if !ok {
		return nil, fmt.Errorf("character %s has no %s wallet", w.CharacterID, currency)
	}
	return wl, nil
}

// Balance returns the current (in-command) balance.
func (w *Work) Balance(currency string) (int64, error) {
	wl, err := w.wallet(currency)
	if err != nil {
		return 0, err
	}
	return wl.balance, nil
}

// WalletCap is the effective cap: the row's own cap, else published config.
// 0 means "not configured".
func (w *Work) WalletCap(currency string) (int64, error) {
	wl, err := w.wallet(currency)
	if err != nil {
		return 0, err
	}
	if wl.cap != nil {
		return *wl.cap, nil
	}
	if currency == "XU" {
		return w.Config.XuCap, nil
	}
	return w.Config.DiamondCap, nil
}

func (w *Work) Debit(currency string, amount int64, reason string) error {
	if amount <= 0 {
		return fmt.Errorf("debit amount must be positive, got %d", amount)
	}
	wl, err := w.wallet(currency)
	if err != nil {
		return err
	}
	if wl.balance < amount {
		return apperr.InsufficientFunds.With(map[string]any{"currency": currency, "required": amount, "balance": wl.balance})
	}
	wl.balance -= amount
	wl.changed = true
	w.ledger = append(w.ledger, ledgerEntry{kind: currency, location: "WALLET", reason: reason, delta: -amount, after: wl.balance})
	return nil
}

// Credit adds money. Going over the cap is refused, never truncated: the rule
// for Xu over the cap is not decided (GAP-01), so nothing is destroyed.
func (w *Work) Credit(currency string, amount int64, reason string) error {
	if amount <= 0 {
		return fmt.Errorf("credit amount must be positive, got %d", amount)
	}
	wl, err := w.wallet(currency)
	if err != nil {
		return err
	}
	limit, err := w.WalletCap(currency)
	if err != nil {
		return err
	}
	if limit <= 0 {
		return apperr.NotConfigured.With(map[string]any{"config": "wallet." + currencyKey(currency) + "_cap"})
	}
	if wl.balance+amount > limit {
		return apperr.WalletCapExceeded.With(map[string]any{"currency": currency, "cap": limit, "balance": wl.balance, "amount": amount})
	}
	wl.balance += amount
	wl.changed = true
	w.ledger = append(w.ledger, ledgerEntry{kind: currency, location: "WALLET", reason: reason, delta: amount, after: wl.balance})
	return nil
}

func currencyKey(c string) string {
	if c == "XU" {
		return "xu"
	}
	return "diamond"
}

// AddItem puts a whole quantity of a stackable item in the bag or fails with
// INVENTORY_CAPACITY_INSUFFICIENT without changing anything.
func (w *Work) AddItem(code string, qty int64, reason string) error {
	item, ok := w.s.catalog.Items[code]
	if !ok {
		return fmt.Errorf("unknown item %q", code)
	}
	bag, err := w.Bag()
	if err != nil {
		return err
	}
	if err := bag.Add(item.inv(), qty); err != nil {
		if errors.Is(err, inventory.ErrNoSpace) {
			return apperr.InventoryCapacity.With(map[string]any{"item_code": code, "required": qty, "free_capacity": bag.FreeCapacity(item.inv())})
		}
		return err
	}
	w.ledger = append(w.ledger, ledgerEntry{kind: "ITEM", ref: code, location: "BAG", reason: reason, delta: qty, after: bag.Count(code)})
	return nil
}

func (w *Work) RemoveItem(code string, qty int64, reason string) error {
	bag, err := w.Bag()
	if err != nil {
		return err
	}
	if err := bag.Remove(code, qty); err != nil {
		if errors.Is(err, inventory.ErrNotEnough) {
			return apperr.ItemInsufficient.With(map[string]any{"item_code": code, "required": qty, "held": bag.Count(code)})
		}
		return err
	}
	w.ledger = append(w.ledger, ledgerEntry{kind: "ITEM", ref: code, location: "BAG", reason: reason, delta: -qty, after: bag.Count(code)})
	return nil
}

// LogItem records an item movement that does not go through the bag model
// (pending lines).
func (w *Work) LogItem(code, location string, delta, after int64, reason string) {
	w.ledger = append(w.ledger, ledgerEntry{kind: "ITEM", ref: code, location: location, reason: reason, delta: delta, after: after})
}

func (w *Work) LogDiamondPending(delta, after int64, reason string) {
	w.ledger = append(w.ledger, ledgerEntry{kind: "DIAMOND", location: "PENDING_DIAMOND", reason: reason, delta: delta, after: after})
}

func (w *Work) flush() error {
	if w.bag != nil {
		upserts, deletes := w.bag.Changes()
		for _, no := range deletes {
			if err := w.Exec(`DELETE FROM inventory_slots WHERE character_id = $1 AND slot_no = $2`, w.CharacterID, no); err != nil {
				return err
			}
		}
		for _, s := range upserts {
			var inst any
			if s.InstanceID != "" {
				inst = s.InstanceID
			}
			if err := w.Exec(`INSERT INTO inventory_slots (character_id, slot_no, item_code, quantity, item_instance_id, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (character_id, slot_no) DO UPDATE
				SET item_code = EXCLUDED.item_code, quantity = EXCLUDED.quantity,
				    item_instance_id = EXCLUDED.item_instance_id, updated_at = EXCLUDED.updated_at`,
				w.CharacterID, s.No, s.ItemCode, s.Quantity, inst, w.Now); err != nil {
				return err
			}
		}
	}
	for cur, wl := range w.wallets {
		if !wl.changed {
			continue
		}
		if err := w.Exec(`UPDATE character_wallets SET balance = $3, version = version + 1, updated_at = $4
			WHERE character_id = $1 AND currency = $2`, w.CharacterID, cur, wl.balance, w.Now); err != nil {
			return err
		}
	}
	return nil
}

func (w *Work) writeLedger(txID string) error {
	for _, e := range w.ledger {
		var ref any
		if e.ref != "" {
			ref = e.ref
		}
		if err := w.Exec(`INSERT INTO asset_ledger
			(transaction_id, character_id, asset_kind, asset_ref, location, amount_delta, balance_after, reason_code, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			txID, w.CharacterID, e.kind, ref, e.location, e.delta, e.after, e.reason, w.Now); err != nil {
			return err
		}
	}
	return nil
}
