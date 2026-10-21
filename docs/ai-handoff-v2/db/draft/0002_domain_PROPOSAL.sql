-- ========================================================================
-- PROJECT THANH / PROPOSED DOMAIN SCHEMA — REVIEW REQUIRED (NOT A MIGRATION
-- TO AUTO-RUN). Target: PostgreSQL 15+ or currently-supported compatible.
-- Prerequisite existing M1 `0001_auth.sql` with `accounts` and `auth_sessions`.
-- Source: docs/06_DATABASE_SPEC.md; [DE XUAT] implementation, not product law.
-- Never execute against a real or production database before DBA review.
-- ========================================================================
BEGIN;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Characters and renames. Canonical display_name_key derived application-side
-- using Unicode NFC + casefold WITHOUT accent stripping.
CREATE TABLE IF NOT EXISTS characters (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id UUID NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
  display_name TEXT NOT NULL,
  display_name_key TEXT NOT NULL UNIQUE,
  inventory_slot_cap INTEGER NOT NULL DEFAULT 10 CHECK (inventory_slot_cap >= 10),
  unlocked_plot_count INTEGER NOT NULL DEFAULT 6 CHECK (unlocked_plot_count >= 6),
  map_id TEXT,
  last_x NUMERIC(12,4), last_y NUMERIC(12,4),
  appearance_version BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS character_renames (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  old_name TEXT NOT NULL, new_name TEXT NOT NULL,
  consumed_item_instance_id UUID,
  transaction_id UUID,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Versioned published configuration. Drafts never mutate the active version.
CREATE TABLE IF NOT EXISTS game_config_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  label TEXT NOT NULL UNIQUE,
  state TEXT NOT NULL CHECK (state IN ('draft','staged','published','retired')),
  published_at TIMESTAMPTZ,
  effective_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_one_active_game_config
  ON game_config_versions(state) WHERE state='published';
CREATE TABLE IF NOT EXISTS game_config_values (
  config_version_id UUID NOT NULL REFERENCES game_config_versions(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  value JSONB NOT NULL,
  PRIMARY KEY(config_version_id,key)
);

-- Item definitions are catalog metadata, never the canonical inventory balance.
CREATE TABLE IF NOT EXISTS item_definitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_code TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('seed','crop','worm','fashion','upgrade','other')),
  stack_limit INTEGER NOT NULL CHECK (stack_limit>=1),
  trade_policy TEXT NOT NULL CHECK (trade_policy IN ('not_applicable','bound','unbound','restricted')),
  time_limited BOOLEAN NOT NULL DEFAULT FALSE,
  is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS item_instances (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_definition_id UUID NOT NULL REFERENCES item_definitions(id),
  owner_character_id UUID REFERENCES characters(id) ON DELETE CASCADE,
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS ix_item_instances_owner ON item_instances(owner_character_id);
CREATE TABLE IF NOT EXISTS inventory_slots (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  slot_no INTEGER NOT NULL CHECK (slot_no>=1),
  item_definition_id UUID NOT NULL REFERENCES item_definitions(id),
  item_instance_id UUID UNIQUE REFERENCES item_instances(id) ON DELETE CASCADE,
  quantity BIGINT NOT NULL CHECK (quantity>=1 AND quantity<=999),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(character_id,slot_no),
  CHECK (item_instance_id IS NULL OR quantity=1)
);
CREATE TABLE IF NOT EXISTS character_equipment (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  equip_slot TEXT NOT NULL,
  item_instance_id UUID NOT NULL UNIQUE REFERENCES item_instances(id),
  equipped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(character_id,equip_slot)
);

-- Canonical money balances and immutable mutation records.
CREATE TABLE IF NOT EXISTS character_wallets (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  currency TEXT NOT NULL CHECK (currency IN ('XU','DIAMOND')),
  balance BIGINT NOT NULL DEFAULT 0 CHECK (balance>=0),
  wallet_cap BIGINT NOT NULL CHECK (wallet_cap>0),
  version BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY(character_id,currency),
  CHECK (balance<=wallet_cap)
);
CREATE TABLE IF NOT EXISTS transaction_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id UUID REFERENCES characters(id) ON DELETE CASCADE,
  request_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  payload_digest BYTEA NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('processing','committed','rejected')),
  response_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  committed_at TIMESTAMPTZ,
  UNIQUE(character_id,request_id)
);
CREATE TABLE IF NOT EXISTS asset_ledger (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id UUID NOT NULL REFERENCES transaction_requests(id) ON DELETE RESTRICT,
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  asset_kind TEXT NOT NULL CHECK (asset_kind IN ('XU','DIAMOND','ITEM','CROP','WORM')),
  asset_ref TEXT,
  amount_delta BIGINT NOT NULL CHECK (amount_delta <> 0),
  balance_after BIGINT,
  reason_code TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_asset_ledger_character_time
  ON asset_ledger(character_id,created_at DESC,id);
CREATE TABLE IF NOT EXISTS outbox_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id UUID REFERENCES transaction_requests(id) ON DELETE SET NULL,
  topic TEXT NOT NULL,
  aggregate_id UUID,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS ix_outbox_unpublished ON outbox_events(created_at) WHERE published_at IS NULL;

-- Separate diamond pending entries by award source. NOT crop pending.
CREATE TABLE IF NOT EXISTS pending_diamond_awards (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  source_kind TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  quantity BIGINT NOT NULL CHECK (quantity>0),
  remaining BIGINT NOT NULL CHECK (remaining>=0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(character_id,source_kind,source_ref),
  CHECK (remaining<=quantity)
);
CREATE INDEX IF NOT EXISTS ix_pending_diamonds_fifo
  ON pending_diamond_awards(character_id,created_at,id) WHERE remaining>0;

-- Catalog for crops and snapshot economics.
CREATE TABLE IF NOT EXISTS crop_definitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  crop_code TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  seed_item_id UUID NOT NULL UNIQUE REFERENCES item_definitions(id),
  harvest_item_id UUID NOT NULL UNIQUE REFERENCES item_definitions(id),
  planned_release TEXT NOT NULL CHECK (planned_release IN ('MVP','V1','LATER'))
);
CREATE TABLE IF NOT EXISTS crop_config_versions (
  config_version_id UUID NOT NULL REFERENCES game_config_versions(id) ON DELETE CASCADE,
  crop_id UUID NOT NULL REFERENCES crop_definitions(id),
  seed_price_xu BIGINT CHECK (seed_price_xu>=0),
  npc_sale_price_xu BIGINT CHECK (npc_sale_price_xu>=0),
  base_yield INTEGER CHECK (base_yield>0),
  growth_seconds INTEGER CHECK (growth_seconds>0),
  pest_probability NUMERIC(7,6) CHECK (pest_probability>=0 AND pest_probability<=1),
  PRIMARY KEY(config_version_id,crop_id)
);
CREATE TABLE IF NOT EXISTS farm_plots (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  plot_no INTEGER NOT NULL CHECK (plot_no>=1),
  is_unlocked BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY(character_id,plot_no)
);
CREATE TABLE IF NOT EXISTS crop_plantings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id UUID NOT NULL,
  plot_no INTEGER NOT NULL,
  crop_id UUID NOT NULL REFERENCES crop_definitions(id),
  planted_at TIMESTAMPTZ NOT NULL,
  mature_at TIMESTAMPTZ NOT NULL,
  growth_seconds_snapshot INTEGER NOT NULL CHECK (growth_seconds_snapshot>0),
  base_yield_snapshot INTEGER NOT NULL CHECK (base_yield_snapshot>0),
  config_version_id UUID NOT NULL REFERENCES game_config_versions(id),
  pest_roll_completed BOOLEAN NOT NULL DEFAULT FALSE,
  state TEXT NOT NULL CHECK (state IN ('growing','harvested','uprooted')),
  completed_at TIMESTAMPTZ,
  FOREIGN KEY(character_id,plot_no) REFERENCES farm_plots(character_id,plot_no),
  CHECK (mature_at>planted_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_one_active_plant_per_plot
  ON crop_plantings(character_id,plot_no) WHERE state='growing';
CREATE INDEX IF NOT EXISTS ix_planting_owner_state ON crop_plantings(character_id,state,mature_at);
CREATE TABLE IF NOT EXISTS pest_incidents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planting_id UUID NOT NULL UNIQUE REFERENCES crop_plantings(id) ON DELETE CASCADE,
  spawn_at TIMESTAMPTZ NOT NULL,
  deadline_at TIMESTAMPTZ NOT NULL,
  penalty_percent SMALLINT NOT NULL CHECK (penalty_percent BETWEEN 5 AND 10),
  caught_at TIMESTAMPTZ,
  worm_awarded BOOLEAN NOT NULL DEFAULT FALSE,
  CHECK (deadline_at>spawn_at),
  CHECK (caught_at IS NULL OR caught_at>=spawn_at)
);
-- Pending crop claims MERGE PER (character,crop), preserving earliest created_at.
CREATE TABLE IF NOT EXISTS pending_crops (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  crop_id UUID NOT NULL REFERENCES crop_definitions(id),
  quantity BIGINT NOT NULL CHECK (quantity>0),
  earliest_created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(character_id,crop_id)
);
CREATE INDEX IF NOT EXISTS ix_pending_crop_fifo
  ON pending_crops(character_id,earliest_created_at,crop_id);

-- Published NPC catalog / buying and selling prices by maintenance version.
CREATE TABLE IF NOT EXISTS npc_definitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  npc_code TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  map_id TEXT NOT NULL,
  spawn_x NUMERIC(12,4) NOT NULL,
  spawn_y NUMERIC(12,4) NOT NULL,
  collision_radius NUMERIC(12,4) NOT NULL CHECK(collision_radius>0)
);
CREATE TABLE IF NOT EXISTS npc_offers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  config_version_id UUID NOT NULL REFERENCES game_config_versions(id),
  npc_id UUID NOT NULL REFERENCES npc_definitions(id),
  item_definition_id UUID NOT NULL REFERENCES item_definitions(id),
  direction TEXT NOT NULL CHECK(direction IN ('NPC_SELLS','NPC_BUYS')),
  currency TEXT NOT NULL CHECK(currency IN ('XU','DIAMOND')),
  unit_price BIGINT NOT NULL CHECK(unit_price>=0),
  is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE(config_version_id,npc_id,item_definition_id,direction)
);

