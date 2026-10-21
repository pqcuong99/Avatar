-- Project Thanh M1b: rename with a Rename Card (P0-026..029). The card is
-- consumed only by a successful rename; no cooldown, no limit. Where cards
-- come from (NPC, shop, events, rewards) is Admin config (P0-027): this only
-- defines the item. Its trade policy is not decided; 'bound' is [ĐỀ XUẤT].

INSERT INTO item_definitions (item_code, display_name, category, stack_limit, trade_policy)
VALUES ('rename_card', 'Thẻ Đổi tên', 'other', 999, 'bound');

CREATE TABLE character_renames (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    old_name TEXT NOT NULL,
    new_name TEXT NOT NULL,
    -- The request row is written at the end of the same transaction.
    transaction_id UUID NOT NULL REFERENCES transaction_requests(id) DEFERRABLE INITIALLY DEFERRED,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_character_renames_character ON character_renames (character_id, changed_at DESC);
