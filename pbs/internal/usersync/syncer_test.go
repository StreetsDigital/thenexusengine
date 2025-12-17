package usersync

import (
	"strings"
	"testing"
)

func TestNewSyncer(t *testing.T) {
	tests := []struct {
		name      string
		config    SyncerConfig
		expectErr bool
	}{
		{
			name: "valid redirect syncer",
			config: SyncerConfig{
				Key:         "testbidder",
				RedirectURL: "https://sync.example.com/sync?r={{.RedirectURL}}",
				ExternalURL: "https://pbs.example.com",
			},
			expectErr: false,
		},
		{
			name: "valid iframe syncer",
			config: SyncerConfig{
				Key:         "testbidder",
				IFrameURL:   "https://sync.example.com/iframe?r={{.RedirectURL}}",
				ExternalURL: "https://pbs.example.com",
			},
			expectErr: false,
		},
		{
			name: "missing key",
			config: SyncerConfig{
				RedirectURL: "https://sync.example.com/sync",
				ExternalURL: "https://pbs.example.com",
			},
			expectErr: true,
		},
		{
			name: "missing URLs",
			config: SyncerConfig{
				Key:         "testbidder",
				ExternalURL: "https://pbs.example.com",
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			syncer, err := NewSyncer(tc.config)
			if tc.expectErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if syncer == nil {
					t.Error("syncer should not be nil")
				}
			}
		})
	}
}

func TestSyncerGetSync(t *testing.T) {
	config := SyncerConfig{
		Key:         "testbidder",
		RedirectURL: "https://sync.example.com/sync?gdpr={{.GDPR}}&consent={{.GDPRConsent}}&r={{.RedirectURL}}",
		ExternalURL: "https://pbs.example.com",
	}

	syncer, err := NewSyncer(config)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}

	privacy := Privacy{
		GDPR:        true,
		GDPRConsent: "BOtest",
	}

	sync, err := syncer.GetSync([]SyncType{SyncTypePixel}, privacy)
	if err != nil {
		t.Fatalf("GetSync failed: %v", err)
	}

	if sync.Type != SyncTypePixel {
		t.Errorf("expected sync type %s, got %s", SyncTypePixel, sync.Type)
	}

	if !strings.Contains(sync.URL, "gdpr=1") {
		t.Error("sync URL should contain gdpr=1")
	}

	if !strings.Contains(sync.URL, "consent=BOtest") {
		t.Error("sync URL should contain consent parameter")
	}
}

func TestSyncerSupportsType(t *testing.T) {
	redirectOnlyConfig := SyncerConfig{
		Key:         "redirectonly",
		RedirectURL: "https://sync.example.com/sync",
		ExternalURL: "https://pbs.example.com",
	}

	syncer, _ := NewSyncer(redirectOnlyConfig)

	if !syncer.SupportsType(SyncTypePixel) {
		t.Error("should support redirect/pixel type")
	}

	if syncer.SupportsType(SyncTypeIFrame) {
		t.Error("should not support iframe type")
	}
}

func TestSyncerRegistry(t *testing.T) {
	registry := NewSyncerRegistry()

	syncer, _ := NewSyncer(SyncerConfig{
		Key:         "testbidder",
		RedirectURL: "https://sync.example.com/sync",
		ExternalURL: "https://pbs.example.com",
	})

	// Register
	err := registry.Register(syncer)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Get
	retrieved, ok := registry.Get("testbidder")
	if !ok {
		t.Error("Get should return true for registered syncer")
	}
	if retrieved.Key() != "testbidder" {
		t.Error("retrieved syncer has wrong key")
	}

	// Get non-existent
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent syncer")
	}

	// Keys
	keys := registry.Keys()
	if len(keys) != 1 || keys[0] != "testbidder" {
		t.Error("Keys should return registered syncer keys")
	}

	// Duplicate registration
	err = registry.Register(syncer)
	if err == nil {
		t.Error("duplicate registration should return error")
	}
}

func TestDynamicSyncerRegistry(t *testing.T) {
	registry, err := NewDynamicSyncerRegistry("https://pbs.example.com")
	if err != nil {
		t.Fatalf("NewDynamicSyncerRegistry failed: %v", err)
	}

	// Should have static syncers
	if len(registry.Keys()) == 0 {
		t.Error("registry should have static syncers")
	}

	// Register dynamic syncer
	err = registry.RegisterDynamicSyncer("custombidder", DynamicSyncerConfig{
		Enabled:     true,
		RedirectURL: "https://custom.example.com/sync?r={{.RedirectURL}}",
	})
	if err != nil {
		t.Fatalf("RegisterDynamicSyncer failed: %v", err)
	}

	// Verify it was registered
	syncer, ok := registry.Get("custombidder")
	if !ok {
		t.Error("custom syncer should be registered")
	}
	if syncer.Key() != "custombidder" {
		t.Error("custom syncer has wrong key")
	}

	// Update syncer
	err = registry.UpdateDynamicSyncer("custombidder", DynamicSyncerConfig{
		Enabled:     true,
		RedirectURL: "https://custom.example.com/newsync?r={{.RedirectURL}}",
	})
	if err != nil {
		t.Fatalf("UpdateDynamicSyncer failed: %v", err)
	}

	// Unregister
	registry.UnregisterDynamicSyncer("custombidder")
	_, ok = registry.Get("custombidder")
	if ok {
		t.Error("custom syncer should be unregistered")
	}
}

func TestInitDefaultSyncers(t *testing.T) {
	registry, err := InitDefaultSyncers("https://pbs.example.com")
	if err != nil {
		t.Fatalf("InitDefaultSyncers failed: %v", err)
	}

	// Should have appnexus syncer
	syncer, ok := registry.Get("appnexus")
	if !ok {
		t.Error("appnexus syncer should be registered")
	}
	if syncer.Key() != "appnexus" {
		t.Error("appnexus syncer has wrong key")
	}

	// Should have multiple syncers
	if len(registry.Keys()) < 10 {
		t.Errorf("expected at least 10 syncers, got %d", len(registry.Keys()))
	}
}
