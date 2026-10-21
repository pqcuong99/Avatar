-- Project Thanh M5: friends (TH-07), private chat (TH-05) and one-way private
-- chat blocks (TH-06). Schema is [ĐỀ XUẤT]; limits the database cannot see
-- (100 friends per character) are enforced by the Game Server in the same
-- transaction that locks both characters.

-- At most one pending request per unordered pair, in either direction.
CREATE TABLE friend_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    receiver_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('pending', 'accepted', 'rejected', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    CHECK (sender_id <> receiver_id),
    CHECK ((state = 'pending') = (decided_at IS NULL))
);
CREATE UNIQUE INDEX ux_friend_requests_pending_pair
    ON friend_requests (LEAST(sender_id, receiver_id), GREATEST(sender_id, receiver_id)) WHERE state = 'pending';
CREATE INDEX ix_friend_requests_receiver ON friend_requests (receiver_id, created_at DESC) WHERE state = 'pending';
CREATE INDEX ix_friend_requests_sender ON friend_requests (sender_id, created_at DESC) WHERE state = 'pending';

-- One row per friendship, stored with character_a < character_b.
CREATE TABLE friendships (
    character_a UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    character_b UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_a, character_b),
    CHECK (character_a < character_b)
);
CREATE INDEX ix_friendships_b ON friendships (character_b);

-- One-way: blocker no longer receives private messages from blocked (TH-06-001).
CREATE TABLE private_chat_blocks (
    blocker_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
CREATE INDEX ix_private_chat_blocks_recent ON private_chat_blocks (blocker_id, created_at DESC, blocked_id);

-- Each message expires 30 days after it was sent (TH-05-001); reads filter on
-- expires_at even when the purge job has not run yet. seen_at is set only by
-- the recipient, for messages that were actually on screen (TH-05-003).
CREATE TABLE private_chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 8 AND 64),
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    seen_at TIMESTAMPTZ,
    UNIQUE (sender_id, request_id),
    CHECK (sender_id <> recipient_id),
    CHECK (expires_at > sent_at),
    CHECK (seen_at IS NULL OR seen_at >= sent_at)
);
-- Conversation history in both directions, newest first.
CREATE INDEX ix_private_messages_pair ON private_chat_messages (
    LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id), sent_at DESC, id DESC);
CREATE INDEX ix_private_messages_unread ON private_chat_messages (recipient_id, sender_id) WHERE seen_at IS NULL;
CREATE INDEX ix_private_messages_expiry ON private_chat_messages (expires_at);
