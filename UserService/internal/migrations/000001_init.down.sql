-- ==============================================================================
-- UserDb: Rollback Schema
-- ==============================================================================

-- Удаление таблиц в обратном порядке внешних ключей
DROP TABLE IF EXISTS known_network CASCADE;
DROP TABLE IF EXISTS device CASCADE;
DROP TABLE IF EXISTS user_profile CASCADE;
DROP TABLE IF EXISTS "user" CASCADE;
