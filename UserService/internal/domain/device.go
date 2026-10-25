package domain

import "time"

type Device struct {
	ID          int64     `json:"id"`
	ProfileID   int64     `json:"profile_id"`
	DeviceUID   string    `json:"device_uid"`
	DeviceType  string    `json:"device_type"`
	UserAgent   string    `json:"user_agent,omitempty"`
	IsTrusted   bool      `json:"is_trusted"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
