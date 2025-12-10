package fpd

import (
	"testing"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

func TestNewEIDFilter(t *testing.T) {
	filter := NewEIDFilter(nil)
	if filter == nil {
		t.Fatal("expected non-nil filter")
	}
}

func TestEIDFilterDisabled(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: false,
	})

	eids := []openrtb.EID{
		{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "123"}}},
	}

	result := filter.FilterEIDs(eids)
	if result != nil {
		t.Errorf("expected nil when EIDs disabled, got %v", result)
	}
}

func TestEIDFilterAllowAll(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{}, // Empty = allow all
	})

	eids := []openrtb.EID{
		{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "123"}}},
		{Source: "custom.com", UIDs: []openrtb.UID{{ID: "456"}}},
	}

	result := filter.FilterEIDs(eids)
	if len(result) != 2 {
		t.Errorf("expected 2 EIDs, got %d", len(result))
	}
}

func TestEIDFilterAllowedSources(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"liveramp.com", "uidapi.com"},
	})

	eids := []openrtb.EID{
		{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "123"}}},
		{Source: "uidapi.com", UIDs: []openrtb.UID{{ID: "456"}}},
		{Source: "blocked.com", UIDs: []openrtb.UID{{ID: "789"}}},
	}

	result := filter.FilterEIDs(eids)
	if len(result) != 2 {
		t.Errorf("expected 2 EIDs, got %d", len(result))
	}

	// Check that correct ones are included
	sources := make(map[string]bool)
	for _, eid := range result {
		sources[eid.Source] = true
	}

	if !sources["liveramp.com"] {
		t.Error("expected liveramp.com to be included")
	}
	if !sources["uidapi.com"] {
		t.Error("expected uidapi.com to be included")
	}
	if sources["blocked.com"] {
		t.Error("expected blocked.com to be filtered")
	}
}

func TestEIDFilterCaseInsensitive(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"LiveRamp.com"},
	})

	eids := []openrtb.EID{
		{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "123"}}},
		{Source: "LIVERAMP.COM", UIDs: []openrtb.UID{{ID: "456"}}},
	}

	result := filter.FilterEIDs(eids)
	if len(result) != 2 {
		t.Errorf("expected case-insensitive match, got %d results", len(result))
	}
}

func TestEIDFilterEmptyInput(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"liveramp.com"},
	})

	result := filter.FilterEIDs(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = filter.FilterEIDs([]openrtb.EID{})
	if len(result) != 0 {
		t.Errorf("expected empty for empty input, got %d", len(result))
	}
}

func TestFilterUserEIDs(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"liveramp.com"},
	})

	user := &openrtb.User{
		ID: "user1",
		EIDs: []openrtb.EID{
			{Source: "liveramp.com", UIDs: []openrtb.UID{{ID: "123"}}},
			{Source: "blocked.com", UIDs: []openrtb.UID{{ID: "456"}}},
		},
	}

	filter.FilterUserEIDs(user)

	if len(user.EIDs) != 1 {
		t.Errorf("expected 1 EID after filtering, got %d", len(user.EIDs))
	}
	if user.EIDs[0].Source != "liveramp.com" {
		t.Errorf("expected liveramp.com, got %s", user.EIDs[0].Source)
	}
}

func TestProcessRequestEIDs(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"uidapi.com"},
	})

	req := &openrtb.BidRequest{
		ID: "req1",
		User: &openrtb.User{
			ID: "user1",
			EIDs: []openrtb.EID{
				{Source: "uidapi.com", UIDs: []openrtb.UID{{ID: "123"}}},
				{Source: "other.com", UIDs: []openrtb.UID{{ID: "456"}}},
			},
		},
	}

	filter.ProcessRequestEIDs(req)

	if len(req.User.EIDs) != 1 {
		t.Errorf("expected 1 EID, got %d", len(req.User.EIDs))
	}
}

func TestProcessRequestEIDsNilUser(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"liveramp.com"},
	})

	req := &openrtb.BidRequest{
		ID:   "req1",
		User: nil,
	}

	// Should not panic
	result := filter.ProcessRequestEIDs(req)
	if result != req {
		t.Error("expected same request returned")
	}
}

func TestGetAllowedSources(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"a.com", "b.com"},
	})

	sources := filter.GetAllowedSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}

	// Allow all case
	filter = NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{},
	})

	sources = filter.GetAllowedSources()
	if sources != nil {
		t.Error("expected nil for allow all")
	}
}

func TestIsEnabled(t *testing.T) {
	filter := NewEIDFilter(&Config{EIDsEnabled: true})
	if !filter.IsEnabled() {
		t.Error("expected enabled")
	}

	filter = NewEIDFilter(&Config{EIDsEnabled: false})
	if filter.IsEnabled() {
		t.Error("expected disabled")
	}
}

func TestAllowsAllSources(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{},
	})
	if !filter.AllowsAllSources() {
		t.Error("expected to allow all sources")
	}

	filter = NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"a.com"},
	})
	if filter.AllowsAllSources() {
		t.Error("expected to not allow all sources")
	}
}

func TestCollectEIDStats(t *testing.T) {
	filter := NewEIDFilter(&Config{
		EIDsEnabled: true,
		EIDSources:  []string{"allowed.com"},
	})

	eids := []openrtb.EID{
		{Source: "allowed.com", UIDs: []openrtb.UID{{ID: "1"}}},
		{Source: "allowed.com", UIDs: []openrtb.UID{{ID: "2"}}},
		{Source: "blocked.com", UIDs: []openrtb.UID{{ID: "3"}}},
	}

	stats := filter.CollectEIDStats(eids)

	if stats.TotalEIDs != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalEIDs)
	}
	if stats.AllowedEIDs != 2 {
		t.Errorf("expected 2 allowed, got %d", stats.AllowedEIDs)
	}
	if stats.FilteredEIDs != 1 {
		t.Errorf("expected 1 filtered, got %d", stats.FilteredEIDs)
	}
	if stats.BySource["allowed.com"] != 2 {
		t.Errorf("expected 2 from allowed.com, got %d", stats.BySource["allowed.com"])
	}
}

func TestGetEIDSourceDescription(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{"liveramp.com", "LiveRamp IdentityLink"},
		{"uidapi.com", "Unified ID 2.0"},
		{"LIVERAMP.COM", "LiveRamp IdentityLink"},
		{"unknown.com", "unknown.com"},
	}

	for _, tt := range tests {
		result := GetEIDSourceDescription(tt.source)
		if result != tt.expected {
			t.Errorf("source %s: expected %s, got %s", tt.source, tt.expected, result)
		}
	}
}

func TestParseEIDSources(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"a.com", []string{"a.com"}},
		{"a.com,b.com", []string{"a.com", "b.com"}},
		{" a.com , b.com ", []string{"a.com", "b.com"}},
		{"a.com,,b.com", []string{"a.com", "b.com"}},
	}

	for _, tt := range tests {
		result := ParseEIDSources(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("input %q: expected %d sources, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("input %q: expected %v, got %v", tt.input, tt.expected, result)
				break
			}
		}
	}
}
