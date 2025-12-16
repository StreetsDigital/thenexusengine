package container

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/StreetsDigital/thenexusengine/pbs/pkg/logger"
)

// Service provides container hook execution
type Service struct {
	mu       sync.RWMutex
	hooks    map[string]Hook
	runtimes map[string]Runtime
	config   *ServiceConfig
	enabled  bool
}

// ServiceConfig configures the container service
type ServiceConfig struct {
	// Enabled enables the container service
	Enabled bool `json:"enabled"`

	// DefaultTimeout for hook execution
	DefaultTimeout time.Duration `json:"default_timeout"`

	// MaxConcurrentHooks limits parallel hook execution
	MaxConcurrentHooks int `json:"max_concurrent_hooks"`

	// FailOpen determines behavior on hook error
	// If true, continues processing even if hooks fail
	FailOpen bool `json:"fail_open"`

	// MetricsEnabled enables hook execution metrics
	MetricsEnabled bool `json:"metrics_enabled"`
}

// DefaultServiceConfig returns default configuration
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		Enabled:            false, // Disabled by default
		DefaultTimeout:     100 * time.Millisecond,
		MaxConcurrentHooks: 10,
		FailOpen:           true,
		MetricsEnabled:     true,
	}
}

// NewService creates a new container service
func NewService(config *ServiceConfig) *Service {
	if config == nil {
		config = DefaultServiceConfig()
	}

	s := &Service{
		hooks:    make(map[string]Hook),
		runtimes: make(map[string]Runtime),
		config:   config,
		enabled:  config.Enabled,
	}

	// Register built-in runtimes
	s.registerBuiltinRuntimes()

	return s
}

// registerBuiltinRuntimes registers the default runtimes
func (s *Service) registerBuiltinRuntimes() {
	// Register local runtime (for development/testing)
	s.runtimes["local"] = &LocalRuntime{}

	// Register HTTP runtime (for serverless functions)
	s.runtimes["http"] = &HTTPRuntime{
		timeout: s.config.DefaultTimeout,
	}

	// Future runtimes can be added here:
	// - WASM runtime for edge compute
	// - Docker runtime for containerized functions
	// - Lambda/CloudFlare Workers/Fastly Compute integrations
}

// Register adds a hook to the service
func (s *Service) Register(hook Hook) error {
	if hook == nil {
		return fmt.Errorf("hook cannot be nil")
	}

	name := hook.Name()
	if name == "" {
		return fmt.Errorf("hook name cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.hooks[name]; exists {
		return fmt.Errorf("hook %s already registered", name)
	}

	s.hooks[name] = hook
	logger.Log.Info().
		Str("hook", name).
		Str("type", string(hook.Type())).
		Msg("Container hook registered")

	return nil
}

// Unregister removes a hook from the service
func (s *Service) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.hooks[name]; !exists {
		return fmt.Errorf("hook %s not found", name)
	}

	delete(s.hooks, name)
	logger.Log.Info().
		Str("hook", name).
		Msg("Container hook unregistered")

	return nil
}

// GetHooks returns all hooks of the given type, sorted by priority
func (s *Service) GetHooks(hookType HookType) []Hook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Hook
	for _, hook := range s.hooks {
		if hook.Type() == hookType && hook.IsEnabled() {
			result = append(result, hook)
		}
	}

	// Sort by priority (assuming hooks implement a Priority method)
	sort.Slice(result, func(i, j int) bool {
		pi, pj := 0, 0
		if ph, ok := result[i].(interface{ Priority() int }); ok {
			pi = ph.Priority()
		}
		if ph, ok := result[j].(interface{ Priority() int }); ok {
			pj = ph.Priority()
		}
		return pi < pj
	})

	return result
}