-- Social: friend limits and block effects are validated by the service.
CREATE TABLE IF NOT EXISTS friend_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  receiver_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  state TEXT NOT NULL CHECK(state IN ('pending','accepted','rejected','cancelled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at TIMESTAMPTZ,
  CHECK(sender_id<>receiver_id)
);
CREATE TABLE IF NOT EXISTS friendships (
  character_a UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  character_b UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(character_a,character_b),
  CHECK(character_a<character_b)
);
CREATE TABLE IF NOT EXISTS private_chat_blocks (
  blocker_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  blocked_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(blocker_id,blocked_id),
  CHECK(blocker_id<>blocked_id)
);
CREATE INDEX IF NOT EXISTS ix_chat_blocks_recent ON private_chat_blocks(blocker_id,created_at DESC);
CREATE TABLE IF NOT EXISTS private_chat_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id TEXT NOT NULL,
  sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  recipient_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  body TEXT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  UNIQUE(sender_id,request_id),
  CHECK(sender_id<>recipient_id),
  CHECK(expires_at>sent_at)
);
CREATE INDEX IF NOT EXISTS ix_private_messages_history
  ON private_chat_messages(sender_id,recipient_id,sent_at DESC,id);
CREATE INDEX IF NOT EXISTS ix_private_message_expiry ON private_chat_messages(expires_at);
CREATE TABLE IF NOT EXISTS private_chat_seen (
  message_id UUID NOT NULL REFERENCES private_chat_messages(id) ON DELETE CASCADE,
  reader_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  actually_visible_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY(message_id,reader_id)
);
-- Public chat retention is configurable; none has been approved.
CREATE TABLE IF NOT EXISTS public_chat_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  room_instance_id TEXT NOT NULL,
  request_id TEXT NOT NULL,
  body TEXT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(sender_id,request_id)
);
CREATE INDEX IF NOT EXISTS ix_public_chat_room_time ON public_chat_messages(room_instance_id,sent_at DESC);

