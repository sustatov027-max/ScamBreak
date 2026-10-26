package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sustatov027-max/ScamBreak/UserService/internal/domain"
)

// ProfileRepository реализует доступ к таблице "user_profile" с использованием чистого SQL.
type ProfileRepository struct {
	db DBTX
}

// NewProfileRepository создаёт новый экземпляр ProfileRepository.
func NewProfileRepository(db DBTX) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// WithTX возвращает копию репозитория, привязанную к транзакции pgx.Tx.
func (p *ProfileRepository) WithTX(tx pgx.Tx) *ProfileRepository {
	return &ProfileRepository{db: tx}
}

// Insert сохраняет новый профиль пользователя в БД.
func (p *ProfileRepository) Insert(ctx context.Context, profile *domain.UserProfile) error {
	// Дефолтные значения для обязательных полей
	if profile.CountryCode == "" {
		profile.CountryCode = "RU"
	}
	if profile.HomeTimezone == "" {
		profile.HomeTimezone = "UTC"
	}
	if profile.UsualHourTo == 0 && profile.UsualHourFrom == 0 {
		profile.UsualHourTo = 23
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}

	// Обработка nullable полей
	var phone *string
	if profile.Phone != "" {
		phone = &profile.Phone
	}

	var lastPaymentAt *time.Time
	if !profile.LastPaymentAt.IsZero() {
		lastPaymentAt = &profile.LastPaymentAt
	}

	query := `
		INSERT INTO user_profile (
			  user_id,
			  full_name,
			  country_code,
			  phone,
			  home_timezone,
			  payments_count,
			  avg_amount,
			  max_amount,
			  last_payment_at,
			  usual_hour_from,
			  usual_hour_to,
			  updated_at
		)  
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, updated_at;
	`

	err := p.db.QueryRow(ctx, query,
		profile.UserID,
		profile.FullName,
		profile.CountryCode,
		phone,
		profile.HomeTimezone,
		profile.PaymentsCount,
		profile.AvgAmount,
		profile.MaxAmount,
		lastPaymentAt,
		profile.UsualHourFrom,
		profile.UsualHourTo,
		profile.UpdatedAt,
	).Scan(&profile.ID, &profile.UpdatedAt)

	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case "23505":
				return fmt.Errorf("профиль для пользователя %d уже существует: %w", profile.UserID, err)
			case "23503":
				return domain.ErrUserNotFound
			}
		}
		return fmt.Errorf("ошибка вставки профиля пользователя: %w", err)
	}

	return nil
}

// FindByUserID ищет профиль пользователя по его ID
func (p *ProfileRepository) FindByUserID(ctx context.Context, userID int64) (*domain.UserProfile, error) {
	query := `
		SELECT 
			id, 
			user_id,
			full_name,
			country_code,
			phone,
			home_timezone,
			payments_count,
			avg_amount,
			max_amount,
			last_payment_at,
			usual_hour_from,
			usual_hour_to,
			updated_at
		FROM user_profile
		WHERE user_id = $1;
	`

	var profile domain.UserProfile
	var phone *string
	var lastPaymentAt *time.Time

	err := p.db.QueryRow(ctx, query, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.FullName,
		&profile.CountryCode,
		&phone,
		&profile.HomeTimezone,
		&profile.PaymentsCount,
		&profile.AvgAmount,
		&profile.MaxAmount,
		&lastPaymentAt,
		&profile.UsualHourFrom,
		&profile.UsualHourTo,
		&profile.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProfileNotFound
		}
		return nil, fmt.Errorf("ошибка поиска профиля по id пользователя: %w", err)
	}

	if phone != nil {
		profile.Phone = *phone
	}
	if lastPaymentAt != nil {
		profile.LastPaymentAt = *lastPaymentAt
	}

	return &profile, nil
}

