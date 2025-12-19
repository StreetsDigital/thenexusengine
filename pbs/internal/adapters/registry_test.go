package adapters

import (
	"sync"
	"testing"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

// mockAdapter implements Adapter for testing
type mockAdapter struct {
	name string
}

func (m *mockAdapter) MakeRequests(request *openrtb.BidRequest, extraInfo *ExtraRequestInfo) ([]*RequestData, []error) {
	return nil, nil
}

func (m *mockAdapter) MakeBids(request *openrtb.BidRequest, responseData *ResponseData) (*BidderResponse, []error) {
	return nil, nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.adapters == nil {
		t.Error("expected adapters map to be initialized")
	}
	if len(r.adapters) != 0 {
		t.Error("expected empty adapters map")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	adapter := &mockAdapter{name: "test"}
	info := BidderInfo{
		Enabled:  true,
		Endpoint: "https://bidder.example.com",
	}

	err := r.Register("testbidder", adapter, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registration
	awi, ok := r.adapters["testbidder"]
	if !ok {
		t.Fatal("expected adapter to be registered")
	}
	if awi.Adapter != adapter {
		t.Error("expected adapter to match")
	}
	if awi.Info.Endpoint != info.Endpoint {
		t.Error("expected info to match")
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()
	adapter := &mockAdapter{name: "test"}
	info := BidderInfo{Enabled: true}

	// First registration should succeed
	err := r.Register("testbidder", adapter, info)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = r.Register("testbidder", adapter, info)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
	if err.Error() != "adapter already registered: testbidder" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	adapter := &mockAdapter{name: "test"}
	info := BidderInfo{
		Enabled:  true,
		Endpoint: "https://bidder.example.com",
	}
	r.Register("testbidder", adapter, info)

	// Test successful get
	awi, ok := r.Get("testbidder")
	if !ok {
		t.Fatal("expected to find adapter")
	}
	if awi.Adapter != adapter {
		t.Error("expected adapter to match")
	}
	if awi.Info.Endpoint != info.Endpoint {
		t.Error("expected info to match")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent adapter")
	}
}

func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry()

	// Register multiple adapters
	adapter1 := &mockAdapter{name: "adapter1"}
	adapter2 := &mockAdapter{name: "adapter2"}
	adapter3 := &mockAdapter{name: "adapter3"}

	r.Register("bidder1", adapter1, BidderInfo{Enabled: true})
	r.Register("bidder2", adapter2, BidderInfo{Enabled: false})
	r.Register("bidder3", adapter3, BidderInfo{Enabled: true})

	all := r.GetAll()

	if len(all) != 3 {
		t.Errorf("expected 3 adapters, got %d", len(all))
	}
	if _, ok := all["bidder1"]; !ok {
		t.Error("expected bidder1 in result")
	}
	if _, ok := all["bidder2"]; !ok {
		t.Error("expected bidder2 in result")
	}
	if _, ok := all["bidder3"]; !ok {
		t.Error("expected bidder3 in result")
	}
}

func TestRegistry_GetAll_Empty(t *testing.T) {
	r := NewRegistry()

	all := r.GetAll()

	if all == nil {
		t.Fatal("expected non-nil map")
	}
	if len(all) != 0 {
		t.Error("expected empty map")
	}
}

func TestRegistry_GetAll_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	adapter := &mockAdapter{name: "test"}
	r.Register("bidder1", adapter, BidderInfo{Enabled: true})

	all := r.GetAll()

	// Modify the returned map
	delete(all, "bidder1")

	// Original should be unchanged
	_, ok := r.Get("bidder1")
	if !ok {
		t.Error("expected original registry to be unchanged")
	}
}

func TestRegistry_ListBidders(t *testing.T) {
	r := NewRegistry()

	r.Register("alpha", &mockAdapter{}, BidderInfo{Enabled: true})
	r.Register("beta", &mockAdapter{}, BidderInfo{Enabled: false})
	r.Register("gamma", &mockAdapter{}, BidderInfo{Enabled: true})

	bidders := r.ListBidders()

	if len(bidders) != 3 {
		t.Errorf("expected 3 bidders, got %d", len(bidders))
	}

	// Check all bidders are present (order not guaranteed)
	bidderSet := make(map[string]bool)
	for _, b := range bidders {
		bidderSet[b] = true
	}

	if !bidderSet["alpha"] {
		t.Error("expected alpha in list")
	}
	if !bidderSet["beta"] {
		t.Error("expected beta in list")
	}
	if !bidderSet["gamma"] {
		t.Error("expected gamma in list")
	}
}

func TestRegistry_ListBidders_Empty(t *testing.T) {
	r := NewRegistry()

	bidders := r.ListBidders()

	if bidders == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(bidders) != 0 {
		t.Error("expected empty slice")
	}
}

func TestRegistry_ListEnabledBidders(t *testing.T) {
	r := NewRegistry()

	r.Register("enabled1", &mockAdapter{}, BidderInfo{Enabled: true})
	r.Register("disabled", &mockAdapter{}, BidderInfo{Enabled: false})
	r.Register("enabled2", &mockAdapter{}, BidderInfo{Enabled: true})

	enabled := r.ListEnabledBidders()

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled bidders, got %d", len(enabled))
	}

	// Check only enabled bidders are present
	bidderSet := make(map[string]bool)
	for _, b := range enabled {
		bidderSet[b] = true
	}

	if !bidderSet["enabled1"] {
		t.Error("expected enabled1 in list")
	}
	if !bidderSet["enabled2"] {
		t.Error("expected enabled2 in list")
	}
	if bidderSet["disabled"] {
		t.Error("expected disabled not to be in list")
	}
}

func TestRegistry_ListEnabledBidders_NoneEnabled(t *testing.T) {
	r := NewRegistry()

	r.Register("disabled1", &mockAdapter{}, BidderInfo{Enabled: false})
	r.Register("disabled2", &mockAdapter{}, BidderInfo{Enabled: false})

	enabled := r.ListEnabledBidders()

	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled bidders, got %d", len(enabled))
	}
}

