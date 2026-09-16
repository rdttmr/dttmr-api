CREATE TABLE IF NOT EXISTS invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inviter_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    invitee_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    code VARCHAR(64) NOT NULL,
    consumed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_invites_code ON invites (code);
CREATE INDEX idx_invites_inviter_user_id ON invites (inviter_user_id);
