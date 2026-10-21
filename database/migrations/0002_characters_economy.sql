-- Project Thanh M1b + M3a: characters, versioned config, item catalog,
-- inventory, wallets, idempotent transaction requests, append-only ledger.
--
-- Source: docs/ai-handoff-v2/docs/06_DATABASE_SPEC.md. The schema is
-- [ĐỀ XUẤT] (technical proposal), adapted from db/draft/0002_domain_PROPOSAL.sql:
--   * catalog tables use their natural code as primary key (item_code, ...);
--   * positions are integers in 1/100 px, the unit of the realtime protocol;
--   * a transaction_requests row is written once, with its final outcome.
--
-- Invariants the database cannot see are enforced by transaction-service and
-- covered by its tests: name grapheme length/charset, stack limit per catalog
-- item, bag slot cap, wallet caps that come from published config.
--
-- The runner wraps this file in a single transaction.

-- ---------------------------------------------------------------------------
-- Characters (P0-018..029). One per account; no delete/recreate API exists.
-- display_name_key = NFC + Unicode case folding, accents kept (P0-021/022).
-- search_key = lower-cased and accent-stripped; for search only, never unique.
-- ---------------------------------------------------------------------------
CREATE TABLE characters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    display_name_key TEXT NOT NULL UNIQUE,
    search_key TEXT NOT NULL,
    inventory_slot_cap INTEGER NOT NULL DEFAULT 10 CHECK (inventory_slot_cap >= 10),
    map_id TEXT,
    pos_x INTEGER,
    pos_y INTEGER,
    appearance_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((pos_x IS NULL) = (pos_y IS NULL)),
    CHECK ((map_id IS NULL) = (pos_x IS NULL))
);
CREATE INDEX ix_characters_search_key ON characters (search_key text_pattern_ops);

-- ---------------------------------------------------------------------------
-- Versioned configuration. Exactly one version is published at a time and a
-- published version is immutable; changes ship as a new version through
-- maintenance (P0-066, P0-152). Values marked [CẦN CẤU HÌNH] are simply
-- absent until an Admin publishes them, and the feature stays disabled.
-- ---------------------------------------------------------------------------
CREATE TABLE game_config_versions (
    id BIGSERIAL PRIMARY KEY,
    label TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('draft', 'published', 'retired')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    CHECK (state = 'draft' OR published_at IS NOT NULL)
);
CREATE UNIQUE INDEX ux_game_config_one_published
    ON game_config_versions (state) WHERE state = 'published';

CREATE TABLE game_config_values (
    config_version_id BIGINT NOT NULL REFERENCES game_config_versions(id),
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    PRIMARY KEY (config_version_id, key)
);

-- Rows that belong to a non-draft config version may not change. Used by every
-- versioned config table (values here, crop configs and NPC offers in 0003).
CREATE FUNCTION thanh_forbid_published_config_change() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_state TEXT;
BEGIN
    SELECT state INTO v_state FROM game_config_versions
     WHERE id = CASE WHEN TG_OP = 'INSERT' THEN NEW.config_version_id ELSE OLD.config_version_id END;
    IF v_state IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'config version % is %, its rows are immutable',
            CASE WHEN TG_OP = 'INSERT' THEN NEW.config_version_id ELSE OLD.config_version_id END, v_state
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER trg_game_config_values_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON game_config_values
    FOR EACH ROW EXECUTE FUNCTION thanh_forbid_published_config_change();

-- A published version can only move on to retired; nothing goes back to draft.
CREATE FUNCTION thanh_config_version_transition() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state = 'published' AND NEW.state NOT IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published config version cannot become %', NEW.state;
    END IF;
    IF OLD.state = 'retired' AND NEW.state <> 'retired' THEN
        RAISE EXCEPTION 'retired config version cannot become %', NEW.state;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER trg_game_config_version_transition
    BEFORE UPDATE ON game_config_versions
    FOR EACH ROW EXECUTE FUNCTION thanh_config_version_transition();

-- ---------------------------------------------------------------------------
-- Item catalog and inventory (TH-02, P0-104). A slot row exists only while it
-- holds something; empty slots are implicit. Farm/upgrade/worm stack to 999,
-- fashion is one instance per slot.
-- ---------------------------------------------------------------------------
CREATE TABLE item_definitions (
    item_code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('seed', 'crop', 'worm', 'fashion', 'upgrade', 'other')),
    stack_limit INTEGER NOT NULL CHECK (stack_limit BETWEEN 1 AND 999),
    -- Seeds and crops are neither bound nor unbound (docs/02 §2).
    trade_policy TEXT NOT NULL CHECK (trade_policy IN ('not_applicable', 'bound', 'unbound')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (category <> 'fashion' OR stack_limit = 1)
);

