-- ==============================================================================
-- FraudDb: Initial Schema (risk_rule, risk_assessment, triggered_rule)
-- ==============================================================================

-- 1. Таблица правил риска
CREATE TABLE IF NOT EXISTS risk_rule (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64) NOT NULL UNIQUE,
    name        VARCHAR(128) NOT NULL,
    weight      SMALLINT NOT NULL CHECK (weight >= 0 AND weight <= 100),
    threshold   NUMERIC(18, 2),
    is_enabled  BOOLEAN NOT NULL DEFAULT true
);

-- Индексы для таблицы risk_rule
CREATE INDEX IF NOT EXISTS idx_risk_rule_code ON risk_rule(code);
CREATE INDEX IF NOT EXISTS idx_risk_rule_is_enabled ON risk_rule(is_enabled);

-- 2. Таблица оценок риска транзакций
CREATE TABLE IF NOT EXISTS risk_assessment (
    id          BIGSERIAL PRIMARY KEY,
    payment_uid UUID NOT NULL UNIQUE,
    user_uid    UUID NOT NULL,
    risk_score  SMALLINT NOT NULL CHECK (risk_score >= 0 AND risk_score <= 100),
    risk_level  VARCHAR(16) NOT NULL,
    decision    VARCHAR(16) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    duration_ms INTEGER NOT NULL DEFAULT 0
);

-- Индексы для таблицы risk_assessment
CREATE INDEX IF NOT EXISTS idx_risk_assessment_payment_uid ON risk_assessment(payment_uid);
CREATE INDEX IF NOT EXISTS idx_risk_assessment_user_uid ON risk_assessment(user_uid);
CREATE INDEX IF NOT EXISTS idx_risk_assessment_risk_level ON risk_assessment(risk_level);
CREATE INDEX IF NOT EXISTS idx_risk_assessment_analyzed_at ON risk_assessment(analyzed_at);

-- 3. Таблица сработавших правил
CREATE TABLE IF NOT EXISTS triggered_rule (
    id            BIGSERIAL PRIMARY KEY,
    assessment_id BIGINT NOT NULL REFERENCES risk_assessment(id) ON DELETE CASCADE,
    rule_id       BIGINT NOT NULL REFERENCES risk_rule(id) ON DELETE RESTRICT,
    payment_uid   UUID NOT NULL,
    user_uid      UUID NOT NULL,
    score_delta   SMALLINT NOT NULL CHECK (score_delta >= 0),
    details       VARCHAR(256)
);

-- Индексы для таблицы triggered_rule (включая payment_uid и user_uid)
CREATE INDEX IF NOT EXISTS idx_triggered_rule_assessment_id ON triggered_rule(assessment_id);
CREATE INDEX IF NOT EXISTS idx_triggered_rule_rule_id ON triggered_rule(rule_id);
CREATE INDEX IF NOT EXISTS idx_triggered_rule_payment_uid ON triggered_rule(payment_uid);
CREATE INDEX IF NOT EXISTS idx_triggered_rule_user_uid ON triggered_rule(user_uid);

-- 4. Первичные данные: 8 правил анализа риска с весами
INSERT INTO risk_rule (code, name, weight, threshold, is_enabled) VALUES
('AMOUNT_ANOMALY', 'Аномальный размер платежа относительно средней и максимальной суммы', 25, 3.00, true),
('VELOCITY', 'Подозрительная частота операций за минуту и за час', 20, 5.00, true),
('PAYMENT_SEQUENCE', 'Серия платежей подряд с нарастающими суммами', 15, NULL, true),
('UNUSUAL_TIME', 'Время совершения операции вне привычных часов пользователя', 10, NULL, true),
('BEHAVIOR_DEVIATION', 'Отличие параметров операции от прошлого поведения пользователя', 15, NULL, true),
('NEW_DEVICE', 'Использование ранее неизвестного или недоверенного устройства', 15, NULL, true),
('NETWORK_ANOMALY', 'Подключение из новой сети, неизвестного IP или смены страны', 20, NULL, true),
('ACTIVITY_CHANGE', 'Резкое изменение ключевых характеристик активности профиля', 15, NULL, true)
ON CONFLICT (code) DO NOTHING;
