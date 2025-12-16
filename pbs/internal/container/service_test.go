package container

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockHook is a test hook implementation
type mockHook struct {
	hookType  HookType
	name      string
	enabled   bool
	priority  int
	execFn    func(ctx context.Context, input *HookInput) (*HookOutput, error)
	execCount int
}

func (h *mockHook) Type() HookType     { return h.hookType }
func (h *mockHook) Name() string       { return h.name }
func (h *mockHook) Priority() int      { return h.priority }
func (h *mockHook) IsEnabled() bool    { return h.enabled }
func (h *mockHook) Execute(ctx context.Context, input *HookInput) (*HookOutput, error) {
	h.execCount++
	if h.execFn != nil {
		return h.execFn(ctx, input)
	}
	return &HookOutput{Success: true, Data: input.Data}, nil
}

func TestNewService(t *testing.T) {
	// Test with nil config
	s := NewService(nil)
	if s == nil {
		t.Fatal("Expected service, got nil")
	}
	if s.enabled {
		t.Error("Expected service to be disabled by default")
	}

	// Test with custom config
	config := &ServiceConfig{
		Enabled:        true,
		DefaultTimeout: 500 * time.Millisecond,
	}
	s = NewService(config)
	if !s.enabled {
		t.Error("Expected service to be enabled")
	}
}

func TestServiceRegister(t *testing.T) {
	s := NewService(DefaultServiceConfig())

	// Test registering a hook
	hook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "test-hook",
		enabled:  true,
	}

	err := s.Register(hook)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test registering nil hook
	err = s.Register(nil)
	if err == nil {
		t.Error("Expected error for nil hook")
	}

	// Test registering hook with empty name
	emptyNameHook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "",
		enabled:  true,
	}
	err = s.Register(emptyNameHook)
	if err == nil {
		t.Error("Expected error for empty name hook")
	}

	// Test registering duplicate hook
	err = s.Register(hook)
	if err == nil {
		t.Error("Expected error for duplicate hook")
	}
}

func TestServiceUnregister(t *testing.T) {
	s := NewService(DefaultServiceConfig())

	hook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "test-hook",
		enabled:  true,
	}
	s.Register(hook)

	// Test unregistering existing hook
	err := s.Unregister("test-hook")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test unregistering non-existent hook
	err = s.Unregister("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent hook")
	}
}

func TestServiceGetHooks(t *testing.T) {
	s := NewService(DefaultServiceConfig())

	// Register multiple hooks of different types
	hook1 := &mockHook{hookType: HookTypePreRequest, name: "pre-1", enabled: true, priority: 2}
	hook2 := &mockHook{hookType: HookTypePreRequest, name: "pre-2", enabled: true, priority: 1}
	hook3 := &mockHook{hookType: HookTypePostResponse, name: "post-1", enabled: true}
	hook4 := &mockHook{hookType: HookTypePreRequest, name: "pre-disabled", enabled: false}

	s.Register(hook1)
	s.Register(hook2)
	s.Register(hook3)
	s.Register(hook4)

	// Get pre-request hooks
	preHooks := s.GetHooks(HookTypePreRequest)
	if len(preHooks) != 2 {
		t.Errorf("Expected 2 pre-request hooks, got %d", len(preHooks))
	}

	// Check priority ordering (lower priority first)
	if preHooks[0].Name() != "pre-2" {
		t.Errorf("Expected 'pre-2' first, got '%s'", preHooks[0].Name())
	}

	// Get post-response hooks
	postHooks := s.GetHooks(HookTypePostResponse)
	if len(postHooks) != 1 {
		t.Errorf("Expected 1 post-response hook, got %d", len(postHooks))
	}

	// Get hooks of type with none registered
	bidHooks := s.GetHooks(HookTypeBidTransform)
	if len(bidHooks) != 0 {
		t.Errorf("Expected 0 bid transform hooks, got %d", len(bidHooks))
	}
}

func TestServiceExecuteHooks(t *testing.T) {
	config := &ServiceConfig{
		Enabled:        true,
		DefaultTimeout: 1 * time.Second,
		FailOpen:       true,
	}
	s := NewService(config)

	// Test with no hooks registered
	input := &HookInput{
		Data: map[string]string{"test": "data"},
	}

	output, err := s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success")
	}

	// Register a hook that modifies data
	hook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "modifier",
		enabled:  true,
		execFn: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
			return &HookOutput{
				Success: true,
				Data:    map[string]string{"modified": "true"},
			}, nil
		},
	}
	s.Register(hook)

	output, err = s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if data, ok := output.Data.(map[string]string); ok {
		if data["modified"] != "true" {
			t.Error("Expected data to be modified")
		}
	} else {
		t.Error("Expected modified data")
	}
}

