DROP TABLE feedback_messages;

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text NOT NULL UNIQUE CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    display_name  text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 100),
    password_hash text NOT NULL CHECK (length(password_hash) BETWEEN 20 AND 100),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash text PRIMARY KEY CHECK (length(token_hash) = 64),
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

ALTER TABLE anekdots
    ADD COLUMN author_id bigint REFERENCES users (id) ON DELETE SET NULL;

CREATE INDEX idx_anekdots_author_id ON anekdots (author_id, created_at DESC)
    WHERE author_id IS NOT NULL;

CREATE TABLE favorites (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    anekdot_id bigint NOT NULL REFERENCES anekdots (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, anekdot_id)
);

CREATE INDEX idx_favorites_user_id_created_at ON favorites (user_id, created_at DESC);
