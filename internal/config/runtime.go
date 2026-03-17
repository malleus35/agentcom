package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type RuntimeConfig struct {
	ClientDialTimeout             time.Duration
	ClientWriteTimeout            time.Duration
	ServerAcceptTimeout           time.Duration
	ServerReadTimeout             time.Duration
	HeartbeatInterval             time.Duration
	HeartbeatStaleThreshold       time.Duration
	PollInterval                  time.Duration
	SupervisorHealthCheckInterval time.Duration
	ClientRetryBackoffs           []time.Duration
	ClientRetryJitterMax          time.Duration
}

func LoadRuntime() (RuntimeConfig, error) {
	clientDialTimeout, err := loadDurationEnv("AGENTCOM_CLIENT_DIAL_TIMEOUT", 5*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	clientWriteTimeout, err := loadDurationEnv("AGENTCOM_CLIENT_WRITE_TIMEOUT", 5*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	serverAcceptTimeout, err := loadDurationEnv("AGENTCOM_SERVER_ACCEPT_TIMEOUT", time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	serverReadTimeout, err := loadDurationEnv("AGENTCOM_SERVER_READ_TIMEOUT", 30*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	heartbeatInterval, err := loadDurationEnv("AGENTCOM_HEARTBEAT_INTERVAL", 10*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	heartbeatStaleThreshold, err := loadDurationEnv("AGENTCOM_HEARTBEAT_STALE_THRESHOLD", 30*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	pollInterval, err := loadDurationEnv("AGENTCOM_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	supervisorHealthCheckInterval, err := loadDurationEnv("AGENTCOM_SUPERVISOR_HEALTH_INTERVAL", 5*time.Second)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	clientRetryBackoffs, err := loadDurationListEnv("AGENTCOM_CLIENT_RETRY_BACKOFFS", []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond})
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}
	clientRetryJitterMax, err := loadDurationEnv("AGENTCOM_CLIENT_RETRY_JITTER_MAX", 25*time.Millisecond)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("config.LoadRuntime: %w", err)
	}

	return RuntimeConfig{
		ClientDialTimeout:             clientDialTimeout,
		ClientWriteTimeout:            clientWriteTimeout,
		ServerAcceptTimeout:           serverAcceptTimeout,
		ServerReadTimeout:             serverReadTimeout,
		HeartbeatInterval:             heartbeatInterval,
		HeartbeatStaleThreshold:       heartbeatStaleThreshold,
		PollInterval:                  pollInterval,
		SupervisorHealthCheckInterval: supervisorHealthCheckInterval,
		ClientRetryBackoffs:           clientRetryBackoffs,
		ClientRetryJitterMax:          clientRetryJitterMax,
	}, nil
}

func loadDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func loadDurationListEnv(key string, fallback []time.Duration) ([]time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return append([]time.Duration(nil), fallback...), nil
	}
	parts := strings.Split(raw, ",")
	values := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		parsed, err := time.ParseDuration(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", key, err)
		}
		values = append(values, parsed)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("parse %s: empty duration list", key)
	}
	return values, nil
}