// Update обновляет профиль пользователя
func (p *ProfileRepository) Update(ctx context.Context, profile *domain.UserProfile) error {
	now := time.Now()
	query := `
		UPDATE user_profile
		SET full_name = $1,
		    country_code = $2,
		    phone = NULLIF($3, ''),
		    home_timezone = $4,
		    updated_at = $5
		WHERE (id = $6 AND $6 > 0) OR (user_id = $7 AND $7 > 0);
	`

	tag, err := p.db.Exec(ctx, query,
		profile.FullName,
		profile.CountryCode,
		profile.Phone,
		profile.HomeTimezone,
		now,
		profile.ID,
		profile.UserID,
	)
	if err != nil {
		return fmt.Errorf("ошибка обновления профиля пользователя: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrProfileNotFound
	}

	profile.UpdatedAt = now
	return nil
}

// UpsertDevice добавляет устройство пользователя или обновляет его данные при повторном визите
func (p *ProfileRepository) UpsertDevice(ctx context.Context, d *domain.Device) error {
	now := time.Now()
	if d.FirstSeenAt.IsZero() {
		d.FirstSeenAt = now
	}
	if d.LastSeenAt.IsZero() {
		d.LastSeenAt = now
	}

	query := `
		INSERT INTO device (
			profile_id,
			device_uid,
			device_type,
			user_agent,
			is_trusted,
			first_seen_at,
			last_seen_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)
		ON CONFLICT (profile_id, device_uid) DO UPDATE
		SET
			device_type  = EXCLUDED.device_type,
			user_agent   = COALESCE(EXCLUDED.user_agent, device.user_agent),
			is_trusted   = device.is_trusted OR EXCLUDED.is_trusted,
			last_seen_at = EXCLUDED.last_seen_at
		RETURNING id, first_seen_at, last_seen_at;
	`

	err := p.db.QueryRow(ctx, query,
		d.ProfileID,
		d.DeviceUID,
		d.DeviceType,
		d.UserAgent,
		d.IsTrusted,
		d.FirstSeenAt,
		d.LastSeenAt,
	).Scan(&d.ID, &d.FirstSeenAt, &d.LastSeenAt)

	if err != nil {
		return fmt.Errorf("ошибка добавления устройства: %w", err)
	}

	return nil
}

// ListDevices отдаёт полный список всех зарегистрированных устройств профиля
func (p *ProfileRepository) ListDevices(ctx context.Context, profileID int64) ([]domain.Device, error) {
	query := `
		SELECT 
			id,
			profile_id,
			device_uid,
			device_type,
			user_agent,
			is_trusted,
			first_seen_at,
			last_seen_at
		FROM device
		WHERE profile_id = $1
		ORDER BY last_seen_at DESC;
	`

	rows, err := p.db.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса устройств: %w", err)
	}
	defer rows.Close()

	devices := make([]domain.Device, 0)
	for rows.Next() {
		var device domain.Device
		var userAgent *string
		err := rows.Scan(
			&device.ID,
			&device.ProfileID,
			&device.DeviceUID,
			&device.DeviceType,
			&userAgent,
			&device.IsTrusted,
			&device.FirstSeenAt,
			&device.LastSeenAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения устройства: %w", err)
		}
		if userAgent != nil {
			device.UserAgent = *userAgent
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении устройств: %w", err)
	}

	return devices, nil
}

// UpsertNetwork сохраняет известную сеть пользователя без дублей по (profile_id, ip_address).
func (p *ProfileRepository) UpsertNetwork(ctx context.Context, n *domain.KnownNetwork) error {
	now := time.Now()
	if n.FirstSeenAt.IsZero() {
		n.FirstSeenAt = now
	}
	if n.LastSeenAt.IsZero() {
		n.LastSeenAt = now
	}

	query := `
		INSERT INTO known_network (
			profile_id,
			ip_address,
			country_code,
			first_seen_at,
			last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (profile_id, ip_address) DO UPDATE
		SET
			country_code = EXCLUDED.country_code,
			last_seen_at = EXCLUDED.last_seen_at
		RETURNING id, first_seen_at, last_seen_at;
	`

	err := p.db.QueryRow(ctx, query,
		n.ProfileID,
		n.IPAddress,
		n.CountryCode,
		n.FirstSeenAt,
		n.LastSeenAt,
	).Scan(&n.ID, &n.FirstSeenAt, &n.LastSeenAt)

	if err != nil {
		return fmt.Errorf("ошибка upsert сети: %w", err)
	}

	return nil
}

func (p *ProfileRepository) LoadRiskProfile(ctx context.Context, userUID uuid.UUID) (*domain.RiskProfile, error) {
	// Загрузка пользователя и профиля
	query := `
		SELECT 
			u.is_active,
			COALESCE(p.id, 0),
			COALESCE(p.country_code, 'RU'),
			COALESCE(p.home_timezone, 'UTC'),
			COALESCE(p.payments_count, 0),
			COALESCE(p.avg_amount, 0.00),
			COALESCE(p.max_amount, 0.00),
			p.last_payment_at,
			COALESCE(p.usual_hour_from, 0),
			COALESCE(p.usual_hour_to, 23)
		FROM "user" u
		LEFT JOIN user_profile p ON p.user_id = u.id
		WHERE u.uid = $1;
	`

	var (
		isActive      bool
		profileID     int64
		countryCode   string
		homeTimezone  string
		paymentsCount int32
		avgAmount     float64
		maxAmount     float64
		lastPaymentAt *time.Time
		usualHourFrom int32
		usualHourTo   int32
	)

	err := p.db.QueryRow(ctx, query, userUID).Scan(
		&isActive,
		&profileID,
		&countryCode,
		&homeTimezone,
		&paymentsCount,
		&avgAmount,
		&maxAmount,
		&lastPaymentAt,
		&usualHourFrom,
		&usualHourTo,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.RiskProfile{Exists: false}, nil
		}
		return nil, fmt.Errorf("ошибка загрузки профиля: %w", err)
	}

	rp := &domain.RiskProfile{
		Exists:        true,
		IsActive:      isActive,
		CountryCode:   countryCode,
		HomeTimezone:  homeTimezone,
		KnownDevices:  make([]string, 0),
		KnownNetworks: make([]string, 0),
		PaymentsCount: paymentsCount,
		AvgAmount:     avgAmount,
		MaxAmount:     maxAmount,
		LastPaymentAt: lastPaymentAt,
		UsualHourFrom: usualHourFrom,
		UsualHourTo:   usualHourTo,
	}

	if profileID == 0 {
		return rp, nil
	}

	// Загрузка списка известных устройств (device_uid)
	deviceRows, err := p.db.Query(ctx, `SELECT device_uid FROM device WHERE profile_id = $1 ORDER BY last_seen_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки устройств: %w", err)
	}
	defer deviceRows.Close()
	for deviceRows.Next() {
		var dUID string
		if err := deviceRows.Scan(&dUID); err != nil {
			return nil, fmt.Errorf("ошибка сканирования device_uid: %w", err)
		}
		rp.KnownDevices = append(rp.KnownDevices, dUID)
	}
	// Загрузка списка известных сетей (ip_address)
	networkRows, err := p.db.Query(ctx, `SELECT host(ip_address) FROM known_network WHERE profile_id = $1 ORDER BY last_seen_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки сетей: %w", err)
	}
	defer networkRows.Close()
	for networkRows.Next() {
		var ip string
		if err := networkRows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("ошибка сканирования ip: %w", err)
		}
		rp.KnownNetworks = append(rp.KnownNetworks, ip)
	}
	return rp, nil
}
