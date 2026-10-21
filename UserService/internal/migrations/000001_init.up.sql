-- ==============================================================================
-- UserDb: Initial Schema (user, user_profile, device, known_network)
-- ==============================================================================

-- 1. Таблица пользователей (аутентификация и учетные записи)
CREATE TABLE IF NOT EXISTS "user" (
    id            BIGSERIAL PRIMARY KEY,
    uid           UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    email         VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    role          VARCHAR(16) NOT NULL DEFAULT 'User',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы для таблицы "user"
CREATE INDEX IF NOT EXISTS idx_user_role ON "user"(role);
CREATE INDEX IF NOT EXISTS idx_user_is_active ON "user"(is_active);

-- 2. Таблица профиля пользователя и истории платежной активности
CREATE TABLE IF NOT EXISTS user_profile (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
    full_name       VARCHAR(128) NOT NULL,
    country_code    CHAR(2) NOT NULL,
    phone           VARCHAR(32),
    home_timezone   VARCHAR(64) NOT NULL DEFAULT 'UTC',
    payments_count  INTEGER NOT NULL DEFAULT 0,
    avg_amount      NUMERIC(18, 2) NOT NULL DEFAULT 0.00,
    max_amount      NUMERIC(18, 2) NOT NULL DEFAULT 0.00,
    last_payment_at TIMESTAMPTZ,
    usual_hour_from SMALLINT NOT NULL DEFAULT 0,
    usual_hour_to   SMALLINT NOT NULL DEFAULT 23,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы для таблицы user_profile
CREATE INDEX IF NOT EXISTS idx_user_profile_user_id ON user_profile(user_id);
CREATE INDEX IF NOT EXISTS idx_user_profile_phone ON user_profile(phone) WHERE phone IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_profile_country_code ON user_profile(country_code);

-- 3. Таблица используемых устройств
CREATE TABLE IF NOT EXISTS device (
    id            BIGSERIAL PRIMARY KEY,
    profile_id    BIGINT NOT NULL REFERENCES user_profile(id) ON DELETE CASCADE,
    device_uid    VARCHAR(64) NOT NULL,
    device_type   VARCHAR(32) NOT NULL,
    user_agent    VARCHAR(256),
    is_trusted    BOOLEAN NOT NULL DEFAULT false,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы для таблицы device
CREATE INDEX IF NOT EXISTS idx_device_profile_id ON device(profile_id);
CREATE INDEX IF NOT EXISTS idx_device_device_uid ON device(device_uid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_device_profile_device_uid ON device(profile_id, device_uid);

-- 4. Таблица известных сетевых подключений (IP-адреса и подсети)
CREATE TABLE IF NOT EXISTS known_network (
    id            BIGSERIAL PRIMARY KEY,
    profile_id    BIGINT NOT NULL REFERENCES user_profile(id) ON DELETE CASCADE,
    ip_address    INET NOT NULL,
    country_code  CHAR(2) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы для таблицы known_network
CREATE INDEX IF NOT EXISTS idx_known_network_profile_id ON known_network(profile_id);
CREATE INDEX IF NOT EXISTS idx_known_network_ip_address ON known_network(ip_address);
CREATE UNIQUE INDEX IF NOT EXISTS idx_known_network_profile_ip ON known_network(profile_id, ip_address);
