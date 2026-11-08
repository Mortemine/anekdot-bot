DROP TABLE favorites;

DROP INDEX idx_anekdots_author_id;

ALTER TABLE anekdots DROP COLUMN author_id;

DROP TABLE sessions;
DROP TABLE users;

CREATE TABLE feedback_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message    text NOT NULL CHECK (length(message) BETWEEN 1 AND 4000),
    contact    text CHECK (contact IS NULL OR length(contact) <= 200),
    status     text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'read', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_feedback_messages_created_at ON feedback_messages (created_at DESC);
CREATE INDEX idx_feedback_messages_status ON feedback_messages (status) WHERE status = 'new';
