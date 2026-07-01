package devices

import "time"

// SelectionStrategy controls how a device is chosen when a message is enqueued
// without an explicit device ID.
type SelectionStrategy string

const (
	// SelectionStrategyRandom picks an eligible device uniformly at random.
	SelectionStrategyRandom SelectionStrategy = "random"
	// SelectionStrategyLeastLoaded picks the eligible device with the fewest
	// pending messages, breaking ties randomly.
	SelectionStrategyLeastLoaded SelectionStrategy = "least-loaded"
)

type Config struct {
	// SelectionStrategy is the automatic device selection strategy. Any value
	// other than "random" is treated as "least-loaded".
	SelectionStrategy SelectionStrategy

	// ServiceCooldown is how long a device is skipped for automatic selection
	// after it reports a no-service send failure. Zero disables the feature.
	ServiceCooldown time.Duration
}
