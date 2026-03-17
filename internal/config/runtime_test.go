package config

import (
	"testing"
	"time"
)

func TestLoadRuntimeUsesDefaults(t *testing.T) {
	t.Setenv("AGENTCOM_CLIENT_DIAL_TIMEOUT", "")
	t.Setenv("AGENTCOM_SERVER_READ_TIMEOUT", "")
	t.Setenv("AGENTCOM_HEARTBEAT_INTERVAL", "")

	runtimeCfg, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if runtimeCfg.ClientDialTimeout != 5*time.Second {
		t.Fatalf("ClientDialTimeout = %v, want 5s", runtimeCfg.ClientDialTimeout)
	}
	if runtimeCfg.ClientWriteTimeout != 5*time.Second {
		t.Fatalf("ClientWriteTimeout = %v, want 5s", runtimeCfg.ClientWriteTimeout)
	}
	if runtimeCfg.ServerAcceptTimeout != time.Second {
		t.Fatalf("ServerAcceptTimeout = %v, want 1s", runtimeCfg.ServerAcceptTimeout)
	}
	if runtimeCfg.ServerReadTimeout != 30*time.Second {
		t.Fatalf("ServerReadTimeout = %v, want 30s", runtimeCfg.ServerReadTimeout)
	}
	if runtimeCfg.HeartbeatInterval != 10*time.Second {
		t.Fatalf("HeartbeatInterval = %v, want 10s", runtimeCfg.HeartbeatInterval)
	}
	if runtimeCfg.HeartbeatStaleThreshold != 30*time.Second {
		t.Fatalf("HeartbeatStaleThreshold = %v, want 30s", runtimeCfg.HeartbeatStaleThreshold)
	}
	if runtimeCfg.PollInterval != 5*time.Second {
		t.Fatalf("PollInterval = %v, want 5s", runtimeCfg.PollInterval)
	}
	if runtimeCfg.SupervisorHealthCheckInterval != 5*time.Second {
		t.Fatalf("SupervisorHealthCheckInterval = %v, want 5s", runtimeCfg.SupervisorHealthCheckInterval)
	}
	if len(runtimeCfg.ClientRetryBackoffs) != 3 || runtimeCfg.ClientRetryBackoffs[0] != 100*time.Millisecond || runtimeCfg.ClientRetryBackoffs[2] != 400*time.Millisecond {
		t.Fatalf("ClientRetryBackoffs = %v, want [100ms 200ms 400ms]", runtimeCfg.ClientRetryBackoffs)
	}
	if runtimeCfg.ClientRetryJitterMax != 25*time.Millisecond {
		t.Fatalf("ClientRetryJitterMax = %v, want 25ms", runtimeCfg.ClientRetryJitterMax)
	}
}

func TestLoadRuntimeUsesEnvOverrides(t *testing.T) {
	t.Setenv("AGENTCOM_CLIENT_DIAL_TIMEOUT", "7s")
	t.Setenv("AGENTCOM_CLIENT_WRITE_TIMEOUT", "8s")
	t.Setenv("AGENTCOM_SERVER_ACCEPT_TIMEOUT", "3s")
	t.Setenv("AGENTCOM_SERVER_READ_TIMEOUT", "45s")
	t.Setenv("AGENTCOM_HEARTBEAT_INTERVAL", "2s")
	t.Setenv("AGENTCOM_HEARTBEAT_STALE_THRESHOLD", "9s")
	t.Setenv("AGENTCOM_POLL_INTERVAL", "4s")
	t.Setenv("AGENTCOM_SUPERVISOR_HEALTH_INTERVAL", "6s")
	t.Setenv("AGENTCOM_CLIENT_RETRY_BACKOFFS", "50ms,75ms")
	t.Setenv("AGENTCOM_CLIENT_RETRY_JITTER_MAX", "9ms")

	runtimeCfg, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if runtimeCfg.ClientDialTimeout != 7*time.Second {
		t.Fatalf("ClientDialTimeout = %v, want 7s", runtimeCfg.ClientDialTimeout)
	}
	if runtimeCfg.ClientWriteTimeout != 8*time.Second {
		t.Fatalf("ClientWriteTimeout = %v, want 8s", runtimeCfg.ClientWriteTimeout)
	}
	if runtimeCfg.ServerAcceptTimeout != 3*time.Second {
		t.Fatalf("ServerAcceptTimeout = %v, want 3s", runtimeCfg.ServerAcceptTimeout)
	}
	if runtimeCfg.ServerReadTimeout != 45*time.Second {
		t.Fatalf("ServerReadTimeout = %v, want 45s", runtimeCfg.ServerReadTimeout)
	}
	if runtimeCfg.HeartbeatInterval != 2*time.Second {
		t.Fatalf("HeartbeatInterval = %v, want 2s", runtimeCfg.HeartbeatInterval)
	}
	if runtimeCfg.HeartbeatStaleThreshold != 9*time.Second {
		t.Fatalf("HeartbeatStaleThreshold = %v, want 9s", runtimeCfg.HeartbeatStaleThreshold)
	}
	if runtimeCfg.PollInterval != 4*time.Second {
		t.Fatalf("PollInterval = %v, want 4s", runtimeCfg.PollInterval)
	}
	if runtimeCfg.SupervisorHealthCheckInterval != 6*time.Second {
		t.Fatalf("SupervisorHealthCheckInterval = %v, want 6s", runtimeCfg.SupervisorHealthCheckInterval)
	}
	if len(runtimeCfg.ClientRetryBackoffs) != 2 || runtimeCfg.ClientRetryBackoffs[0] != 50*time.Millisecond || runtimeCfg.ClientRetryBackoffs[1] != 75*time.Millisecond {
		t.Fatalf("ClientRetryBackoffs = %v, want [50ms 75ms]", runtimeCfg.ClientRetryBackoffs)
	}
	if runtimeCfg.ClientRetryJitterMax != 9*time.Millisecond {
		t.Fatalf("ClientRetryJitterMax = %v, want 9ms", runtimeCfg.ClientRetryJitterMax)
	}
}
