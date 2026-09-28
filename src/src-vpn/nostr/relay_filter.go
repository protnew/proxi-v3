package nostr

import (
	"encoding/json"
	"strings"
)

// matchFilter checks if an event matches a NIP-01 filter, including #p tags.
func matchFilter(event *Event, filter *Filter) bool {
	if len(filter.IDs) > 0 && !contains(filter.IDs, event.ID) {
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
	if !matchTagFilters(event, filter.TagFilters) {
		return false
	}
	return true
}

// UnmarshalJSON keeps NIP-01 fields and captures #e/#p tag filters.
func (f *Filter) UnmarshalJSON(data []byte) error {
	type wire struct {
		IDs     []string `json:"ids"`
		Authors []string `json:"authors"`
		Kinds   []int    `json:"kinds"`
		Since   *int64   `json:"since"`
		Until   *int64   `json:"until"`
		Limit   int      `json:"limit"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	f.IDs = w.IDs
	f.Authors = w.Authors
	f.Kinds = w.Kinds
	f.Since = w.Since
	f.Until = w.Until
	f.Limit = w.Limit
	f.TagFilters = nil
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		if len(k) < 2 || k[0] != '#' {
			continue
		}
		var vals []string
		if err := json.Unmarshal(v, &vals); err != nil {
			continue
		}
		if f.TagFilters == nil {
			f.TagFilters = map[string][]string{}
		}
		f.TagFilters[k[1:]] = vals
	}
	return nil
}

func matchTagFilters(event *Event, tags map[string][]string) bool {
	if len(tags) == 0 {
		return true
	}
	for name, want := range tags {
		if len(want) == 0 {
			continue
		}
		if !eventHasTag(event, name, want) {
			return false
		}
	}
	return true
}

func eventHasTag(event *Event, name string, want []string) bool {
	for _, tag := range event.Tags {
		if len(tag) < 2 || tag[0] != name {
			continue
		}
		for _, w := range want {
			if strings.EqualFold(tag[1], w) {
				return true
			}
		}
	}
	return false
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
