ALTER TABLE favorites RENAME TO likes;

ALTER TABLE likes RENAME CONSTRAINT favorites_pkey TO likes_pkey;
ALTER TABLE likes RENAME CONSTRAINT favorites_user_id_fkey TO likes_user_id_fkey;
ALTER TABLE likes RENAME CONSTRAINT favorites_anekdot_id_fkey TO likes_anekdot_id_fkey;

ALTER INDEX idx_favorites_user_id_created_at RENAME TO idx_likes_user_id_created_at;

CREATE INDEX idx_likes_anekdot_id ON likes (anekdot_id);
