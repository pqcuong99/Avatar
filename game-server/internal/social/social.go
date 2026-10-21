// Package social is friends, profiles, private chat and private-chat blocks
// (MVP-03, TH-05/06/07). It never touches assets.
package social

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/avatar/game-server/internal/textkey"
)

var (
	ErrNotFound        = errors.New("social: not found")
	ErrSelf            = errors.New("social: cannot target yourself")
	ErrAlreadyFriends  = errors.New("social: already friends")
	ErrNotFriends      = errors.New("social: not friends")
	ErrFriendLimit     = errors.New("social: friend limit reached")
	ErrNotPending      = errors.New("social: request is not pending")
	ErrCannotDeliver   = errors.New("social: message cannot be delivered") // never says why (TH-06-002)
	ErrBadCursor       = errors.New("social: bad cursor")
	ErrMessageTooLarge = errors.New("social: message empty or too long")
)

const (
	// MaxFriends is TH-07-001.
	MaxFriends = 100
	// MessageTTL is TH-05-001: each private message lives 30 days after it was sent.
	MessageTTL = 30 * 24 * time.Hour
	// First page / next pages of history (TH-05-002) and of the block list (TH-06-007).
	FirstPage = 50
	NextPage  = 20
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IsUUID(s string) bool { return uuidPattern.MatchString(s) }

type Service struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

func New(pool *pgxpool.Pool) *Service { return &Service{Pool: pool, Now: time.Now} }

func (s *Service) now() time.Time { return s.Now().UTC().Truncate(time.Microsecond) }

type Player struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (s *Service) player(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Player, error) {
	if !IsUUID(id) {
		return Player{}, ErrNotFound
	}
	p := Player{ID: id}
	err := q.QueryRow(ctx, `SELECT display_name FROM characters WHERE id = $1`, id).Scan(&p.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// Search finds characters by name, ignoring case and accents, or by the full
// id (TH-06-006 uses the same rule for the block list).
func (s *Service) Search(ctx context.Context, me, q string) ([]Player, error) {
	q = strings.TrimSpace(q)
	out := []Player{}
	if q == "" {
		return out, nil
	}
	var rows pgx.Rows
	var err error
	if IsUUID(strings.ToLower(q)) {
		rows, err = s.Pool.Query(ctx, `SELECT id::text, display_name FROM characters WHERE id = $1 AND id <> $2`, strings.ToLower(q), me)
	} else {
		key := textkey.Search(q)
		if key == "" {
			return out, nil
		}
		rows, err = s.Pool.Query(ctx, `SELECT id::text, display_name FROM characters
			WHERE search_key LIKE '%' || $1 || '%' AND id <> $2
			ORDER BY (search_key LIKE $1 || '%') DESC, length(search_key), display_name LIMIT 20`, likeEscape(key), me)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Player])
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func pair(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}

type Profile struct {
	Player
	IsFriend    bool    `json:"is_friend"`
	Request     string  `json:"request"` // "", "incoming", "outgoing"
	RequestID   *string `json:"request_id,omitempty"`
	BlockedByMe bool    `json:"blocked_by_me"`
}

func (s *Service) Profile(ctx context.Context, me, target string) (*Profile, error) {
	p, err := s.player(ctx, s.Pool, target)
	if err != nil {
		return nil, err
	}
	out := &Profile{Player: p}
	a, b := pair(me, target)
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM friendships WHERE character_a = $1 AND character_b = $2)`, a, b).Scan(&out.IsFriend); err != nil {
		return nil, err
	}
	var reqID, sender string
	err = s.Pool.QueryRow(ctx, `SELECT id::text, sender_id::text FROM friend_requests
		WHERE state = 'pending' AND LEAST(sender_id, receiver_id) = $1 AND GREATEST(sender_id, receiver_id) = $2`, a, b).Scan(&reqID, &sender)
	switch {
	case err == nil:
		out.RequestID = &reqID
		if sender == me {
			out.Request = "outgoing"
		} else {
			out.Request = "incoming"
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM private_chat_blocks WHERE blocker_id = $1 AND blocked_id = $2)`, me, target).Scan(&out.BlockedByMe); err != nil {
		return nil, err
	}
	return out, nil
}

// lockPair locks both characters in id order so concurrent friend changes of
// either one serialize without deadlocks.
func lockPair(ctx context.Context, tx pgx.Tx, a, b string) error {
	x, y := pair(a, b)
	rows, err := tx.Query(ctx, `SELECT id FROM characters WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`, x, y)
	if err != nil {
		return err
	}
	n := 0
	for rows.Next() {
		n++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if n != 2 {
		return ErrNotFound
	}
	return nil
}

func friendCount(ctx context.Context, tx pgx.Tx, id string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM friendships WHERE character_a = $1 OR character_b = $1`, id).Scan(&n)
	return n, err
}

// FriendLimitError says whose list is full; it is reported to the player.
type FriendLimitError struct{ Mine bool }

func (e *FriendLimitError) Error() string { return ErrFriendLimit.Error() }
func (e *FriendLimitError) Unwrap() error { return ErrFriendLimit }

func befriend(ctx context.Context, tx pgx.Tx, a, b string, at time.Time) error {
	for _, id := range []string{a, b} {
		n, err := friendCount(ctx, tx, id)
		if err != nil {
			return err
		}
		if n >= MaxFriends {
			return &FriendLimitError{Mine: id == a}
		}
	}
	x, y := pair(a, b)
	_, err := tx.Exec(ctx, `INSERT INTO friendships (character_a, character_b, created_at) VALUES ($1, $2, $3)`, x, y, at)
	return err
}

type RequestResult struct {
	RequestID string `json:"request_id"`
	State     string `json:"state"` // pending or accepted
}

// SendRequest asks target to be friends. If target had already asked me,
// that request is accepted instead, so the pair never gets two requests.
func (s *Service) SendRequest(ctx context.Context, me, target string) (*RequestResult, error) {
	if me == target {
		return nil, ErrSelf
	}
	var out *RequestResult
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockPair(ctx, tx, me, target); err != nil {
			return err
		}
		a, b := pair(me, target)
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM friendships WHERE character_a = $1 AND character_b = $2)`, a, b).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrAlreadyFriends
		}
		now := s.now()
		var id, sender string
		err := tx.QueryRow(ctx, `SELECT id::text, sender_id::text FROM friend_requests
			WHERE state = 'pending' AND LEAST(sender_id, receiver_id) = $1 AND GREATEST(sender_id, receiver_id) = $2`, a, b).Scan(&id, &sender)
		switch {
		case err == nil && sender == me:
			out = &RequestResult{RequestID: id, State: "pending"}
			return nil
		case err == nil:
			if err := befriend(ctx, tx, me, target, now); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE friend_requests SET state = 'accepted', decided_at = $2 WHERE id = $1`, id, now)
			out = &RequestResult{RequestID: id, State: "accepted"}
			return err
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		// Early feedback; the authoritative check runs again on accept.
		if n, err := friendCount(ctx, tx, me); err != nil {
			return err
		} else if n >= MaxFriends {
			return &FriendLimitError{Mine: true}
		}
		err = tx.QueryRow(ctx, `INSERT INTO friend_requests (sender_id, receiver_id, state, created_at) VALUES ($1, $2, 'pending', $3) RETURNING id::text`,
			me, target, now).Scan(&id)
		out = &RequestResult{RequestID: id, State: "pending"}
		return err
	})
	return out, err
}

// Decide accepts, rejects (receiver) or cancels (sender) a pending request.
func (s *Service) Decide(ctx context.Context, me, requestID, action string) (*Player, error) {
	if !IsUUID(requestID) {
		return nil, ErrNotFound
	}
	var other Player
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var sender, receiver, state string
		err := tx.QueryRow(ctx, `SELECT sender_id::text, receiver_id::text, state FROM friend_requests WHERE id = $1`, requestID).Scan(&sender, &receiver, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if (action == "cancel" && sender != me) || (action != "cancel" && receiver != me) {
			return ErrNotFound
		}
		if err := lockPair(ctx, tx, sender, receiver); err != nil {
			return err
		}
		// Re-read under the locks: a concurrent decision may have won.
		if err := tx.QueryRow(ctx, `SELECT state FROM friend_requests WHERE id = $1`, requestID).Scan(&state); err != nil {
			return err
		}
		if state != "pending" {
			return ErrNotPending
		}
		now := s.now()
		newState := map[string]string{"accept": "accepted", "reject": "rejected", "cancel": "cancelled"}[action]
		if newState == "" {
			return ErrNotFound
		}
		if action == "accept" {
			if err := befriend(ctx, tx, receiver, sender, now); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE friend_requests SET state = $2, decided_at = $3 WHERE id = $1`, requestID, newState, now); err != nil {
			return err
		}
		otherID := sender
		if action == "cancel" {
			otherID = receiver
		}
		other, err = s.player(ctx, tx, otherID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &other, nil
}

// RemoveFriend ends a friendship. [ĐỀ XUẤT] — removal is not specified in
// the decisions; it does not block, notify or delete chat history.
func (s *Service) RemoveFriend(ctx context.Context, me, target string) error {
	if !IsUUID(target) {
		return ErrNotFound
	}
	a, b := pair(me, target)
	tag, err := s.Pool.Exec(ctx, `DELETE FROM friendships WHERE character_a = $1 AND character_b = $2`, a, b)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFriends
	}
	return nil
}

