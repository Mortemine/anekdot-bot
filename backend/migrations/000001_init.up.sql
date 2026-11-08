CREATE TABLE categories (
    id         int PRIMARY KEY,
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE anekdots (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id int NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    text        text NOT NULL CHECK (length(text) > 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_anekdots_category_id ON anekdots (category_id);

-- Guards the seeder against duplicate rows when it is re-run.
CREATE UNIQUE INDEX idx_anekdots_category_text ON anekdots (category_id, md5(text));

CREATE TABLE feedback_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message    text NOT NULL CHECK (length(message) BETWEEN 1 AND 4000),
    contact    text CHECK (contact IS NULL OR length(contact) <= 200),
    status     text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'read', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_feedback_messages_created_at ON feedback_messages (created_at DESC);
CREATE INDEX idx_feedback_messages_status ON feedback_messages (status) WHERE status = 'new';
