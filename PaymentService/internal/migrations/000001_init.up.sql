-- ==============================================================================
-- PaymentDb: Initial Schema (payment, payment_status_history)
-- ==============================================================================

-- 1. Таблица платежей
CREATE TABLE IF NOT EXISTS payment (
                                       id              BIGSERIAL PRIMARY KEY,
                                       uid             UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
                                       user_uid        UUID NOT NULL,
                                       external_id     VARCHAR(64),
                                       idempotency_key VARCHAR(64) NOT NULL,
                                       source          VARCHAR(16) NOT NULL DEFAULT 'web',
                                       amount          NUMERIC(18, 2) NOT NULL,
                                       currency        CHAR(3) NOT NULL DEFAULT 'RUB',
                                       operation_type  VARCHAR(32) NOT NULL,
                                       operation_time  TIMESTAMPTZ NOT NULL DEFAULT now(),
                                       device_uid      VARCHAR(64),
                                       ip_address      INET,
                                       status          VARCHAR(16) NOT NULL DEFAULT 'PENDING',
                                       risk_score      SMALLINT NOT NULL DEFAULT 0,
                                       risk_level      VARCHAR(16) NOT NULL DEFAULT 'LOW',
                                       analyzed_at     TIMESTAMPTZ,
                                       created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Уникальный индекс для обеспечения идемпотентности по связке (user_uid + idempotency_key)
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_user_idempotency ON payment (user_uid, idempotency_key);

-- Уникальный индекс по external_id для внешних интеграций
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_external_id ON payment (external_id) WHERE external_id IS NOT NULL;

-- Индексы для ускорения поиска и выборок
CREATE INDEX IF NOT EXISTS idx_payment_user_uid ON payment (user_uid);
CREATE INDEX IF NOT EXISTS idx_payment_status ON payment (status);
CREATE INDEX IF NOT EXISTS idx_payment_created_at ON payment (created_at);

-- 2. Таблица истории изменения статусов платежа
CREATE TABLE IF NOT EXISTS payment_status_history (
                                                      id          BIGSERIAL PRIMARY KEY,
                                                      payment_id  BIGINT NOT NULL REFERENCES payment(id) ON DELETE CASCADE,
                                                      old_status  VARCHAR(16) NOT NULL,
                                                      new_status  VARCHAR(16) NOT NULL,
                                                      changed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
                                                      reason      VARCHAR(256)
);

-- Индексы для таблицы payment_status_history
CREATE INDEX IF NOT EXISTS idx_payment_status_history_payment_id ON payment_status_history (payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_status_history_changed_at ON payment_status_history (changed_at);