func (s *Service) AreFriends(ctx context.Context, a, b string) (bool, error) {
	x, y := pair(a, b)
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM friendships WHERE character_a = $1 AND character_b = $2)`, x, y).Scan(&ok)
	return ok, err
}

type Friend struct {
	Player
	Since time.Time `json:"since"`
}

func (s *Service) Friends(ctx context.Context, me string) ([]Friend, error) {
	rows, err := s.Pool.Query(ctx, `SELECT c.id::text, c.display_name, f.created_at
		FROM friendships f JOIN characters c ON c.id = CASE WHEN f.character_a = $1 THEN f.character_b ELSE f.character_a END
		WHERE f.character_a = $1 OR f.character_b = $1
		ORDER BY c.display_name`, me)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Friend, error) {
		var f Friend
		err := r.Scan(&f.ID, &f.DisplayName, &f.Since)
		return f, err
	})
}

type PendingRequest struct {
	ID        string    `json:"id"`
	Player    Player    `json:"player"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) Requests(ctx context.Context, me string) (incoming, outgoing []PendingRequest, err error) {
	rows, err := s.Pool.Query(ctx, `SELECT r.id::text, r.sender_id = $1, c.id::text, c.display_name, r.created_at
		FROM friend_requests r JOIN characters c ON c.id = CASE WHEN r.sender_id = $1 THEN r.receiver_id ELSE r.sender_id END
		WHERE r.state = 'pending' AND (r.sender_id = $1 OR r.receiver_id = $1)
		ORDER BY r.created_at DESC`, me)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	incoming, outgoing = []PendingRequest{}, []PendingRequest{}
	for rows.Next() {
		var p PendingRequest
		var mine bool
		if err := rows.Scan(&p.ID, &mine, &p.Player.ID, &p.Player.DisplayName, &p.CreatedAt); err != nil {
			return nil, nil, err
		}
		if mine {
			outgoing = append(outgoing, p)
		} else {
			incoming = append(incoming, p)
		}
	}
	return incoming, outgoing, rows.Err()
}

