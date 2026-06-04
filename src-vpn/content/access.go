package content

import (
	"errors"
	"sync"
)

// AccessLevel represents the access level for a piece of content.
type AccessLevel string

const (
	AccessPublic  AccessLevel = "public"
	AccessPrivate AccessLevel = "private"
	AccessPaid    AccessLevel = "paid"
)

// AccessPolicy defines who can access a piece of content.
type AccessPolicy struct {
	ContentID    string
	Level        AccessLevel
	AllowedUsers []string // empty = all for public
	Price        int64    // sats for paid content
}

// ErrAccessDenied is returned when a user does not have access.
var ErrAccessDenied = errors.New("access denied")

// ErrNoPolicy is returned when no policy exists for a content item.
var ErrNoPolicy = errors.New("no access policy defined")

// AccessManager manages content access policies.
type AccessManager struct {
	mu       sync.RWMutex
	policies map[string]AccessPolicy // contentID → policy
}

// NewAccessManager creates a new AccessManager.
func NewAccessManager() *AccessManager {
	return &AccessManager{
		policies: make(map[string]AccessPolicy),
	}
}

// SetPolicy sets or replaces the access policy for a content item.
func (am *AccessManager) SetPolicy(policy AccessPolicy) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.policies[policy.ContentID] = policy
}

// CheckAccess verifies whether a user can access a piece of content.
// Returns nil if access is granted, or an error if denied.
func (am *AccessManager) CheckAccess(contentID, userID string, isPremium bool) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	policy, ok := am.policies[contentID]
	if !ok {
		return ErrNoPolicy
	}

	switch policy.Level {
	case AccessPublic:
		return nil

	case AccessPrivate:
		for _, u := range policy.AllowedUsers {
			if u == userID {
				return nil
			}
		}
		return ErrAccessDenied

	case AccessPaid:
		if isPremium {
			return nil
		}
		for _, u := range policy.AllowedUsers {
			if u == userID {
				return nil
			}
		}
		return ErrAccessDenied
	}

	return ErrAccessDenied
}
