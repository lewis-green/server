package devices

import (
	"time"

	"gorm.io/gorm"
)

type SelectFilter func(*selectFilter)

func WithID(id string) SelectFilter {
	return func(f *selectFilter) {
		f.id = &id
	}
}

func WithToken(token string) SelectFilter {
	return func(f *selectFilter) {
		f.token = &token
	}
}

func WithUserID(userID string) SelectFilter {
	return func(f *selectFilter) {
		f.userID = &userID
	}
}

func ActiveWithin(duration time.Duration) SelectFilter {
	return func(f *selectFilter) {
		f.activeWithin = duration
	}
}

// Sendable restricts the selection to devices not currently in a service
// cooldown (i.e. believed to have cellular service).
func Sendable() SelectFilter {
	return func(f *selectFilter) {
		f.sendableOnly = true
	}
}

type selectFilter struct {
	id           *string
	userID       *string
	token        *string
	activeWithin time.Duration
	sendableOnly bool
}

func newFilter(filters ...SelectFilter) *selectFilter {
	f := new(selectFilter)
	f.merge(filters...)
	return f
}

func (f *selectFilter) merge(filters ...SelectFilter) {
	for _, filter := range filters {
		filter(f)
	}
}

func (f *selectFilter) apply(query *gorm.DB) *gorm.DB {
	if f.id != nil {
		query = query.Where("id = ?", *f.id)
	}
	if f.token != nil {
		query = query.Where("auth_token = ?", *f.token)
	}
	if f.userID != nil {
		query = query.Where("user_id = ?", *f.userID)
	}
	if f.activeWithin != 0 {
		query = query.Where("last_seen > ?", time.Now().Add(-f.activeWithin))
	}
	if f.sendableOnly {
		// Parenthesised: raw-string conditions are ANDed without wrapping, so a
		// bare OR here would break operator precedence against the other filters.
		query = query.Where("(service_degraded_until IS NULL OR service_degraded_until <= ?)", time.Now())
	}
	return query
}
