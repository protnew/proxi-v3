package nostr

// matchFilter checks if an event matches a NIP-01 filter.
func matchFilter(event *Event, filter *Filter) bool {	if len(filter.IDs) > 0 && !contains(filter.IDs, event.ID) {
		return false
	}
	if len(filter.Authors) > 0 && !contains(filter.Authors, event.PubKey) {
		return false
	}
	if len(filter.Kinds) > 0 && !containsInt(filter.Kinds, event.Kind) {
		return false
	}
	if filter.Since != nil && event.CreatedAt < *filter.Since {
		return false
	}
	if filter.Until != nil && event.CreatedAt > *filter.Until {
		return false
	}
	return true
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
