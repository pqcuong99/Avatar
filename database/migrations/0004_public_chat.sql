-- Project Thanh M5 (part): public room chat (MVP-03).
-- Messages reach members of the same room only. Retention, maximum length and
-- moderation rules are [CẦN CẤU HÌNH] (GAP-11): nothing here purges rows yet.
-- request_id makes a retried send idempotent (S-10).

CREATE TABLE public_chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    room_id TEXT NOT NULL,
    request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 8 AND 64),
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (sender_id, request_id)
);
CREATE INDEX ix_public_chat_room_time ON public_chat_messages (room_id, sent_at DESC);
