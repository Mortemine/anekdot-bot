CREATE TABLE collections (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id   bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    is_public  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_collections_owner_name ON collections (owner_id, lower(name));
CREATE INDEX idx_collections_public ON collections (created_at DESC) WHERE is_public;

CREATE TABLE collection_anekdots (
    collection_id bigint NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
    anekdot_id    bigint NOT NULL REFERENCES anekdots (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (collection_id, anekdot_id)
);

CREATE INDEX idx_collection_anekdots_anekdot_id ON collection_anekdots (anekdot_id);

CREATE TABLE collection_favorites (
    user_id       bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    collection_id bigint NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, collection_id)
);

CREATE INDEX idx_collection_favorites_user_id ON collection_favorites (user_id, created_at DESC);
