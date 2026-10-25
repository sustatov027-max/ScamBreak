package domain

import "time"

type KnownNetwork struct {
	ID          int64     `json:"id"`
	ProfileID   int64     `json:"profile_id"`
	IPAddress   string    `json:"ip_address"` // тип INET в Postgres
	CountryCode string    `json:"country_code"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
