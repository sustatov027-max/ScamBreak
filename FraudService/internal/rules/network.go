package rules

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"encoding/json"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/sustatov027-max/ScamBreak/FraudService/internal/domain"
)

type NetworkRule struct {
	defaultWeight int
	httpClient    *http.Client
	cache         *expirable.LRU[string, string]
}

type geoResponse struct {
	CountryCode string `json:"countryCode"`
}

func NewNetworkRule() *NetworkRule {
	// Кэш на 1000 записей со временем жизни 1 час
	cache := expirable.NewLRU[string, string](1000, nil, 1*time.Hour)

	return &NetworkRule{
		defaultWeight: 20,
		httpClient:    &http.Client{Timeout: 2 * time.Second},
		cache:         cache,
	}
}

func (r *NetworkRule) Code() string {
	return "NETWORK_ANOMALY"
}

func (r *NetworkRule) Evaluate(c *domain.AnalysisContext) *domain.RuleResult {
	if c == nil || c.Profile == nil || !c.Profile.Exists || c.Profile.PaymentsCount < 3 {
		return nil
	}

	scoreDelta := r.defaultWeight
	if c.Settings != nil {
		if setting, ok := c.Settings[r.Code()]; ok {
			if !setting.IsEnabled {
				return nil
			}
			if setting.Weight > 0 {
				scoreDelta = setting.Weight
			}
		}
	}

	// 1. Прямая проверка: известен ли IP
	if slices.Contains(c.Profile.KnownNetworks, c.Payment.IPAddress) {
		return nil
	}

	// 2. Мягкое срабатывание: страна та же, но IP новый (с использованием кэша)
	currentCountry := r.getCountryByIP(c.Payment.IPAddress)
	sameCountry := c.Profile.CountryCode != "" && c.Profile.CountryCode == currentCountry

	finalScore := scoreDelta
	details := fmt.Sprintf("операция из неизвестной сети: %s", c.Payment.IPAddress)

	if sameCountry {
		finalScore = scoreDelta / 2
		details = fmt.Sprintf("операция из неизвестной сети (%s), но страна совпадает (%s)", c.Payment.IPAddress, currentCountry)
	}

	return &domain.RuleResult{
		RuleCode:   r.Code(),
		ScoreDelta: finalScore,
		Details:    details,
	}
}

func (r *NetworkRule) getCountryByIP(ip string) string {
	// Проверка в кэше
	if val, ok := r.cache.Get(ip); ok {
		return val
	}

	// Запрос к API
	url := fmt.Sprintf("http://ip-api.com/json/%s", ip)
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return "UNKNOWN"
	}
	defer resp.Body.Close()

	var geo geoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return "UNKNOWN"
	}

	// Сохранение в кэш
	r.cache.Add(ip, geo.CountryCode)

	return geo.CountryCode
}
