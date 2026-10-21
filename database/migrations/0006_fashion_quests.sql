-- Project Thanh M6: basic fashion (MVP-06, P0-158) and personal daily quests
-- (MVP-07, P0-159). Schema is [ĐỀ XUẤT].
--
-- Fashion: every fashion item is its own instance taking one bag slot
-- (TH-02-004). Which items exist, their slots, prices and art are
-- [CẦN CẤU HÌNH] (catalog content, GAP-13/14): nothing is inserted here.
-- An equipped item stays in its bag slot (the draft's proposal; GAP-14 asks
-- for a UX review), so unequipping can never fail for lack of space.

CREATE TABLE fashion_items (
    item_code TEXT PRIMARY KEY REFERENCES item_definitions(item_code),
    equip_slot TEXT NOT NULL CHECK (equip_slot IN ('hat', 'top', 'bottom')),
    -- Rendering parameters of the placeholder art (colours, hat shape).
    style JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE item_instances (
    id UUID PRIMARY KEY,
    item_code TEXT NOT NULL REFERENCES item_definitions(item_code),
    owner_character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    bound BOOLEAN NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ
);
CREATE INDEX ix_item_instances_owner ON item_instances (owner_character_id);

ALTER TABLE inventory_slots
    ADD CONSTRAINT fk_inventory_slots_instance FOREIGN KEY (item_instance_id) REFERENCES item_instances(id) ON DELETE CASCADE;

CREATE TABLE character_equipment (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    equip_slot TEXT NOT NULL CHECK (equip_slot IN ('hat', 'top', 'bottom')),
    item_instance_id UUID NOT NULL UNIQUE REFERENCES item_instances(id) ON DELETE CASCADE,
    equipped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, equip_slot)
);

-- Equipment may only reference an instance the character owns.
CREATE FUNCTION thanh_equipment_owner_check() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM item_instances WHERE id = NEW.item_instance_id AND owner_character_id = NEW.character_id) THEN
        RAISE EXCEPTION 'item instance % is not owned by character %', NEW.item_instance_id, NEW.character_id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_character_equipment_owner
    BEFORE INSERT OR UPDATE ON character_equipment
    FOR EACH ROW EXECUTE FUNCTION thanh_equipment_owner_check();

-- ---------------------------------------------------------------------------
-- Daily quests. The list, targets, rewards and reset time zone are
-- [CẦN CẤU HÌNH] (GAP-10): definitions belong to a config version and the
-- engine stays idle until one is published. Progress comes only from server
-- events recorded in the same transaction as the action (docs/03 §6).
-- ---------------------------------------------------------------------------
CREATE TABLE quest_definitions (
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    quest_code TEXT NOT NULL,
    title TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('plant_crop', 'harvest_crop', 'sell_crop_to_npc', 'catch_pest', 'public_chat')),
    target_quantity INTEGER NOT NULL CHECK (target_quantity > 0),
    reward_currency TEXT NOT NULL CHECK (reward_currency IN ('XU', 'DIAMOND')),
    reward_amount BIGINT NOT NULL CHECK (reward_amount > 0),
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (config_version_id, quest_code)
);
CREATE TRIGGER trg_quest_definitions_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON quest_definitions
    FOR EACH ROW EXECUTE FUNCTION thanh_forbid_published_config_change();

-- One row per character, quest and quest day; target and reward are
-- snapshotted when the row is created so a later publish cannot change a
-- quest already under way. The reward is claimed at most once (claimed_at).
CREATE TABLE character_daily_quests (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    quest_day DATE NOT NULL,
    quest_code TEXT NOT NULL,
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    title TEXT NOT NULL,
    event_type TEXT NOT NULL,
    target_quantity INTEGER NOT NULL CHECK (target_quantity > 0),
    reward_currency TEXT NOT NULL CHECK (reward_currency IN ('XU', 'DIAMOND')),
    reward_amount BIGINT NOT NULL CHECK (reward_amount > 0),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0),
    completed_at TIMESTAMPTZ,
    claimed_at TIMESTAMPTZ,
    PRIMARY KEY (character_id, quest_day, quest_code),
    CHECK ((completed_at IS NOT NULL) = (progress >= target_quantity)),
    CHECK (claimed_at IS NULL OR completed_at IS NOT NULL)
);
