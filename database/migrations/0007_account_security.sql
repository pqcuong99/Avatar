-- Project Thanh M1 completion: optional recovery email, password reset with a
-- 10-minute one-time code that revokes every session (P0-011, 014..016), and
-- optional player TOTP with one-time recovery codes (P0-146). Login names and
-- the M1 tables keep their shape; only columns are added.

ALTER TABLE accounts
    ADD COLUMN email TEXT,
    ADD COLUMN email_verified_at TIMESTAMPTZ,
    -- AES-GCM ciphertext; the key is outside the database (TOTP_ENCRYPTION_KEY).
    ADD COLUMN totp_secret_enc BYTEA,
    ADD COLUMN totp_enabled_at TIMESTAMPTZ,
    ADD COLUMN totp_pending_secret_enc BYTEA,
    ADD COLUMN ban_reason TEXT,
    ADD CONSTRAINT accounts_totp_enabled_has_secret CHECK (totp_enabled_at IS NULL OR totp_secret_enc IS NOT NULL),
    ADD CONSTRAINT accounts_verified_has_email CHECK (email_verified_at IS NULL OR email IS NOT NULL);

-- One-time codes sent by email: verification of the recovery address and
-- password reset. Only a hash is stored; each code works once, until expiry.
CREATE TABLE account_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    email TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX ix_account_codes_lookup ON account_codes (account_id, purpose, created_at DESC);

CREATE TABLE account_recovery_codes (
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    used_at TIMESTAMPTZ,
    PRIMARY KEY (account_id, code_hash)
);

-- Second login step when TOTP is on: the password step yields a short-lived
-- challenge, the TOTP step turns it into a session.
CREATE TABLE login_challenges (
    token_hash BYTEA PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    attempts INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);
