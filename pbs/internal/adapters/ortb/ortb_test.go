package ortb

import (
	"encoding/json"
	"testing"

	"github.com/StreetsDigital/thenexusengine/pbs/internal/openrtb"
)

func TestAugmentSChain_NilSource(t *testing.T) {
	adapter := &GenericAdapter{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "nexusengine.com", SID: "nexus-001", HP: 1},
		},
		Version: "1.0",
	}

	result := adapter.augmentSChain(nil, augment)

	if result == nil {
		t.Fatal("expected non-nil result when source is nil")
	}
	if result.SChain == nil {
		t.Fatal("expected non-nil schain")
	}
	if len(result.SChain.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(result.SChain.Nodes))
	}
	if result.SChain.Nodes[0].ASI != "nexusengine.com" {
		t.Errorf("expected ASI 'nexusengine.com', got '%s'", result.SChain.Nodes[0].ASI)
	}
	if result.SChain.Ver != "1.0" {
		t.Errorf("expected version '1.0', got '%s'", result.SChain.Ver)
	}
	if result.SChain.Complete != 1 {
		t.Errorf("expected complete=1 (default), got %d", result.SChain.Complete)
	}
}

func TestAugmentSChain_EmptySource(t *testing.T) {
	adapter := &GenericAdapter{}
	source := &openrtb.Source{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "test.com", SID: "test-001", HP: 1, Name: "Test Entity"},
		},
		Version: "1.0",
	}

	result := adapter.augmentSChain(source, augment)

	if result.SChain == nil {
		t.Fatal("expected schain to be created")
	}
	if len(result.SChain.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(result.SChain.Nodes))
	}
	if result.SChain.Nodes[0].Name != "Test Entity" {
		t.Errorf("expected Name 'Test Entity', got '%s'", result.SChain.Nodes[0].Name)
	}
}

func TestAugmentSChain_AppendToExisting(t *testing.T) {
	adapter := &GenericAdapter{}
	source := &openrtb.Source{
		SChain: &openrtb.SupplyChain{
			Ver:      "1.0",
			Complete: 1,
			Nodes: []openrtb.SupplyChainNode{
				{ASI: "publisher.com", SID: "pub-001", HP: 1},
			},
		},
	}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "nexusengine.com", SID: "nexus-001", HP: 1},
		},
	}

	result := adapter.augmentSChain(source, augment)

	if len(result.SChain.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(result.SChain.Nodes))
	}
	// First node should be the original
	if result.SChain.Nodes[0].ASI != "publisher.com" {
		t.Errorf("first node should be original, got ASI '%s'", result.SChain.Nodes[0].ASI)
	}
	// Second node should be the appended one
	if result.SChain.Nodes[1].ASI != "nexusengine.com" {
		t.Errorf("second node should be appended, got ASI '%s'", result.SChain.Nodes[1].ASI)
	}
}

func TestAugmentSChain_MultipleNodes(t *testing.T) {
	adapter := &GenericAdapter{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "exchange.com", SID: "ex-001", HP: 1},
			{ASI: "reseller.com", SID: "resell-001", HP: 0},
			{ASI: "ssp.com", SID: "ssp-001", HP: 1},
		},
		Version: "1.0",
	}

	result := adapter.augmentSChain(nil, augment)

	if len(result.SChain.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(result.SChain.Nodes))
	}
	// Verify order is preserved
	if result.SChain.Nodes[0].ASI != "exchange.com" {
		t.Errorf("expected first node ASI 'exchange.com', got '%s'", result.SChain.Nodes[0].ASI)
	}
	if result.SChain.Nodes[1].HP != 0 {
		t.Errorf("expected second node HP=0 (indirect), got %d", result.SChain.Nodes[1].HP)
	}
	if result.SChain.Nodes[2].ASI != "ssp.com" {
		t.Errorf("expected third node ASI 'ssp.com', got '%s'", result.SChain.Nodes[2].ASI)
	}
}

func TestAugmentSChain_OverrideComplete(t *testing.T) {
	adapter := &GenericAdapter{}

	// Test override to 0 (incomplete)
	complete0 := 0
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "test.com", SID: "test-001", HP: 1},
		},
		Complete: &complete0,
	}

	result := adapter.augmentSChain(nil, augment)

	if result.SChain.Complete != 0 {
		t.Errorf("expected complete=0, got %d", result.SChain.Complete)
	}

	// Test override to 1 (complete)
	complete1 := 1
	augment.Complete = &complete1

	result = adapter.augmentSChain(nil, augment)

	if result.SChain.Complete != 1 {
		t.Errorf("expected complete=1, got %d", result.SChain.Complete)
	}
}

