package devices

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/db"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	devices *Repository
	cache   *cache

	idGen db.IDGen

	logger *zap.Logger
}

func NewService(
	config Config,
	devices *Repository,
	idGen db.IDGen,
	logger *zap.Logger,
) *Service {
	return &Service{
		config: config,

		devices: devices,
		cache:   newCache(),

		idGen: idGen,

		logger: logger,
	}
}

func (s *Service) Insert(ctx context.Context, userID string, device DeviceInfo) (*Device, error) {
	input := DeviceInput{
		DeviceInfo: device,
		ID:         s.idGen(),
		UserID:     userID,
		AuthToken:  s.idGen(),
	}

	return s.devices.Insert(ctx, input)
}

// Select returns a list of devices for a specific user that match the provided filters.
func (s *Service) Select(ctx context.Context, userID string, filter ...SelectFilter) ([]Device, error) {
	filter = append(filter, WithUserID(userID))

	return s.devices.Select(ctx, filter...)
}

// Exists checks if there exists a device that matches the provided filters.
//
// If the device does not exist, it returns false and nil error. If there is an
// error during the query, it returns false and the error. Otherwise, it returns
// true and nil error.
func (s *Service) Exists(ctx context.Context, userID string, filter ...SelectFilter) (bool, error) {
	filter = append(filter, WithUserID(userID))

	return s.devices.Exists(ctx, filter...)
}

// Get returns a single device based on the provided filters for a specific user.
// It ensures that the filter includes the user's ID. If no device matches the
// criteria, it returns ErrNotFound. If more than one device matches, it returns
// ErrMoreThanOne.
func (s *Service) Get(ctx context.Context, userID string, filter ...SelectFilter) (*Device, error) {
	filter = append(filter, WithUserID(userID))

	return s.devices.Get(ctx, filter...)
}

// LoadFunc reports the number of pending messages for each of the provided
// device IDs. Devices absent from the returned map are treated as having zero
// pending messages. It lets balanced selection query message load without the
// devices module depending on the messages module.
type LoadFunc func(deviceIDs []string) (map[string]int, error)

// selectCandidates returns the devices eligible for automatic selection for the
// given user, optionally narrowed to a single device ID and/or to devices seen
// within the provided duration.
//
// For automatic selection (no pinned device) it prefers devices that are
// recently active (server default) and not in a service cooldown. These are
// soft preferences: if they would leave no candidates, it falls back to the
// base set so a message is still attempted. An explicit deviceID or an explicit
// per-request duration is always honoured as-is.
func (s *Service) selectCandidates(ctx context.Context, userID string, deviceID string, duration time.Duration) ([]Device, error) {
	// base filters, rebuilt fresh each call to avoid slice-aliasing between the
	// preferred and fallback queries.
	base := func() []SelectFilter {
		f := []SelectFilter{WithUserID(userID)}
		if deviceID != "" {
			f = append(f, WithID(deviceID))
		}
		if duration > 0 {
			f = append(f, ActiveWithin(duration))
		}
		return f
	}

	if deviceID == "" {
		preferred := base()
		softened := false
		if duration == 0 && s.config.DefaultActiveWithin > 0 {
			preferred = append(preferred, ActiveWithin(s.config.DefaultActiveWithin))
			softened = true
		}
		if s.config.ServiceCooldown > 0 {
			preferred = append(preferred, Sendable())
			softened = true
		}

		if softened {
			devices, err := s.devices.Select(ctx, preferred...)
			if err != nil {
				return nil, err
			}
			if len(devices) > 0 {
				return devices, nil
			}
		}
	}

	return s.devices.Select(ctx, base()...)
}

// GetForSending selects a device to enqueue a message on, honouring the
// configured selection strategy. deviceID pins a specific device; duration
// limits selection to devices active within it; load supplies pending-message
// counts and is only consulted by the least-loaded strategy (never called for
// random selection, so it incurs no query in that mode).
func (s *Service) GetForSending(ctx context.Context, userID string, deviceID string, duration time.Duration, load LoadFunc) (*Device, error) {
	if s.config.SelectionStrategy == SelectionStrategyRandom {
		return s.GetAny(ctx, userID, deviceID, duration)
	}

	return s.GetLeastLoaded(ctx, userID, deviceID, duration, load)
}

func (s *Service) GetAny(ctx context.Context, userID string, deviceID string, duration time.Duration) (*Device, error) {
	devices, err := s.selectCandidates(ctx, userID, deviceID, duration)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, ErrNotFound
	}

	if len(devices) == 1 {
		return &devices[0], nil
	}

	idx := rand.IntN(len(devices)) //nolint:gosec //not critical

	return &devices[idx], nil
}

