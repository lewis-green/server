package devices

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/models"
	"gorm.io/gorm"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrInvalidFilter = errors.New("invalid filter")
	ErrMoreThanOne   = errors.New("more than one record")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Select(filter ...SelectFilter) ([]models.Device, error) {
	if len(filter) == 0 {
		return nil, ErrInvalidFilter
	}

	f := newFilter(filter...)
	devices := []models.Device{}

	return devices, f.apply(r.db).Find(&devices).Error
}

// Exists checks if there exists a device with the given filters.
//
// If the device does not exist, it returns false and nil error. If there is an
// error during the query, it returns false and the error. Otherwise, it returns
// true and nil error.
func (r *Repository) Exists(filters ...SelectFilter) (bool, error) {
	err := newFilter(filters...).apply(r.db).Take(new(models.Device)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) Get(filter ...SelectFilter) (models.Device, error) {
	devices, err := r.Select(filter...)
	if err != nil {
		return models.Device{}, fmt.Errorf("failed to get device: %w", err)
	}

	if len(devices) == 0 {
		return models.Device{}, ErrNotFound
	}

	if len(devices) > 1 {
		return models.Device{}, ErrMoreThanOne
	}

	return devices[0], nil
}

func (r *Repository) Insert(device *models.Device) error {
	return r.db.Create(device).Error
}

func (r *Repository) UpdatePushToken(id string, token *string) error {
	res := r.db.Model((*models.Device)(nil)).Where("id = ?", id).Update("push_token", token)
	if res.Error != nil {
		return fmt.Errorf("failed to update device: %w", res.Error)
	}

	return nil
}

// SetServiceDegradedUntil puts the device into a service cooldown until the
// given time, so automatic selection skips it.
func (r *Repository) SetServiceDegradedUntil(ctx context.Context, id string, until time.Time) error {
	res := r.db.WithContext(ctx).
		Model((*models.Device)(nil)).
		Where("id = ?", id).
		UpdateColumn("service_degraded_until", until)
	if res.Error != nil {
		return fmt.Errorf("failed to set device service state: %w", res.Error)
	}

	return nil
}

// ClearServiceDegraded lifts a device's service cooldown. The IS NOT NULL guard
// makes it a no-op write for the common case where the device isn't degraded.
func (r *Repository) ClearServiceDegraded(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Model((*models.Device)(nil)).
		Where("id = ? AND service_degraded_until IS NOT NULL", id).
		UpdateColumn("service_degraded_until", nil)
	if res.Error != nil {
		return fmt.Errorf("failed to clear device service state: %w", res.Error)
	}

	return nil
}

func (r *Repository) SetLastSeen(ctx context.Context, id string, lastSeen time.Time) error {
	if lastSeen.IsZero() {
		return nil // ignore zero timestamps
	}
	res := r.db.WithContext(ctx).
		Model((*models.Device)(nil)).
		Where("id = ? AND last_seen < ?", id, lastSeen).
		UpdateColumn("last_seen", lastSeen)
	if res.Error != nil {
		return res.Error
	}

	// RowsAffected==0 => not found or stale timestamp; treat as no-op.
	return nil
}

func (r *Repository) Remove(filter ...SelectFilter) error {
	if len(filter) == 0 {
		return ErrInvalidFilter
	}

	f := newFilter(filter...)
	return f.apply(r.db).Delete(new(models.Device)).Error
}

func (r *Repository) Cleanup(ctx context.Context, until time.Time) (int64, error) {
	res := r.db.
		WithContext(ctx).
		Where("last_seen < ?", until).
		Delete(new(models.Device))

	return res.RowsAffected, res.Error
}
