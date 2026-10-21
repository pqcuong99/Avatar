package econ

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"example.com/avatar/transaction-service/internal/apperr"
	"example.com/avatar/transaction-service/internal/names"
)

type RenameResult struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

// Rename changes the character name with a Rename Card (P0-026..029): same
// rules as at creation, no cooldown or limit, and the card is consumed only
// when the rename succeeds — a refusal rolls the whole command back (A-07).
func (w *Work) Rename(raw string) (any, error) {
	name, problem := names.Validate(raw)
	if problem != "" {
		return nil, apperr.CharacterNameInvalid.With(map[string]any{"problem": problem, "min": names.MinGraphemes, "max": names.MaxGraphemes})
	}
	var old string
	if err := w.tx.QueryRow(w.ctx, `SELECT display_name FROM characters WHERE id = $1`, w.CharacterID).Scan(&old); err != nil {
		return nil, err
	}
	if old == name.Display {
		return nil, apperr.Validation.With(map[string]any{"reason": "same_name"})
	}
	if err := w.RemoveItem("rename_card", 1, "CHARACTER_RENAME"); err != nil {
		return nil, err
	}
	err := w.Exec(`UPDATE characters SET display_name = $2, display_name_key = $3, search_key = $4, updated_at = $5 WHERE id = $1`,
		w.CharacterID, name.Display, name.Key, name.SearchKey, w.Now)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, apperr.CharacterNameTaken
	}
	if err != nil {
		return nil, err
	}
	if err := w.Exec(`INSERT INTO character_renames (character_id, old_name, new_name, transaction_id, changed_at) VALUES ($1, $2, $3, $4, $5)`,
		w.CharacterID, old, name.Display, w.txID, w.Now); err != nil {
		return nil, err
	}
	return RenameResult{OldName: old, NewName: name.Display}, nil
}

type renameCmd struct {
	DisplayName string `json:"display_name"`
}

func (c *renameCmd) exec(w *Work) (any, error) { return w.Rename(c.DisplayName) }