// GetLeastLoaded selects the eligible device with the fewest pending messages.
//
// It applies the same filters as GetAny. When more than one device is eligible
// it picks the one with the lowest pending-message count reported by load,
// breaking ties randomly. If load is nil it falls back to random selection.
func (s *Service) GetLeastLoaded(ctx context.Context, userID string, deviceID string, duration time.Duration, load LoadFunc) (*Device, error) {
	devices, err := s.selectCandidates(ctx, userID, deviceID, duration)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, ErrNotFound
	}

	if len(devices) == 1 {
		return &devices[0], nil
	}

	if load == nil {
		idx := rand.IntN(len(devices)) //nolint:gosec //not critical

		return &devices[idx], nil
	}

	ids := make([]string, len(devices))
	for i := range devices {
		ids[i] = devices[i].ID
	}

	counts, err := load(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get device load: %w", err)
	}

	return pickLeastLoaded(devices, counts), nil
}

// pickLeastLoaded returns the device with the lowest count. Devices missing from
// counts are treated as zero. Ties are broken randomly by shuffling first.
func pickLeastLoaded(devices []Device, counts map[string]int) *Device {
	rand.Shuffle(len(devices), func(i, j int) { //nolint:gosec //not critical
		devices[i], devices[j] = devices[j], devices[i]
	})

	best := 0
	for i := 1; i < len(devices); i++ {
		if counts[devices[i].ID] < counts[devices[best].ID] {
			best = i
		}
	}

	return &devices[best]
}

// MarkServiceDegraded records that the device reported a no-service send
// failure, putting it into a cooldown during which automatic selection skips
// it. It is a no-op when the feature is disabled.
func (s *Service) MarkServiceDegraded(ctx context.Context, deviceID string) error {
	if s.config.ServiceCooldown <= 0 {
		return nil
	}

	return s.devices.SetServiceDegradedUntil(ctx, deviceID, time.Now().Add(s.config.ServiceCooldown))
}

// ClearServiceDegraded lifts a device's service cooldown, e.g. after it reports
// a successful send. It is a no-op when the feature is disabled.
func (s *Service) ClearServiceDegraded(ctx context.Context, deviceID string) error {
	if s.config.ServiceCooldown <= 0 {
		return nil
	}

	return s.devices.ClearServiceDegraded(ctx, deviceID)
}

// GetByToken returns a device by token.
//
// This method is used to retrieve a device by its auth token. If the device
// does not exist, it returns ErrNotFound.
func (s *Service) GetByToken(ctx context.Context, token string) (*Device, error) {
	device, err := s.cache.GetByToken(token)
	if err == nil {
		return &device, nil
	}

	devicePtr, err := s.devices.Get(ctx, WithToken(token))
	if err != nil {
		return nil, err
	}

	if setErr := s.cache.Set(*devicePtr); setErr != nil {
		s.logger.Error("failed to cache device", zap.String("device_id", devicePtr.ID), zap.Error(setErr))
	}

	return devicePtr, nil
}

func (s *Service) Update(ctx context.Context, id string, device DeviceUpdate) error {
	if err := s.devices.Update(ctx, id, device); err != nil {
		return err
	}

	if cacheErr := s.cache.DeleteByID(id); cacheErr != nil {
		s.logger.Error("failed to invalidate cache",
			zap.String("device_id", id),
			zap.Error(cacheErr),
		)
	}

	return nil
}

func (s *Service) SetLastSeen(ctx context.Context, batch map[string]time.Time) error {
	if err := s.devices.SetLastSeenBatch(ctx, batch); err != nil {
		s.logger.Error("failed to set last seen batch", zap.Error(err))
		return fmt.Errorf("failed to set last seen batch: %w", err)
	}
	return nil
}

// Remove removes devices for a specific user that match the provided filters.
// It ensures that the filter includes the user's ID.
func (s *Service) Remove(ctx context.Context, userID string, filter ...SelectFilter) error {
	filter = append(filter, WithUserID(userID))

	devices, err := s.devices.Select(ctx, filter...)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil
	}

	for _, device := range devices {
		if cacheErr := s.cache.DeleteByID(device.ID); cacheErr != nil {
			s.logger.Error("failed to invalidate cache",
				zap.String("device_id", device.ID),
				zap.Error(cacheErr),
			)
		}
	}

	if rmErr := s.devices.Remove(ctx, filter...); rmErr != nil {
		return rmErr
	}

	return nil
}