// ---------------------------------------------------------------------------
// Private chat
// ---------------------------------------------------------------------------

type Message struct {
	ID          string     `json:"id"`
	SenderID    string     `json:"sender_id"`
	RecipientID string     `json:"recipient_id"`
	RequestID   string     `json:"request_id,omitempty"`
	Body        string     `json:"body"`
	SentAt      time.Time  `json:"sent_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	SeenAt      *time.Time `json:"seen_at"`
}

func (s *Service) IsBlocked(ctx context.Context, blocker, blocked string) (bool, error) {
	var b bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM private_chat_blocks WHERE blocker_id = $1 AND blocked_id = $2)`, blocker, blocked).Scan(&b)
	return b, err
}

// SendPM stores a private message once per (sender, request_id): a retried
// request returns the stored message with inserted=false (S-10). A recipient
// who blocked the sender makes the send fail with the generic
// ErrCannotDeliver (TH-06-002).
func (s *Service) SendPM(ctx context.Context, me, target, requestID, body string, maxChars int) (*Message, bool, error) {
	if me == target {
		return nil, false, ErrSelf
	}
	if _, err := s.player(ctx, s.Pool, target); err != nil {
		return nil, false, err
	}
	if n := len([]rune(body)); n == 0 || n > maxChars {
		return nil, false, ErrMessageTooLarge
	}
	// A retry of something already stored is answered first: the original
	// was delivered before any later block.
	if m, err := s.byRequest(ctx, me, requestID); err == nil {
		return m, false, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	blocked, err := s.IsBlocked(ctx, target, me)
	if err != nil {
		return nil, false, err
	}
	if blocked {
		return nil, false, ErrCannotDeliver
	}
	now := s.now()
	m := &Message{SenderID: me, RecipientID: target, RequestID: requestID, Body: body, SentAt: now, ExpiresAt: now.Add(MessageTTL)}
	err = s.Pool.QueryRow(ctx, `INSERT INTO private_chat_messages (sender_id, recipient_id, request_id, body, sent_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (sender_id, request_id) DO NOTHING RETURNING id::text`,
		me, target, requestID, body, now, m.ExpiresAt).Scan(&m.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		m, err := s.byRequest(ctx, me, requestID)
		return m, false, err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return nil, false, fmt.Errorf("store private message: %w", err)
	}
	return m, err == nil, err
}

func (s *Service) byRequest(ctx context.Context, sender, requestID string) (*Message, error) {
	m := &Message{}
	err := s.Pool.QueryRow(ctx, `SELECT id::text, sender_id::text, recipient_id::text, request_id, body, sent_at, expires_at, seen_at
		FROM private_chat_messages WHERE sender_id = $1 AND request_id = $2`, sender, requestID).
		Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.RequestID, &m.Body, &m.SentAt, &m.ExpiresAt, &m.SeenAt)
	return m, err
}

