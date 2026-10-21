-- Project Thanh M3b + M4: crop catalog, per-version crop config, farm plots,
-- plantings with snapshots, persisted pest incidents, merged crop pending,
-- NPC catalog and per-version NPC offers.
--
-- Source: docs/ai-handoff-v2/docs/02_GAMEPLAY_FARM_INVENTORY_ECONOMY.md and
-- docs/06_DATABASE_SPEC.md §7-8. Schema is [ĐỀ XUẤT].
--
-- Only the CHỐT catalog structure is inserted here (the five MVP crops of
-- P0-059, their seed/crop items and the worm item of P0-104). Yield, NPC
-- prices and pest probability are [CẦN CẤU HÌNH]: they live in a published
-- config version and are not part of any migration.

INSERT INTO item_definitions (item_code, display_name, category, stack_limit, trade_policy) VALUES
    -- Seed stack 999 is [ĐỀ XUẤT]; docs fix 999 for crops, upgrade items and worms.
    ('seed_carrot',  'Hạt cà rốt',     'seed', 999, 'not_applicable'),
    ('seed_potato',  'Hạt khoai tây',  'seed', 999, 'not_applicable'),
    ('seed_tomato',  'Hạt cà chua',    'seed', 999, 'not_applicable'),
    ('seed_corn',    'Hạt ngô',        'seed', 999, 'not_applicable'),
    ('seed_cabbage', 'Hạt bắp cải',    'seed', 999, 'not_applicable'),
    ('crop_carrot',  'Cà rốt',         'crop', 999, 'not_applicable'),
    ('crop_potato',  'Khoai tây',      'crop', 999, 'not_applicable'),
    ('crop_tomato',  'Cà chua',        'crop', 999, 'not_applicable'),
    ('crop_corn',    'Ngô',            'crop', 999, 'not_applicable'),
    ('crop_cabbage', 'Bắp cải',        'crop', 999, 'not_applicable'),
    -- Trade policy of worms is not decided; birds and the feed store are post-MVP.
    ('worm',         'Sâu',            'worm', 999, 'not_applicable');

CREATE TABLE crop_definitions (
    crop_code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    seed_item_code TEXT NOT NULL UNIQUE REFERENCES item_definitions(item_code),
    crop_item_code TEXT NOT NULL UNIQUE REFERENCES item_definitions(item_code),
    planned_release TEXT NOT NULL CHECK (planned_release IN ('MVP', 'V1')),
    sort_order INTEGER NOT NULL UNIQUE
);

INSERT INTO crop_definitions (crop_code, display_name, seed_item_code, crop_item_code, planned_release, sort_order) VALUES
    ('carrot',  'Cà rốt',    'seed_carrot',  'crop_carrot',  'MVP', 1),
    ('potato',  'Khoai tây', 'seed_potato',  'crop_potato',  'MVP', 2),
    ('tomato',  'Cà chua',   'seed_tomato',  'crop_tomato',  'MVP', 3),
    ('corn',    'Ngô',       'seed_corn',    'crop_corn',    'MVP', 4),
    ('cabbage', 'Bắp cải',   'seed_cabbage', 'crop_cabbage', 'MVP', 5);

-- A crop without a row in the published version cannot be planted (GAP-04).
-- growth_seconds is CHỐT per crop (P0-061) but still lives here so that a
-- planting snapshots the whole effective config in one place.
CREATE TABLE crop_configs (
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    crop_code TEXT NOT NULL REFERENCES crop_definitions(crop_code),
    growth_seconds INTEGER NOT NULL CHECK (growth_seconds > 0),
    base_yield INTEGER NOT NULL CHECK (base_yield > 0),
    pest_probability NUMERIC(7, 6) NOT NULL CHECK (pest_probability >= 0 AND pest_probability <= 1),
    PRIMARY KEY (config_version_id, crop_code)
);
CREATE TRIGGER trg_crop_configs_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON crop_configs
    FOR EACH ROW EXECUTE FUNCTION thanh_forbid_published_config_change();

-- ---------------------------------------------------------------------------
-- Farm (P0-068..095). Plots are numbered from 1 and unlocked in order.
-- ---------------------------------------------------------------------------
CREATE TABLE farm_plots (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    plot_no INTEGER NOT NULL CHECK (plot_no >= 1),
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, plot_no)
);

