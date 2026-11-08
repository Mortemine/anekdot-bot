DROP INDEX idx_likes_anekdot_id;

ALTER INDEX idx_likes_user_id_created_at RENAME TO idx_favorites_user_id_created_at;

ALTER TABLE likes RENAME CONSTRAINT likes_anekdot_id_fkey TO favorites_anekdot_id_fkey;
ALTER TABLE likes RENAME CONSTRAINT likes_user_id_fkey TO favorites_user_id_fkey;
ALTER TABLE likes RENAME CONSTRAINT likes_pkey TO favorites_pkey;

ALTER TABLE likes RENAME TO favorites;
