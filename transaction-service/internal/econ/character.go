package econ

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/avatar/transaction-service/internal/apperr"
	"example.com/avatar/transaction-service/internal/names"
)

// Character is the public summary of a character.
type Character struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"account_id"`
	DisplayName       string    `json:"display_name"`
	InventorySlotCap  int       `json:"inventory_slot_cap"`
	AppearanceVersion int64     `json:"appearance_version"`
	CreatedAt         time.Time `json:"created_at"`
}

const opCreateCharacter = "character.create"

// CreateCharacter creates the one character of an account together with its
// starting assets, in a single transaction (docs/06 §4, P0-018/030/031/068):
// 5,000 Xu, 0 Diamond, a 10-slot empty bag and plots 1..6. Any failure rolls
// everything back. Concurrent calls for one account serialize on the account
// row, so exactly one character and one starter grant exist (A-02).
//
// Replaying the same request_id after success returns the same character.
func (s *Service) CreateCharacter(ctx context.Context, accountID, rawName, requestID string) (*Character, bool, error) {
	if !IsUUID(accountID) {
		return nil, false, apperr.NotFound
	}
	if !requestIDPattern.MatchString(requestID) {
		return nil, false, apperr.Validation.With(map[string]any{"field": "request_id"})
	}
	name, problem := names.Validate(rawName)
	if problem != "" {
		return nil, false, apperr.CharacterNameInvalid.With(map[string]any{
			"problem": problem, "min": names.MinGraphemes, "max": names.MaxGraphemes,
		})
	}

	var out *Character
	var replayed bool
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		out, replayed = nil, false
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound
		}
		if err != nil {
			return err
		}

		existing, err := characterByAccount(ctx, tx, accountID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if existing != nil {
			var op string
			err := tx.QueryRow(ctx, `SELECT operation FROM transaction_requests WHERE character_id = $1 AND request_id = $2`,
				existing.ID, requestID).Scan(&op)
			if err == nil && op == opCreateCharacter {
				out, replayed = existing, true
				return nil
			}
			return apperr.CharacterExists
		}

		if on, err := maintenanceActive(ctx, tx); err != nil {
			return err
		} else if on {
			return errMaintenance
		}
		cfg, err := loadPublishedConfig(ctx, tx)
		if err != nil {
			return err
		}
		now := s.now()
		id := newUUID()
		_, err = tx.Exec(ctx, `INSERT INTO characters (id, account_id, display_name, display_name_key, search_key,
			inventory_slot_cap, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
			id, accountID, name.Display, name.Key, name.SearchKey, StartingSlots, now)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "characters_display_name_key_key" {
				return apperr.CharacterNameTaken
			}
			return apperr.CharacterExists
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id, currency, balance, wallet_cap, updated_at)
			VALUES ($1, 'XU', $2, $3, $5), ($1, 'DIAMOND', $4, NULL, $5)`,
			id, StartingXu, cfg.XuCap, StartingDiamond, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO farm_plots (character_id, plot_no, unlocked_at)
			SELECT $1, n, $3 FROM generate_series(1, $2) AS n`, id, StartingPlots, now); err != nil {
			return err
		}
		txID := newUUID()
		resp := storedResponse{Result: mustJSON(map[string]string{"character_id": id})}
		if _, err := tx.Exec(ctx, `INSERT INTO transaction_requests
			(id, character_id, request_id, operation, payload_digest, state, response, created_at)
			VALUES ($1, $2, $3, $4, $5, 'committed', $6, $7)`,
			txID, id, requestID, opCreateCharacter, []byte(name.Key), resp, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO asset_ledger
			(transaction_id, character_id, asset_kind, location, amount_delta, balance_after, reason_code, created_at)
			VALUES ($1, $2, 'XU', 'WALLET', $3, $3, 'STARTER_GRANT', $4)`, txID, id, StartingXu, now); err != nil {
			return err
		}
		out = &Character{ID: id, AccountID: accountID, DisplayName: name.Display, InventorySlotCap: StartingSlots, CreatedAt: now}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return out, replayed, nil
}

func characterByAccount(ctx context.Context, q querier, accountID string) (*Character, error) {
	c := &Character{}
	err := q.QueryRow(ctx, `SELECT id, account_id, display_name, inventory_slot_cap, appearance_version, created_at
		FROM characters WHERE account_id = $1`, accountID).
		Scan(&c.ID, &c.AccountID, &c.DisplayName, &c.InventorySlotCap, &c.AppearanceVersion, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// CharacterByAccount returns apperr.CharacterRequired when the account has none.
func (s *Service) CharacterByAccount(ctx context.Context, accountID string) (*Character, error) {
	if !IsUUID(accountID) {
		return nil, apperr.NotFound
	}
	c, err := characterByAccount(ctx, s.pool, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.CharacterRequired
	}
	return c, err
}
