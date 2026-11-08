CREATE TABLE favorites (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    anekdot_id bigint NOT NULL REFERENCES anekdots (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, anekdot_id)
);

CREATE INDEX idx_favorites_user_id_created_at ON favorites (user_id, created_at DESC);