-- Daily quests. Reset timezone, rewards, quotas remain admin-configured.
CREATE TABLE IF NOT EXISTS quest_definitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quest_code TEXT NOT NULL UNIQUE,
  event_type TEXT NOT NULL,
  target_quantity BIGINT NOT NULL CHECK(target_quantity>0),
  reward_json JSONB NOT NULL,
  config_version_id UUID REFERENCES game_config_versions(id),
  is_active BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE IF NOT EXISTS character_daily_quests (
  character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  quest_id UUID NOT NULL REFERENCES quest_definitions(id),
  reset_date DATE NOT NULL,
  progress BIGINT NOT NULL DEFAULT 0 CHECK(progress>=0),
  completed_at TIMESTAMPTZ,
  claimed_at TIMESTAMPTZ,
  PRIMARY KEY(character_id,quest_id,reset_date)
);

-- Admin must be a separate identity domain so beta player reset keeps Admin.
CREATE TABLE IF NOT EXISTS admin_users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  login_name TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  totp_secret_encrypted BYTEA,
  totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_roles (
  role_code TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS admin_permissions (
  permission_code TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS admin_user_roles (
  admin_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  role_code TEXT NOT NULL REFERENCES admin_roles(role_code),
  PRIMARY KEY(admin_id,role_code)
);
CREATE TABLE IF NOT EXISTS admin_role_permissions (
  role_code TEXT NOT NULL REFERENCES admin_roles(role_code),
  permission_code TEXT NOT NULL REFERENCES admin_permissions(permission_code),
  PRIMARY KEY(role_code,permission_code)
);
CREATE TABLE IF NOT EXISTS admin_asset_adjustments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  target_character_id UUID REFERENCES characters(id) ON DELETE SET NULL,
  target_character_snapshot UUID NOT NULL,
  asset_kind TEXT NOT NULL,
  asset_ref TEXT,
  requested_delta BIGINT NOT NULL CHECK(requested_delta<>0),
  reason TEXT NOT NULL,
  proposer_id UUID NOT NULL REFERENCES admin_users(id),
  approver_id UUID REFERENCES admin_users(id),
  state TEXT NOT NULL CHECK(state IN ('pending','approved','rejected','executed','failed')),
  transaction_id UUID REFERENCES transaction_requests(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at TIMESTAMPTZ,
  CHECK (approver_id IS NULL OR approver_id<>proposer_id)
);
CREATE TABLE IF NOT EXISTS admin_audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id UUID REFERENCES admin_users(id),
  action_code TEXT NOT NULL,
  object_type TEXT NOT NULL,
  object_id TEXT,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
-- NOTES:
-- (1) No master-data INSERTs here; ADMIN fields must be configured first.
-- (2) Trigger/policy preventing UPDATE/DELETE on asset_ledger needs careful
--     Beta reset design; prefer append-only SQL role with INSERT/SELECT only.
-- (3) Earning and spending must operate atomically with transaction_requests.
-- (4) App-level validation must enforce bag slot cap, stack by catalog type,
--     equipment ownership, 100 friends, crop pest window and time logic.
-- (5) This is design DRAFT, not safe automatic deployment for M1 prototype.