-- state = 'growing' means "still on the plot", mature or not; maturity is
-- derived from matures_at and server time (P0-169), plants never wither.
CREATE TABLE crop_plantings (
    id UUID PRIMARY KEY,
    character_id UUID NOT NULL,
    plot_no INTEGER NOT NULL,
    crop_code TEXT NOT NULL REFERENCES crop_definitions(crop_code),
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    planted_at TIMESTAMPTZ NOT NULL,
    matures_at TIMESTAMPTZ NOT NULL,
    growth_seconds_snapshot INTEGER NOT NULL CHECK (growth_seconds_snapshot > 0),
    base_yield_snapshot INTEGER NOT NULL CHECK (base_yield_snapshot > 0),
    -- The pest roll happens once, when planting; there is never a second roll.
    pest_roll_completed BOOLEAN NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('growing', 'harvested', 'uprooted')),
    completed_at TIMESTAMPTZ,
    harvest_quantity INTEGER CHECK (harvest_quantity >= 0),
    harvest_destination TEXT CHECK (harvest_destination IN ('BAG', 'PENDING')),
    FOREIGN KEY (character_id, plot_no) REFERENCES farm_plots(character_id, plot_no) ON DELETE CASCADE,
    CHECK (pest_roll_completed),
    CHECK (matures_at = planted_at + make_interval(secs => growth_seconds_snapshot)),
    CHECK ((state = 'growing') = (completed_at IS NULL)),
    CHECK ((state = 'harvested') = (harvest_quantity IS NOT NULL AND harvest_destination IS NOT NULL))
);
CREATE UNIQUE INDEX ux_crop_plantings_one_active_per_plot
    ON crop_plantings (character_id, plot_no) WHERE state = 'growing';
CREATE INDEX ix_crop_plantings_owner_state ON crop_plantings (character_id, state, plot_no);

-- At most one pest per planting (P0-097). Spawn lies in the first 70% of the
-- growth time (P0-098); the deadline is spawn + 30% of the full growth time,
-- so it never passes matures_at. Penalty is an integer 5..10 (P0-100).
CREATE TABLE pest_incidents (
    planting_id UUID PRIMARY KEY REFERENCES crop_plantings(id) ON DELETE CASCADE,
    spawn_at TIMESTAMPTZ NOT NULL,
    deadline_at TIMESTAMPTZ NOT NULL,
    penalty_percent SMALLINT NOT NULL CHECK (penalty_percent BETWEEN 5 AND 10),
    caught_at TIMESTAMPTZ,
    CHECK (deadline_at > spawn_at),
    CHECK (caught_at IS NULL OR (caught_at >= spawn_at AND caught_at < deadline_at))
);

-- Crop pending (P0-075..085): one merged line per crop, FIFO by the earliest
-- time a lot entered it. Never sold from here, never auto-claimed.
CREATE TABLE pending_crops (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    crop_code TEXT NOT NULL REFERENCES crop_definitions(crop_code),
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    earliest_created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, crop_code)
);
CREATE INDEX ix_pending_crops_fifo ON pending_crops (character_id, earliest_created_at, crop_code);

-- ---------------------------------------------------------------------------
-- NPCs. Positions and colliders come from the versioned Tiled map (shared by
-- Client and Game Server); the database only knows codes and names.
-- ---------------------------------------------------------------------------
CREATE TABLE npc_definitions (
    npc_code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    functions TEXT[] NOT NULL DEFAULT '{}'
);

-- Prices per published version (P0-066). NPC_SELLS = the player buys,
-- NPC_BUYS = the player sells. Offers of a published version never change.
CREATE TABLE npc_offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    npc_code TEXT NOT NULL REFERENCES npc_definitions(npc_code),
    item_code TEXT NOT NULL REFERENCES item_definitions(item_code),
    direction TEXT NOT NULL CHECK (direction IN ('NPC_SELLS', 'NPC_BUYS')),
    currency TEXT NOT NULL CHECK (currency IN ('XU', 'DIAMOND')),
    unit_price BIGINT NOT NULL CHECK (unit_price > 0),
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (config_version_id, npc_code, item_code, direction)
);
CREATE TRIGGER trg_npc_offers_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON npc_offers
    FOR EACH ROW EXECUTE FUNCTION thanh_forbid_published_config_change();