func TestAugmentSChain_PreserveOriginalComplete(t *testing.T) {
	adapter := &GenericAdapter{}
	source := &openrtb.Source{
		SChain: &openrtb.SupplyChain{
			Ver:      "1.0",
			Complete: 0, // Original is incomplete
			Nodes: []openrtb.SupplyChainNode{
				{ASI: "original.com", SID: "orig-001", HP: 1},
			},
		},
	}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "appended.com", SID: "app-001", HP: 1},
		},
		Complete: nil, // Don't override
	}

	result := adapter.augmentSChain(source, augment)

	if result.SChain.Complete != 0 {
		t.Errorf("expected complete=0 (preserved), got %d", result.SChain.Complete)
	}
}

func TestAugmentSChain_VersionOverride(t *testing.T) {
	adapter := &GenericAdapter{}
	source := &openrtb.Source{
		SChain: &openrtb.SupplyChain{
			Ver:   "1.0",
			Nodes: []openrtb.SupplyChainNode{},
		},
	}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "test.com", SID: "test-001", HP: 1},
		},
		Version: "2.0",
	}

	result := adapter.augmentSChain(source, augment)

	if result.SChain.Ver != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", result.SChain.Ver)
	}
}

func TestAugmentSChain_DefaultVersion(t *testing.T) {
	adapter := &GenericAdapter{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "test.com", SID: "test-001", HP: 1},
		},
		Version: "", // Empty version should default to "1.0"
	}

	result := adapter.augmentSChain(nil, augment)

	if result.SChain.Ver != "1.0" {
		t.Errorf("expected default version '1.0', got '%s'", result.SChain.Ver)
	}
}

func TestAugmentSChain_NodeWithAllFields(t *testing.T) {
	adapter := &GenericAdapter{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{
				ASI:    "nexusengine.com",
				SID:    "nexus-seat-001",
				HP:     1,
				RID:    "request-12345",
				Name:   "The Nexus Engine",
				Domain: "nexusengine.com",
			},
		},
	}

	result := adapter.augmentSChain(nil, augment)

	node := result.SChain.Nodes[0]
	if node.ASI != "nexusengine.com" {
		t.Errorf("expected ASI 'nexusengine.com', got '%s'", node.ASI)
	}
	if node.SID != "nexus-seat-001" {
		t.Errorf("expected SID 'nexus-seat-001', got '%s'", node.SID)
	}
	if node.HP != 1 {
		t.Errorf("expected HP 1, got %d", node.HP)
	}
	if node.RID != "request-12345" {
		t.Errorf("expected RID 'request-12345', got '%s'", node.RID)
	}
	if node.Name != "The Nexus Engine" {
		t.Errorf("expected Name 'The Nexus Engine', got '%s'", node.Name)
	}
	if node.Domain != "nexusengine.com" {
		t.Errorf("expected Domain 'nexusengine.com', got '%s'", node.Domain)
	}
}

func TestAugmentSChain_NodeWithExt(t *testing.T) {
	adapter := &GenericAdapter{}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{
				ASI: "test.com",
				SID: "test-001",
				HP:  1,
				Ext: map[string]interface{}{
					"custom_field": "value",
					"partner_id":   12345,
				},
			},
		},
	}

	result := adapter.augmentSChain(nil, augment)

	node := result.SChain.Nodes[0]
	if node.Ext == nil {
		t.Fatal("expected Ext to be set")
	}

	var extData map[string]interface{}
	if err := json.Unmarshal(node.Ext, &extData); err != nil {
		t.Fatalf("failed to unmarshal ext: %v", err)
	}

	if extData["custom_field"] != "value" {
		t.Errorf("expected custom_field='value', got '%v'", extData["custom_field"])
	}
	// JSON unmarshals numbers as float64
	if extData["partner_id"] != float64(12345) {
		t.Errorf("expected partner_id=12345, got '%v'", extData["partner_id"])
	}
}

