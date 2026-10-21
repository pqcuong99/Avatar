package econ

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/avatar/transaction-service/internal/apperr"
)

// Daily personal quests (MVP-07, P0-159). The quest list, targets, rewards and
// the reset time zone are Admin config (GAP-10); with no `quests.daily`
// setting published the engine stays idle. Progress is only ever counted by
// the server, inside the transaction of the action it counts (docs/03 §6),
// and a reward is claimed once per character, quest and day.

type QuestDef struct {
	Code           string `json:"quest_code"`
	Title          string `json:"title"`
	EventType      string `json:"event_type"`
	TargetQuantity int    `json:"target_quantity"`
	RewardCurrency string `json:"reward_currency"`
	RewardAmount   int64  `json:"reward_amount"`
	SortOrder      int    `json:"sort_order"`
}

type questSettings struct {
	Timezone string `json:"timezone"`
}

func loadQuests(ctx context.Context, q querier, cfg *Config) error {
	raw, ok := cfg.Values["quests.daily"]
	if !ok {
		return nil
	}
	var s questSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("quests.daily: %w", err)
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return fmt.Errorf("quests.daily timezone: %w", err)
	}
	cfg.QuestLocation = loc
	rows, err := q.Query(ctx, `SELECT quest_code, title, event_type, target_quantity, reward_currency, reward_amount, sort_order
		FROM quest_definitions WHERE config_version_id = $1 ORDER BY sort_order, quest_code`, cfg.VersionID)
	if err != nil {
		return err
	}
	cfg.Quests, err = pgx.CollectRows(rows, pgx.RowToStructByPos[QuestDef])
	return err
}

// questDay is the calendar day of t in the quest time zone; quests reset at
// local midnight. [ĐỀ XUẤT] reset hour 0.
func (c *Config) questDay(t time.Time) (time.Time, bool) {
	if c.QuestLocation == nil {
		return time.Time{}, false
	}
	l := t.In(c.QuestLocation)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC), true
}

// QuestEvent adds progress to today's quests of this event type. It creates
// the day's row on first progress, snapshotting target and reward.
func (w *Work) QuestEvent(eventType string, amount int) error {
	if amount <= 0 {
		return nil
	}
	day, ok := w.Config.questDay(w.Now)
	if !ok {
		return nil
	}
	for _, q := range w.Config.Quests {
		if q.EventType != eventType {
			continue
		}
		if err := w.Exec(`INSERT INTO character_daily_quests
				(character_id, quest_day, quest_code, config_version_id, title, event_type, target_quantity, reward_currency, reward_amount, progress, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, LEAST($10::int, $7::int), CASE WHEN $10 >= $7 THEN $11::timestamptz END)
			ON CONFLICT (character_id, quest_day, quest_code) DO UPDATE
			SET progress = LEAST(character_daily_quests.progress + $10, character_daily_quests.target_quantity),
			    completed_at = CASE WHEN character_daily_quests.progress + $10 >= character_daily_quests.target_quantity
			                        THEN COALESCE(character_daily_quests.completed_at, $11) END`,
			w.CharacterID, day, q.Code, w.Config.VersionID, q.Title, q.EventType, q.TargetQuantity, q.RewardCurrency, q.RewardAmount,
			amount, w.Now); err != nil {
			return err
		}
	}
	return nil
}

type QuestView struct {
	QuestDef
	Progress    int        `json:"progress"`
	CompletedAt *time.Time `json:"completed_at"`
	ClaimedAt   *time.Time `json:"claimed_at"`
}

type QuestsView struct {
	Enabled  bool        `json:"enabled"`
	Day      string      `json:"day,omitempty"`
	ResetsAt *time.Time  `json:"resets_at,omitempty"`
	Quests   []QuestView `json:"quests"`
}

