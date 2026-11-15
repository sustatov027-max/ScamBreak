package domain

// AnalysisContext - единый источник данных для всех правил
type AnalysisContext struct {
	Payment       PaymentCreatedEvent // Сырые данные текущей входящей операции
	Profile       *RiskProfile        // Долговременный профиль поведения пользователя
	Count1m       int                 // Частота транзакций за последние 60 секунд
	Count1h       int                 // Частота транзакций за последние 3600 секунд
	RecentAmounts []float64           // Последние N сумм операций (в порядке LIFO)
	Settings      map[string]RiskRule // Актуальные настройки правил из БД (ключ — RuleCode)
}