// cursor encodes the (sent_at, id) of the oldest message already loaded.
func encodeCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(t.UnixMicro(), 10) + "|" + id))
}

func decodeCursor(c string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", ErrBadCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", ErrBadCursor
	}
	us, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", ErrBadCursor
	}
	return time.UnixMicro(us).UTC(), parts[1], nil
}

type Page struct {
	Messages   []Message `json:"messages"` // oldest first
	NextCursor *string   `json:"next_cursor"`
}

// History returns the newest messages of a conversation (FirstPage), or with
// a cursor the NextPage messages older than it. Expired messages are never
// returned, even before the purge job removes them (S-06).
func (s *Service) History(ctx context.Context, me, other, cursor string) (*Page, error) {
	if !IsUUID(other) {
		return nil, ErrNotFound
	}
	a, b := pair(me, other)
	limit := FirstPage
	args := []any{a, b, s.now()}
	where := ""
	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		limit = NextPage
		args = append(args, t, id)
		where = ` AND (sent_at, id) < ($4, $5::uuid)`
	}
	args = append(args, limit+1)
	rows, err := s.Pool.Query(ctx, `SELECT id::text, sender_id::text, recipient_id::text, '', body, sent_at, expires_at, seen_at
		FROM private_chat_messages
		WHERE LEAST(sender_id, recipient_id) = $1 AND GREATEST(sender_id, recipient_id) = $2 AND expires_at > $3`+where+`
		ORDER BY sent_at DESC, id DESC LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	msgs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Message])
	if err != nil {
		return nil, err
	}
	page := &Page{Messages: []Message{}}
	if len(msgs) > limit {
		msgs = msgs[:limit]
		last := msgs[len(msgs)-1]
		c := encodeCursor(last.SentAt, last.ID)
		page.NextCursor = &c
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		page.Messages = append(page.Messages, msgs[i])
	}
	return page, nil
}

type SeenUpdate struct {
	SenderID   string    `json:"sender_id"`
	MessageIDs []string  `json:"message_ids"`
	SeenAt     time.Time `json:"seen_at"`
}

// MarkSeen records that the recipient had these messages on screen. Only
// the recipient can mark a message, only once, and never an expired one.
func (s *Service) MarkSeen(ctx context.Context, me string, ids []string) ([]SeenUpdate, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if IsUUID(id) {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 || len(valid) > 200 {
		return nil, nil
	}
	now := s.now()
	rows, err := s.Pool.Query(ctx, `UPDATE private_chat_messages SET seen_at = $3
		WHERE id = ANY($2::uuid[]) AND recipient_id = $1 AND seen_at IS NULL AND expires_at > $3
		RETURNING id::text, sender_id::text`, me, valid, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySender := map[string]*SeenUpdate{}
	var out []SeenUpdate
	for rows.Next() {
		var id, sender string
		if err := rows.Scan(&id, &sender); err != nil {
			return nil, err
		}
		u := bySender[sender]
		if u == nil {
			u = &SeenUpdate{SenderID: sender, SeenAt: now}
			bySender[sender] = u
		}
		u.MessageIDs = append(u.MessageIDs, id)
	}
	for _, u := range bySender {
		out = append(out, *u)
	}
	return out, rows.Err()
}

type Conversation struct {
	Partner Player    `json:"partner"`
	Last    Message   `json:"last"`
	Unread  int       `json:"unread"`
	Updated time.Time `json:"updated_at"`
}

func (s *Service) Conversations(ctx context.Context, me string) ([]Conversation, error) {
	rows, err := s.Pool.Query(ctx, `WITH mine AS (
			SELECT m.*, CASE WHEN m.sender_id = $1 THEN m.recipient_id ELSE m.sender_id END AS partner
			FROM private_chat_messages m
			WHERE (m.sender_id = $1 OR m.recipient_id = $1) AND m.expires_at > $2
		), last AS (
			SELECT DISTINCT ON (partner) * FROM mine ORDER BY partner, sent_at DESC, id DESC
		)
		SELECT c.id::text, c.display_name, l.id::text, l.sender_id::text, l.recipient_id::text, l.body, l.sent_at, l.expires_at, l.seen_at,
			(SELECT count(*) FROM mine u WHERE u.partner = l.partner AND u.recipient_id = $1 AND u.seen_at IS NULL)
		FROM last l JOIN characters c ON c.id = l.partner
		ORDER BY l.sent_at DESC LIMIT 100`, me, s.now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		m := &c.Last
		if err := rows.Scan(&c.Partner.ID, &c.Partner.DisplayName, &m.ID, &m.SenderID, &m.RecipientID, &m.Body, &m.SentAt, &m.ExpiresAt, &m.SeenAt, &c.Unread); err != nil {
			return nil, err
		}
		c.Updated = m.SentAt
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

func (s *Service) Block(ctx context.Context, me, target string) error {
	if me == target {
		return ErrSelf
	}
	if _, err := s.player(ctx, s.Pool, target); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO private_chat_blocks (blocker_id, blocked_id, created_at) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, me, target, s.now())
	return err
}

// Unblock never sends a message, never befriends and never brings back
// expired messages (TH-06-004). It is idempotent.
func (s *Service) Unblock(ctx context.Context, me, target string) error {
	if !IsUUID(target) {
		return ErrNotFound
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM private_chat_blocks WHERE blocker_id = $1 AND blocked_id = $2`, me, target)
	return err
}