// Quests lists today's quests: the published definitions, with the progress
// snapshot of any row already started today.
func (s *Service) Quests(ctx context.Context, characterID string) (*QuestsView, error) {
	if !IsUUID(characterID) {
		return nil, apperr.NotFound
	}
	cfg, err := loadPublishedConfig(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	now := s.now()
	day, ok := cfg.questDay(now)
	out := &QuestsView{Quests: []QuestView{}}
	if !ok {
		return out, nil
	}
	out.Enabled = true
	out.Day = day.Format("2006-01-02")
	l := now.In(cfg.QuestLocation)
	next := time.Date(l.Year(), l.Month(), l.Day()+1, 0, 0, 0, 0, cfg.QuestLocation).UTC()
	out.ResetsAt = &next

	started := map[string]QuestView{}
	rows, err := s.pool.Query(ctx, `SELECT quest_code, title, event_type, target_quantity, reward_currency, reward_amount, progress, completed_at, claimed_at
		FROM character_daily_quests WHERE character_id = $1 AND quest_day = $2`, characterID, day)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v QuestView
		if err := rows.Scan(&v.Code, &v.Title, &v.EventType, &v.TargetQuantity, &v.RewardCurrency, &v.RewardAmount, &v.Progress, &v.CompletedAt, &v.ClaimedAt); err != nil {
			rows.Close()
			return nil, err
		}
		started[v.Code] = v
	}
	rows.Close()
	for _, q := range cfg.Quests {
		if v, ok := started[q.Code]; ok {
			v.SortOrder = q.SortOrder
			out.Quests = append(out.Quests, v)
			delete(started, q.Code)
			continue
		}
		out.Quests = append(out.Quests, QuestView{QuestDef: q})
	}
	// Quests started today under an older version keep their snapshot.
	for _, v := range started {
		out.Quests = append(out.Quests, v)
	}
	return out, nil
}

type ClaimQuestResult struct {
	QuestCode string `json:"quest_code"`
	Currency  string `json:"currency"`
	Amount    int64  `json:"amount"`
	Credited  int64  `json:"credited"`
	Pending   int64  `json:"pending"` // Diamonds above the cap wait in pending (P0-034)
}

// ClaimQuest pays a completed quest of today exactly once.
func (w *Work) ClaimQuest(code string) (any, error) {
	day, ok := w.Config.questDay(w.Now)
	if !ok {
		return nil, apperr.NotConfigured.With(map[string]any{"config": "quests.daily"})
	}
	var cur string
	var amount int64
	var completed, claimed *time.Time
	err := w.tx.QueryRow(w.ctx, `SELECT reward_currency, reward_amount, completed_at, claimed_at FROM character_daily_quests
		WHERE character_id = $1 AND quest_day = $2 AND quest_code = $3 FOR UPDATE`, w.CharacterID, day, code).Scan(&cur, &amount, &completed, &claimed)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && completed == nil) {
		return nil, errQuestNotComplete.With(map[string]any{"quest_code": code})
	}
	if err != nil {
		return nil, err
	}
	if claimed != nil {
		return nil, errQuestClaimed.With(map[string]any{"quest_code": code})
	}
	res := ClaimQuestResult{QuestCode: code, Currency: cur, Amount: amount}
	source := "quest:" + day.Format("2006-01-02") + ":" + code
	if cur == "DIAMOND" {
		res.Credited, res.Pending, err = w.GrantDiamonds(amount, "quest", source, "QUEST_REWARD")
	} else {
		err = w.Credit(cur, amount, "QUEST_REWARD")
		res.Credited = amount
	}
	if err != nil {
		return nil, err
	}
	if err := w.Exec(`UPDATE character_daily_quests SET claimed_at = $4 WHERE character_id = $1 AND quest_day = $2 AND quest_code = $3`,
		w.CharacterID, day, code, w.Now); err != nil {
		return nil, err
	}
	return res, nil
}

var (
	errQuestNotComplete = &apperr.Error{Code: "QUEST_NOT_COMPLETE", Message: "Nhiệm vụ chưa hoàn thành.", Status: 409}
	errQuestClaimed     = &apperr.Error{Code: "QUEST_ALREADY_CLAIMED", Message: "Phần thưởng nhiệm vụ đã được nhận.", Status: 409}
)

var questEventTypes = map[string]bool{"public_chat": true}

// ExternalQuestEvent counts progress for events that happen outside this
// service (public chat in the Game Server). The caller's request_id (the
// chat message id) makes it count once.
func (w *Work) ExternalQuestEvent(eventType string, amount int) (any, error) {
	if !questEventTypes[eventType] || amount < 1 || amount > 100 {
		return nil, apperr.Validation.With(map[string]any{"field": "event_type"})
	}
	if err := w.QuestEvent(eventType, amount); err != nil {
		return nil, err
	}
	return map[string]any{"event_type": eventType, "amount": amount}, nil
}