func TestServiceExecuteHooksDisabled(t *testing.T) {
	config := DefaultServiceConfig() // disabled by default
	s := NewService(config)

	hook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "test-hook",
		enabled:  true,
	}
	s.Register(hook)

	input := &HookInput{
		Data: map[string]string{"test": "data"},
	}

	output, err := s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success even when disabled")
	}
	if hook.execCount != 0 {
		t.Error("Hook should not be executed when service is disabled")
	}
}

func TestServiceExecuteHooksFailOpen(t *testing.T) {
	config := &ServiceConfig{
		Enabled:        true,
		DefaultTimeout: 1 * time.Second,
		FailOpen:       true,
	}
	s := NewService(config)

	// Register a failing hook
	failingHook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "failing",
		enabled:  true,
		priority: 1,
		execFn: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
			return nil, errors.New("hook failed")
		},
	}

	// Register a successful hook after the failing one
	successHook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "success",
		enabled:  true,
		priority: 2,
	}

	s.Register(failingHook)
	s.Register(successHook)

	input := &HookInput{Data: "test"}

	// With FailOpen, should continue despite failure
	output, err := s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err != nil {
		t.Errorf("Expected no error with FailOpen, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success with FailOpen")
	}
	if successHook.execCount != 1 {
		t.Error("Success hook should have executed")
	}
}

func TestServiceExecuteHooksFailClosed(t *testing.T) {
	config := &ServiceConfig{
		Enabled:        true,
		DefaultTimeout: 1 * time.Second,
		FailOpen:       false, // Fail closed
	}
	s := NewService(config)

	failingHook := &mockHook{
		hookType: HookTypePreRequest,
		name:     "failing",
		enabled:  true,
		execFn: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
			return nil, errors.New("hook failed")
		},
	}
	s.Register(failingHook)

	input := &HookInput{Data: "test"}

	// With FailClosed, should return error
	output, err := s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err == nil {
		t.Error("Expected error with FailClosed")
	}
	if output.Success {
		t.Error("Expected failure with FailClosed")
	}
}

func TestServiceEnableDisable(t *testing.T) {
	s := NewService(DefaultServiceConfig())

	if s.IsEnabled() {
		t.Error("Expected disabled by default")
	}

	s.Enable()
	if !s.IsEnabled() {
		t.Error("Expected enabled after Enable()")
	}

	s.Disable()
	if s.IsEnabled() {
		t.Error("Expected disabled after Disable()")
	}
}

func TestServiceStats(t *testing.T) {
	config := &ServiceConfig{Enabled: true}
	s := NewService(config)

	hook1 := &mockHook{hookType: HookTypePreRequest, name: "pre-1", enabled: true}
	hook2 := &mockHook{hookType: HookTypePreRequest, name: "pre-2", enabled: true}
	hook3 := &mockHook{hookType: HookTypePostResponse, name: "post-1", enabled: true}

	s.Register(hook1)
	s.Register(hook2)
	s.Register(hook3)

	stats := s.Stats()

	if stats["enabled"] != true {
		t.Error("Expected enabled=true in stats")
	}
	if stats["total_hooks"] != 3 {
		t.Errorf("Expected total_hooks=3, got %v", stats["total_hooks"])
	}

	hooksByType := stats["hooks_by_type"].(map[string]int)
	if hooksByType[string(HookTypePreRequest)] != 2 {
		t.Error("Expected 2 pre-request hooks in stats")
	}
	if hooksByType[string(HookTypePostResponse)] != 1 {
		t.Error("Expected 1 post-response hook in stats")
	}
}

func TestLocalRuntime(t *testing.T) {
	runtime := &LocalRuntime{}

	if runtime.Name() != "local" {
		t.Errorf("Expected name 'local', got '%s'", runtime.Name())
	}

	if !runtime.IsAvailable() {
		t.Error("Expected local runtime to be available")
	}

	input := &HookInput{Data: "test"}
	output, err := runtime.Execute(context.Background(), &ContainerConfig{}, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success")
	}
	if output.Data != "test" {
		t.Error("Expected pass-through of data")
	}
}

func TestHTTPRuntime(t *testing.T) {
	runtime := &HTTPRuntime{timeout: 1 * time.Second}

	if runtime.Name() != "http" {
		t.Errorf("Expected name 'http', got '%s'", runtime.Name())
	}

	if !runtime.IsAvailable() {
		t.Error("Expected HTTP runtime to be available")
	}

	// Test without URL
	input := &HookInput{Data: "test"}
	_, err := runtime.Execute(context.Background(), &ContainerConfig{}, input)
	if err == nil {
		t.Error("Expected error without URL")
	}

	// Test with URL (stub)
	config := &ContainerConfig{URL: "https://example.com/hook"}
	output, err := runtime.Execute(context.Background(), config, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success")
	}
}