type Blocked struct {
	Player
	BlockedAt time.Time `json:"blocked_at"`
}

type BlockPage struct {
	Items      []Blocked `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// Blocks lists my blocks, most recent first, 50 then 20 per page; q filters
// by name (case and accents ignored) or by the full id (TH-06-005..007).
func (s *Service) Blocks(ctx context.Context, me, q, cursor string) (*BlockPage, error) {
	limit := FirstPage
	args := []any{me}
	where := ""
	if q = strings.TrimSpace(q); q != "" {
		if IsUUID(strings.ToLower(q)) {
			args = append(args, strings.ToLower(q))
			where += ` AND b.blocked_id = $2`
		} else {
			args = append(args, likeEscape(textkey.Search(q)))
			where += ` AND c.search_key LIKE '%' || $2 || '%'`
		}
	}
	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, err
		}
		limit = NextPage
		args = append(args, t, id)
		where += fmt.Sprintf(` AND (b.created_at, b.blocked_id) < ($%d, $%d::uuid)`, len(args)-1, len(args))
	}
	args = append(args, limit+1)
	rows, err := s.Pool.Query(ctx, `SELECT c.id::text, c.display_name, b.created_at
		FROM private_chat_blocks b JOIN characters c ON c.id = b.blocked_id
		WHERE b.blocker_id = $1`+where+`
		ORDER BY b.created_at DESC, b.blocked_id DESC LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Blocked, error) {
		var b Blocked
		err := r.Scan(&b.ID, &b.DisplayName, &b.BlockedAt)
		return b, err
	})
	if err != nil {
		return nil, err
	}
	page := &BlockPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		c := encodeCursor(last.BlockedAt, last.ID)
		page.NextCursor = &c
	}
	return page, nil
}

// PurgeExpired deletes private messages past their expiry; reads already
// ignore them, this only reclaims space.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM private_chat_messages WHERE expires_at <= $1`, s.now())
	return tag.RowsAffected(), err
}