CREATE TABLE inventory_slots (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    slot_no INTEGER NOT NULL CHECK (slot_no >= 1),
    item_code TEXT NOT NULL REFERENCES item_definitions(item_code),
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 999),
    -- Set for instance items (fashion); the FK is added together with the
    -- item_instances table.
    item_instance_id UUID UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, slot_no),
    CHECK (item_instance_id IS NULL OR quantity = 1)
);

-- ---------------------------------------------------------------------------
-- Wallets (P0-030..035). wallet_cap NULL means the cap comes from published
-- config (the Diamond cap is Admin-configured, P0-033).
-- ---------------------------------------------------------------------------
CREATE TABLE character_wallets (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    currency TEXT NOT NULL CHECK (currency IN ('XU', 'DIAMOND')),
    balance BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    wallet_cap BIGINT CHECK (wallet_cap > 0),
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, currency),
    CHECK (wallet_cap IS NULL OR balance <= wallet_cap)
);

-- ---------------------------------------------------------------------------
-- Idempotency (docs/06 §6). One row per (character, request_id), written in
-- the same database transaction as the state change it describes, with the
-- final outcome. A replay with the same digest returns the stored response;
-- a different digest is IDEMPOTENCY_CONFLICT.
-- ---------------------------------------------------------------------------
CREATE TABLE transaction_requests (
    id UUID PRIMARY KEY,
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 8 AND 64),
    operation TEXT NOT NULL,
    payload_digest BYTEA NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('committed', 'rejected')),
    response JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, request_id)
);

-- ---------------------------------------------------------------------------
-- Asset ledger (P0-141): every change of Xu, Diamond or item quantity, in the
-- same transaction as the change. Append-only; only the reviewed Closed Beta
-- reset tool may purge it, by setting thanh.allow_player_data_purge = 'on'.
-- balance_after is the wallet balance (XU/DIAMOND) or the total quantity of
-- asset_ref held at `location` after the change (ITEM).
-- ---------------------------------------------------------------------------
CREATE TABLE asset_ledger (
    id BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES transaction_requests(id),
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    asset_kind TEXT NOT NULL CHECK (asset_kind IN ('XU', 'DIAMOND', 'ITEM')),
    asset_ref TEXT,
    location TEXT NOT NULL CHECK (location IN ('WALLET', 'BAG', 'PENDING', 'PENDING_DIAMOND')),
    amount_delta BIGINT NOT NULL CHECK (amount_delta <> 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    reason_code TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((asset_kind = 'ITEM') = (asset_ref IS NOT NULL))
);
CREATE INDEX ix_asset_ledger_character_time ON asset_ledger (character_id, created_at DESC, id DESC);
CREATE INDEX ix_asset_ledger_transaction ON asset_ledger (transaction_id);

-- Row level: UPDATE never, DELETE only during the reviewed purge.
CREATE FUNCTION thanh_asset_ledger_guard_row() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('thanh.allow_player_data_purge', true) = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'asset_ledger is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END $$;

-- Statement level: TRUNCATE skips row triggers, so it needs its own guard.
CREATE FUNCTION thanh_asset_ledger_guard_truncate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('thanh.allow_player_data_purge', true) = 'on' THEN
        RETURN NULL;
    END IF;
    RAISE EXCEPTION 'asset_ledger is append-only: TRUNCATE is not allowed'
        USING ERRCODE = 'insufficient_privilege';
END $$;

CREATE TRIGGER trg_asset_ledger_append_only_row
    BEFORE UPDATE OR DELETE ON asset_ledger
    FOR EACH ROW EXECUTE FUNCTION thanh_asset_ledger_guard_row();
CREATE TRIGGER trg_asset_ledger_append_only_truncate
    BEFORE TRUNCATE ON asset_ledger
    FOR EACH STATEMENT EXECUTE FUNCTION thanh_asset_ledger_guard_truncate();

-- Requests are history too: their outcome never changes after commit.
CREATE FUNCTION thanh_transaction_requests_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('thanh.allow_player_data_purge', true) = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'transaction_requests rows are immutable: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END $$;

CREATE TRIGGER trg_transaction_requests_immutable
    BEFORE UPDATE OR DELETE ON transaction_requests
    FOR EACH ROW EXECUTE FUNCTION thanh_transaction_requests_immutable();

-- ---------------------------------------------------------------------------
-- Pending Diamond rewards over the cap (P0-034, 036..039): one entry per award
-- source, FIFO by (created_at, id), never expires, never merged with crops.
-- ---------------------------------------------------------------------------
CREATE TABLE pending_diamond_awards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    remaining BIGINT NOT NULL CHECK (remaining >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at TIMESTAMPTZ,
    UNIQUE (character_id, source_kind, source_ref),
    CHECK (remaining <= quantity),
    CHECK ((remaining = 0) = (claimed_at IS NOT NULL))
);
CREATE INDEX ix_pending_diamond_fifo
    ON pending_diamond_awards (character_id, created_at, id) WHERE remaining > 0;