func TestConfigurableHook(t *testing.T) {
	config := HookConfig{
		Type:     HookTypePreRequest,
		Name:     "configurable-hook",
		Enabled:  true,
		Priority: 5,
	}

	hook := NewConfigurableHook(config, nil)

	if hook.Type() != HookTypePreRequest {
		t.Error("Expected pre_request type")
	}
	if hook.Name() != "configurable-hook" {
		t.Error("Expected configurable-hook name")
	}
	if hook.Priority() != 5 {
		t.Error("Expected priority 5")
	}
	if !hook.IsEnabled() {
		t.Error("Expected enabled")
	}

	// Test execution without runtime or executor (pass through)
	input := &HookInput{Data: "test"}
	output, err := hook.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output.Data != "test" {
		t.Error("Expected pass-through")
	}

	// Test with custom executor
	executed := false
	hook.SetExecutor(func(ctx context.Context, input *HookInput) (*HookOutput, error) {
		executed = true
		return &HookOutput{Success: true, Data: "custom"}, nil
	})

	output, err = hook.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !executed {
		t.Error("Expected executor to be called")
	}
	if output.Data != "custom" {
		t.Error("Expected custom data")
	}
}

func TestConfigurableHookWithRuntime(t *testing.T) {
	config := HookConfig{
		Type:    HookTypePreRequest,
		Name:    "runtime-hook",
		Enabled: true,
		Container: ContainerConfig{
			Runtime: "local",
		},
	}

	runtime := &LocalRuntime{}
	hook := NewConfigurableHook(config, runtime)

	input := &HookInput{Data: "test"}
	output, err := hook.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !output.Success {
		t.Error("Expected success")
	}
}

func TestParseHookConfig(t *testing.T) {
	// JSON duration is in nanoseconds, 100000000 = 100ms
	jsonConfig := `{
		"type": "pre_request",
		"name": "test-hook",
		"enabled": true,
		"priority": 10,
		"timeout": 100000000,
		"container": {
			"runtime": "http",
			"url": "https://example.com/hook"
		}
	}`

	config, err := ParseHookConfig([]byte(jsonConfig))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if config.Type != HookTypePreRequest {
		t.Error("Expected pre_request type")
	}
	if config.Name != "test-hook" {
		t.Error("Expected test-hook name")
	}
	if !config.Enabled {
		t.Error("Expected enabled")
	}
	if config.Priority != 10 {
		t.Error("Expected priority 10")
	}
	if config.Timeout != 100*time.Millisecond {
		t.Errorf("Expected 100ms timeout, got %v", config.Timeout)
	}
	if config.Container.Runtime != "http" {
		t.Error("Expected http runtime")
	}
	if config.Container.URL != "https://example.com/hook" {
		t.Error("Expected correct URL")
	}

	// Test invalid JSON
	_, err = ParseHookConfig([]byte("invalid"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()

	if config.Enabled {
		t.Error("Expected disabled by default")
	}
	if config.DefaultTimeout != 100*time.Millisecond {
		t.Error("Expected 100ms default timeout")
	}
	if config.MaxConcurrentHooks != 10 {
		t.Error("Expected 10 max concurrent hooks")
	}
	if !config.FailOpen {
		t.Error("Expected fail open by default")
	}
	if !config.MetricsEnabled {
		t.Error("Expected metrics enabled by default")
	}
}

func TestDefaultService(t *testing.T) {
	if DefaultService == nil {
		t.Error("DefaultService should not be nil")
	}
	if DefaultService.IsEnabled() {
		t.Error("DefaultService should be disabled by default")
	}
}

func TestHookChaining(t *testing.T) {
	config := &ServiceConfig{
		Enabled:        true,
		DefaultTimeout: 1 * time.Second,
		FailOpen:       true,
	}
	s := NewService(config)

	// Register hooks that chain transformations
	hook1 := &mockHook{
		hookType: HookTypePreRequest,
		name:     "adder",
		enabled:  true,
		priority: 1,
		execFn: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
			data := input.Data.(int)
			return &HookOutput{Success: true, Data: data + 10}, nil
		},
	}

	hook2 := &mockHook{
		hookType: HookTypePreRequest,
		name:     "multiplier",
		enabled:  true,
		priority: 2,
		execFn: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
			data := input.Data.(int)
			return &HookOutput{Success: true, Data: data * 2}, nil
		},
	}

	s.Register(hook1)
	s.Register(hook2)

	input := &HookInput{Data: 5}
	output, err := s.ExecuteHooks(context.Background(), HookTypePreRequest, input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 5 + 10 = 15, 15 * 2 = 30
	if output.Data != 30 {
		t.Errorf("Expected 30, got %v", output.Data)
	}
}
