// Package gamedb is the Game Server's own database access: character identity
// and position (not assets), public chat and the runtime part of published
// config. Assets are never written here (P0-165).
package gamedb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

var ErrNoCharacter = errors.New("gamedb: account has no character")

type Character struct {
	ID          string
	AccountID   string
	DisplayName string
	MapID       string // empty: never placed yet
	X, Y        int32
}

func (s *Store) CharacterByAccount(ctx context.Context, accountID string) (*Character, error) {
	c := &Character{AccountID: accountID}
	var mapID *string
	var x, y *int32
	err := s.Pool.QueryRow(ctx, `SELECT id::text, display_name, map_id, pos_x, pos_y FROM characters WHERE account_id = $1`, accountID).
		Scan(&c.ID, &c.DisplayName, &mapID, &x, &y)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCharacter
	}
	if err != nil {
		return nil, err
	}
	if mapID != nil {
		c.MapID, c.X, c.Y = *mapID, *x, *y
	}
	return c, nil
}

type Position struct {
	CharacterID string
	MapID       string
	X, Y        int32
}

// SavePositions stores the last valid server positions (docs/06 §1: not every
// frame, only on leave and periodically).
func (s *Store) SavePositions(ctx context.Context, ps []Position) error {
	if len(ps) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, p := range ps {
		batch.Queue(`UPDATE characters SET map_id = $2, pos_x = $3, pos_y = $4, updated_at = now() WHERE id = $1`, p.CharacterID, p.MapID, p.X, p.Y)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

type ChatMessage struct {
	ID       string    `json:"id"`
	RoomID   string    `json:"room_id"`
	SenderID string    `json:"sender_id"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

// InsertPublicChat stores a message once per (sender, request_id). A retried
// send returns the stored message with inserted=false (S-10).
func (s *Store) InsertPublicChat(ctx context.Context, senderID, roomID, requestID, body string) (ChatMessage, bool, error) {
	m := ChatMessage{SenderID: senderID}
	err := s.Pool.QueryRow(ctx, `INSERT INTO public_chat_messages (sender_id, room_id, request_id, body)
		VALUES ($1, $2, $3, $4) ON CONFLICT (sender_id, request_id) DO NOTHING
		RETURNING id::text, room_id, body, sent_at`, senderID, roomID, requestID, body).Scan(&m.ID, &m.RoomID, &m.Body, &m.SentAt)
	if err == nil {
		return m, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m, false, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT id::text, room_id, body, sent_at FROM public_chat_messages
		WHERE sender_id = $1 AND request_id = $2`, senderID, requestID).Scan(&m.ID, &m.RoomID, &m.Body, &m.SentAt)
	return m, false, err
}

// Appearance is what other players need to draw a character: the equipped
// fashion items with their style (MVP-06). It is plain JSON for the client.
type Appearance struct {
	Version int64                     `json:"version"`
	Items   map[string]AppearanceItem `json:"items"`
}

type AppearanceItem struct {
	ItemCode string          `json:"item_code"`
	Style    json.RawMessage `json:"style"`
}

func (s *Store) Appearance(ctx context.Context, characterID string) (Appearance, error) {
	a := Appearance{Items: map[string]AppearanceItem{}}
	if err := s.Pool.QueryRow(ctx, `SELECT appearance_version FROM characters WHERE id = $1`, characterID).Scan(&a.Version); err != nil {
		return a, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT e.equip_slot, i.item_code, f.style
		FROM character_equipment e
		JOIN item_instances i ON i.id = e.item_instance_id
		JOIN fashion_items f ON f.item_code = i.item_code
		WHERE e.character_id = $1`, characterID)
	if err != nil {
		return a, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot string
		var it AppearanceItem
		if err := rows.Scan(&slot, &it.ItemCode, &it.Style); err != nil {
			return a, err
		}
		a.Items[slot] = it
	}
	return a, rows.Err()
}

// Runtime is the part of published config the world needs. Zero values mean
// "not configured".
type Runtime struct {
	ConfigVersion     int64
	RoomCapacity      int
	ChatMaxChars      int
	ChatMinIntervalMS int
	Maintenance       Maintenance
}

type Maintenance struct {
	Active  bool   `json:"active"`
	Message string `json:"message"`
}

func (s *Store) LoadRuntime(ctx context.Context) (Runtime, error) {
	var r Runtime
	if err := s.Pool.QueryRow(ctx, `SELECT value FROM server_state WHERE key = 'maintenance'`).Scan(&r.Maintenance); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return r, err
	}
	err := s.Pool.QueryRow(ctx, `SELECT id FROM game_config_versions WHERE state = 'published'`).Scan(&r.ConfigVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT key, value FROM game_config_values
		WHERE config_version_id = $1 AND key IN ('world.room_capacity', 'chat.public')`, r.ConfigVersion)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if err := rows.Scan(&key, &raw); err != nil {
			return r, err
		}
		switch key {
		case "world.room_capacity":
			_ = json.Unmarshal(raw, &r.RoomCapacity)
		case "chat.public":
			var c struct {
				MaxChars      int `json:"max_chars"`
				MinIntervalMS int `json:"min_interval_ms"`
			}
			if json.Unmarshal(raw, &c) == nil {
				r.ChatMaxChars, r.ChatMinIntervalMS = c.MaxChars, c.MinIntervalMS
			}
		}
	}
	return r, rows.Err()
}
