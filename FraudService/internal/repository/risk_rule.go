package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type RiskRuleRepository interface {
	GetRulesMap(ctx context.Context) (map[string]domain.RiskRule, error)
}

type RiskRuleRepo struct {
	pool *pgxpool.Pool
}

func NewRiskRuleRepository(pool *pgxpool.Pool) *RiskRuleRepo {
	return &RiskRuleRepo{pool: pool}
}

// GetRulesMap загружает активные правила риска из БД и возвращает их в виде map[code]RiskRule
func (r *RiskRuleRepo) GetRulesMap(ctx context.Context) (map[string]domain.RiskRule, error) {
	query := `
		SELECT id, code, name, weight, threshold, is_enabled
		FROM risk_rule
		WHERE is_enabled = true;
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса правил риска: %w", err)
	}
	defer rows.Close()

	rules := make(map[string]domain.RiskRule)
	for rows.Next() {
		var rule domain.RiskRule
		if err := rows.Scan(&rule.ID, &rule.Code, &rule.Name, &rule.Weight, &rule.Threshold, &rule.IsEnabled); err != nil {
			return nil, fmt.Errorf("ошибка сканирования правила риска: %w", err)
		}
		rules[rule.Code] = rule
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации правил риска: %w", err)
	}

	return rules, nil
}
