DROP TABLE IF EXISTS telegram_link_codes;

DROP INDEX IF EXISTS users_telegram_user_id_unique_idx;

ALTER TABLE users
DROP COLUMN IF EXISTS telegram_user_id;