func TestAugmentSChain_DoesNotModifyOriginal(t *testing.T) {
	adapter := &GenericAdapter{}
	originalNodes := []openrtb.SupplyChainNode{
		{ASI: "original.com", SID: "orig-001", HP: 1},
	}
	source := &openrtb.Source{
		SChain: &openrtb.SupplyChain{
			Ver:      "1.0",
			Complete: 1,
			Nodes:    originalNodes,
		},
	}
	augment := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "appended.com", SID: "app-001", HP: 1},
		},
	}

	result := adapter.augmentSChain(source, augment)

	// Verify original is not modified
	if len(source.SChain.Nodes) != 1 {
		t.Errorf("original schain should still have 1 node, got %d", len(source.SChain.Nodes))
	}
	if source.SChain.Nodes[0].ASI != "original.com" {
		t.Errorf("original node should be unchanged, got ASI '%s'", source.SChain.Nodes[0].ASI)
	}

	// Verify result has both nodes
	if len(result.SChain.Nodes) != 2 {
		t.Errorf("result should have 2 nodes, got %d", len(result.SChain.Nodes))
	}
}

func TestAugmentSChain_HP_DirectAndIndirect(t *testing.T) {
	adapter := &GenericAdapter{}

	// Test HP=1 (direct/payment)
	augmentDirect := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "direct.com", SID: "dir-001", HP: 1},
		},
	}

	result := adapter.augmentSChain(nil, augmentDirect)
	if result.SChain.Nodes[0].HP != 1 {
		t.Errorf("expected HP=1 for direct, got %d", result.SChain.Nodes[0].HP)
	}

	// Test HP=0 (indirect)
	augmentIndirect := &SChainAugmentConfig{
		Enabled: true,
		Nodes: []SChainNodeConfig{
			{ASI: "indirect.com", SID: "ind-001", HP: 0},
		},
	}

	result = adapter.augmentSChain(nil, augmentIndirect)
	if result.SChain.Nodes[0].HP != 0 {
		t.Errorf("expected HP=0 for indirect, got %d", result.SChain.Nodes[0].HP)
	}
}

// Test the transform request function integrates schain correctly
func TestTransformRequest_WithSChainAugment(t *testing.T) {
	config := &BidderConfig{
		BidderCode: "test-bidder",
		Name:       "Test Bidder",
		Endpoint: EndpointConfig{
			URL: "https://test.com/bid",
		},
		RequestTransform: RequestTransformConfig{
			SChainAugment: SChainAugmentConfig{
				Enabled: true,
				Nodes: []SChainNodeConfig{
					{ASI: "nexusengine.com", SID: "nexus-001", HP: 1},
				},
				Version: "1.0",
			},
		},
	}

	adapter := New(config)
	request := &openrtb.BidRequest{
		ID: "test-request",
	}

	result := adapter.transformRequest(request, config)

	if result.Source == nil {
		t.Fatal("expected Source to be set")
	}
	if result.Source.SChain == nil {
		t.Fatal("expected SChain to be set")
	}
	if len(result.Source.SChain.Nodes) != 1 {
		t.Errorf("expected 1 schain node, got %d", len(result.Source.SChain.Nodes))
	}
	if result.Source.SChain.Nodes[0].ASI != "nexusengine.com" {
		t.Errorf("expected ASI 'nexusengine.com', got '%s'", result.Source.SChain.Nodes[0].ASI)
	}
}

func TestTransformRequest_SChainDisabled(t *testing.T) {
	config := &BidderConfig{
		BidderCode: "test-bidder",
		Name:       "Test Bidder",
		Endpoint: EndpointConfig{
			URL: "https://test.com/bid",
		},
		RequestTransform: RequestTransformConfig{
			SChainAugment: SChainAugmentConfig{
				Enabled: false, // Disabled
				Nodes: []SChainNodeConfig{
					{ASI: "nexusengine.com", SID: "nexus-001", HP: 1},
				},
			},
		},
	}

	adapter := New(config)
	request := &openrtb.BidRequest{
		ID: "test-request",
	}

	result := adapter.transformRequest(request, config)

	// SChain should not be added when disabled
	if result.Source != nil && result.Source.SChain != nil {
		t.Error("expected no SChain when augmentation is disabled")
	}
}

func TestTransformRequest_SChainEmptyNodes(t *testing.T) {
	config := &BidderConfig{
		BidderCode: "test-bidder",
		Name:       "Test Bidder",
		Endpoint: EndpointConfig{
			URL: "https://test.com/bid",
		},
		RequestTransform: RequestTransformConfig{
			SChainAugment: SChainAugmentConfig{
				Enabled: true,
				Nodes:   []SChainNodeConfig{}, // Empty nodes
			},
		},
	}

	adapter := New(config)
	request := &openrtb.BidRequest{
		ID: "test-request",
	}

	result := adapter.transformRequest(request, config)

	// SChain should not be added when nodes are empty
	if result.Source != nil && result.Source.SChain != nil {
		t.Error("expected no SChain when nodes are empty")
	}
}