// ExecuteHooks runs all hooks of the given type
func (s *Service) ExecuteHooks(ctx context.Context, hookType HookType, input *HookInput) (*HookOutput, error) {
	if !s.enabled {
		// Pass through when disabled
		return &HookOutput{
			Success: true,
			Data:    input.Data,
		}, nil
	}

	hooks := s.GetHooks(hookType)
	if len(hooks) == 0 {
		return &HookOutput{
			Success: true,
			Data:    input.Data,
		}, nil
	}

	// Execute hooks in sequence
	currentOutput := &HookOutput{
		Success: true,
		Data:    input.Data,
	}

	for _, hook := range hooks {
		// Create timeout context for this hook
		hookCtx, cancel := context.WithTimeout(ctx, s.config.DefaultTimeout)

		// Update input with current data
		hookInput := &HookInput{
			Context: input.Context,
			Data:    currentOutput.Data,
		}

		start := time.Now()
		output, err := hook.Execute(hookCtx, hookInput)
		duration := time.Since(start)

		cancel()

		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("hook", hook.Name()).
				Str("type", string(hookType)).
				Dur("duration", duration).
				Msg("Container hook execution failed")

			if !s.config.FailOpen {
				return &HookOutput{
					Success: false,
					Error:   err.Error(),
					Metrics: &HookMetrics{Duration: duration},
				}, err
			}
			// FailOpen: continue with previous data
			continue
		}

		if output != nil {
			if output.Data != nil {
				currentOutput.Data = output.Data
			}
			if output.Metrics != nil {
				currentOutput.Metrics = output.Metrics
			}
		}

		logger.Log.Debug().
			Str("hook", hook.Name()).
			Str("type", string(hookType)).
			Dur("duration", duration).
			Msg("Container hook executed")
	}

	return currentOutput, nil
}

// IsEnabled returns whether the service is enabled
func (s *Service) IsEnabled() bool {
	return s.enabled
}

// Enable enables the container service
func (s *Service) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

// Disable disables the container service
func (s *Service) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

// Stats returns service statistics
func (s *Service) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hookCounts := make(map[string]int)
	for _, hook := range s.hooks {
		hookCounts[string(hook.Type())]++
	}

	return map[string]interface{}{
		"enabled":        s.enabled,
		"total_hooks":    len(s.hooks),
		"hooks_by_type":  hookCounts,
		"total_runtimes": len(s.runtimes),
	}
}

// LocalRuntime executes hooks locally (for development/testing)
type LocalRuntime struct{}

func (r *LocalRuntime) Name() string {
	return "local"
}

func (r *LocalRuntime) Execute(ctx context.Context, config *ContainerConfig, input *HookInput) (*HookOutput, error) {
	// Local runtime is a no-op pass-through
	return &HookOutput{
		Success: true,
		Data:    input.Data,
	}, nil
}

func (r *LocalRuntime) IsAvailable() bool {
	return true
}

// HTTPRuntime executes hooks via HTTP calls
type HTTPRuntime struct {
	timeout time.Duration
}

func (r *HTTPRuntime) Name() string {
	return "http"
}

func (r *HTTPRuntime) Execute(ctx context.Context, config *ContainerConfig, input *HookInput) (*HookOutput, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("HTTP runtime requires URL")
	}

	// This is a stub - actual implementation would make HTTP call
	// to the configured endpoint with the input payload
	logger.Log.Debug().
		Str("url", config.URL).
		Msg("HTTP runtime execution (stub)")

	return &HookOutput{
		Success: true,
		Data:    input.Data,
	}, nil
}

func (r *HTTPRuntime) IsAvailable() bool {
	return true
}

// ConfigurableHook is a hook that can be configured via JSON
type ConfigurableHook struct {
	config   HookConfig
	runtime  Runtime
	executor func(ctx context.Context, input *HookInput) (*HookOutput, error)
}

// NewConfigurableHook creates a new configurable hook
func NewConfigurableHook(config HookConfig, runtime Runtime) *ConfigurableHook {
	return &ConfigurableHook{
		config:  config,
		runtime: runtime,
	}
}

func (h *ConfigurableHook) Type() HookType {
	return h.config.Type
}

func (h *ConfigurableHook) Name() string {
	return h.config.Name
}

func (h *ConfigurableHook) Priority() int {
	return h.config.Priority
}

func (h *ConfigurableHook) IsEnabled() bool {
	return h.config.Enabled
}

func (h *ConfigurableHook) Execute(ctx context.Context, input *HookInput) (*HookOutput, error) {
	if h.executor != nil {
		return h.executor(ctx, input)
	}

	if h.runtime != nil {
		return h.runtime.Execute(ctx, &h.config.Container, input)
	}

	// Default: pass through
	return &HookOutput{
		Success: true,
		Data:    input.Data,
	}, nil
}

// SetExecutor sets a custom executor function
func (h *ConfigurableHook) SetExecutor(fn func(ctx context.Context, input *HookInput) (*HookOutput, error)) {
	h.executor = fn
}

// ParseHookConfig parses a hook configuration from JSON
func ParseHookConfig(data []byte) (*HookConfig, error) {
	var config HookConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// DefaultService is the global container service instance
var DefaultService = NewService(DefaultServiceConfig())