func TestRegistry_ListEnabledBidders_Empty(t *testing.T) {
	r := NewRegistry()

	enabled := r.ListEnabledBidders()

	if enabled == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(enabled) != 0 {
		t.Error("expected empty slice")
	}
}

func TestRegisterAdapter_DefaultRegistry(t *testing.T) {
	// Create a unique bidder name to avoid conflicts with other tests
	bidderCode := "test_default_registry_bidder"
	adapter := &mockAdapter{name: "default"}
	info := BidderInfo{
		Enabled:  true,
		Endpoint: "https://default.example.com",
	}

	err := RegisterAdapter(bidderCode, adapter, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify in default registry
	awi, ok := DefaultRegistry.Get(bidderCode)
	if !ok {
		t.Fatal("expected adapter to be in default registry")
	}
	if awi.Info.Endpoint != info.Endpoint {
		t.Error("expected info to match")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bidderCode := string(rune('a' + idx%26))
			adapter := &mockAdapter{name: bidderCode}
			// Ignore error since we might have duplicates
			_ = r.Register(bidderCode, adapter, BidderInfo{Enabled: idx%2 == 0})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bidderCode := string(rune('a' + idx%26))
			r.Get(bidderCode)
		}(i)
	}

	// Concurrent GetAll
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.GetAll()
		}()
	}

	// Concurrent ListBidders
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.ListBidders()
		}()
	}

	// Concurrent ListEnabledBidders
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.ListEnabledBidders()
		}()
	}

	wg.Wait()

	// Verify registry is in consistent state
	all := r.GetAll()
	bidders := r.ListBidders()
	if len(all) != len(bidders) {
		t.Errorf("GetAll and ListBidders counts don't match: %d vs %d", len(all), len(bidders))
	}
}

// Benchmark tests

func BenchmarkRegistry_Get(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		bidderCode := string(rune('a' + i%26))
		r.Register(bidderCode, &mockAdapter{}, BidderInfo{Enabled: true})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Get("m")
	}
}

func BenchmarkRegistry_GetAll(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		bidderCode := string(rune('a' + i%26))
		r.Register(bidderCode, &mockAdapter{}, BidderInfo{Enabled: true})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.GetAll()
	}
}

func BenchmarkRegistry_ListBidders(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		bidderCode := string(rune('a' + i%26))
		r.Register(bidderCode, &mockAdapter{}, BidderInfo{Enabled: true})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ListBidders()
	}
}

func BenchmarkRegistry_ListEnabledBidders(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		bidderCode := string(rune('a' + i%26))
		r.Register(bidderCode, &mockAdapter{}, BidderInfo{Enabled: i%2 == 0})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ListEnabledBidders()
	}
}
