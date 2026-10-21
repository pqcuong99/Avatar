-- Project Thanh M6/M7: Admin identity separate from players (so a Closed Beta
-- reset keeps it, P0-179), RBAC down to permissions (P0-142), mandatory TOTP
-- (P0-143), two-person asset adjustments (P0-144), audit, and the server
-- maintenance switch used to publish configuration (P0-152).
--
-- Role names are CHỐT (P0-142); which permission each role holds is
-- [ĐỀ XUẤT] and can be changed by a Super Admin later.

CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login_name TEXT NOT NULL UNIQUE CHECK (login_name ~ '^[a-z0-9_.-]{3,32}$'),
    password_hash TEXT NOT NULL,
    totp_secret_enc BYTEA NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    login_generation BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admin_recovery_codes (
    admin_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    used_at TIMESTAMPTZ,
    PRIMARY KEY (admin_id, code_hash)
);

CREATE TABLE admin_roles (
    role_code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL
);
CREATE TABLE admin_permissions (
    permission_code TEXT PRIMARY KEY,
    description TEXT NOT NULL
);
CREATE TABLE admin_role_permissions (
    role_code TEXT NOT NULL REFERENCES admin_roles(role_code) ON DELETE CASCADE,
    permission_code TEXT NOT NULL REFERENCES admin_permissions(permission_code) ON DELETE CASCADE,
    PRIMARY KEY (role_code, permission_code)
);
CREATE TABLE admin_user_roles (
    admin_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    role_code TEXT NOT NULL REFERENCES admin_roles(role_code),
    PRIMARY KEY (admin_id, role_code)
);

INSERT INTO admin_roles (role_code, display_name) VALUES
    ('SUPER_ADMIN', 'Super Admin'),
    ('GAME_ADMIN', 'Game Admin'),
    ('SUPPORT', 'Hỗ trợ'),
    ('FINANCE', 'Tài chính');

INSERT INTO admin_permissions (permission_code, description) VALUES
    ('admin.manage',          'Quản lý tài khoản Admin và phân quyền'),
    ('player.read',           'Tra cứu người chơi, nhân vật, tài sản'),
    ('player.ban',            'Khóa / mở khóa tài khoản người chơi'),
    ('asset.adjust.propose',  'Đề xuất điều chỉnh tài sản thủ công'),
    ('asset.adjust.approve',  'Duyệt điều chỉnh tài sản do Admin khác đề xuất'),
    ('maintenance.manage',    'Bật / tắt bảo trì'),
    ('config.publish',        'Soạn và phát hành phiên bản cấu hình qua bảo trì'),
    ('audit.read',            'Xem nhật ký Admin');

INSERT INTO admin_role_permissions (role_code, permission_code)
SELECT 'SUPER_ADMIN', permission_code FROM admin_permissions;
INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
    ('GAME_ADMIN', 'player.read'), ('GAME_ADMIN', 'player.ban'), ('GAME_ADMIN', 'asset.adjust.propose'),
    ('GAME_ADMIN', 'asset.adjust.approve'), ('GAME_ADMIN', 'maintenance.manage'), ('GAME_ADMIN', 'config.publish'),
    ('GAME_ADMIN', 'audit.read'),
    ('SUPPORT', 'player.read'),
    ('FINANCE', 'player.read'), ('FINANCE', 'audit.read');

CREATE TABLE admin_sessions (
    token_hash BYTEA PRIMARY KEY,
    admin_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    login_generation BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- Second step of every Admin login (TOTP is mandatory).
CREATE TABLE admin_login_challenges (
    token_hash BYTEA PRIMARY KEY,
    admin_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    attempts INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

-- Manual asset changes need a proposer and a different approver; only an
-- approved request is executed, once, by the Transaction Service with the
-- adjustment id as its request id.
CREATE TABLE admin_asset_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_character_id UUID REFERENCES characters(id) ON DELETE SET NULL,
    target_snapshot JSONB NOT NULL,
    asset_kind TEXT NOT NULL CHECK (asset_kind IN ('XU', 'DIAMOND', 'ITEM')),
    item_code TEXT REFERENCES item_definitions(item_code),
    amount_delta BIGINT NOT NULL CHECK (amount_delta <> 0),
    reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 5 AND 500),
    proposer_id UUID NOT NULL REFERENCES admin_users(id),
    approver_id UUID REFERENCES admin_users(id),
    state TEXT NOT NULL CHECK (state IN ('pending', 'rejected', 'executed', 'failed')),
    decision_note TEXT,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    CHECK (approver_id IS NULL OR approver_id <> proposer_id),
    CHECK ((asset_kind = 'ITEM') = (item_code IS NOT NULL)),
    CHECK ((state = 'pending') = (decided_at IS NULL))
);
CREATE INDEX ix_admin_adjustments_state ON admin_asset_adjustments (state, created_at DESC);

-- Audit: who did what, never secrets. Append-only.
CREATE TABLE admin_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_id UUID REFERENCES admin_users(id),
    action TEXT NOT NULL,
    object_type TEXT NOT NULL,
    object_id TEXT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_admin_audit_time ON admin_audit_logs (created_at DESC, id DESC);

CREATE FUNCTION thanh_admin_audit_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'admin_audit_logs is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER trg_admin_audit_append_only
    BEFORE UPDATE OR DELETE ON admin_audit_logs
    FOR EACH ROW EXECUTE FUNCTION thanh_admin_audit_append_only();
CREATE TRIGGER trg_admin_audit_no_truncate
    BEFORE TRUNCATE ON admin_audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION thanh_admin_audit_append_only();

-- Server-wide switches. `maintenance`: {"active": bool, "message": text}.
-- While active, the Transaction Service refuses player asset commands and a
-- draft config version may be published.
CREATE TABLE server_state (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO server_state (key, value) VALUES ('maintenance', '{"active": false, "message": ""}');
