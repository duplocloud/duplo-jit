package main

import (
	"testing"

	"github.com/duplocloud/duplo-jit/duplocloud"
)

// TestValidateHelpdeskK8sArgs guards the three-way mutually-exclusive identifier choice: exactly
// one of --cluster-id/--scope-id/--namespace-id, the last requiring --env-id and --rg-id too.
func TestValidateHelpdeskK8sArgs(t *testing.T) {
	cases := []struct {
		name                                            string
		hasCluster, hasScope, hasNamespace, hasEnvAndRg bool
		wantErr                                         bool
	}{
		{"none selected", false, false, false, false, true},
		{"cluster only", true, false, false, false, false},
		{"scope only", false, true, false, false, false},
		{"namespace with env+rg", false, false, true, true, false},
		{"namespace without env+rg", false, false, true, false, true},
		{"cluster and scope both set", true, true, false, false, true},
		{"cluster and namespace both set", true, false, true, true, true},
		{"scope and namespace both set", false, true, true, true, true},
		{"all three set", true, true, true, true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateHelpdeskK8sArgs(c.hasCluster, c.hasScope, c.hasNamespace, c.hasEnvAndRg)
			if (err != nil) != c.wantErr {
				t.Errorf("validateHelpdeskK8sArgs(%v,%v,%v,%v) error = %v, wantErr %v",
					c.hasCluster, c.hasScope, c.hasNamespace, c.hasEnvAndRg, err, c.wantErr)
			}
		})
	}
}

// fakeClientError is a minimal duplocloud.ClientError stub for exercising
// callHelpdeskWithRetry's retry decision without a real HTTP round trip.
type fakeClientError struct{ status int }

func (e fakeClientError) Error() string                    { return "fake client error" }
func (e fakeClientError) Status() int                      { return e.status }
func (e fakeClientError) PossibleMissingAPI() bool         { return false }
func (e fakeClientError) URL() string                      { return "" }
func (e fakeClientError) Response() map[string]interface{} { return nil }

// TestCallHelpdeskWithRetry_401AndNoExplicitToken_RetriesOnce verifies the one case this
// function exists for: a cached/interactive token that the backend now rejects gets cleared and
// retried exactly once. A literal, non-empty token is passed so MustHelpdeskClient's own
// token-resolution never touches the real cache or network on either call — only
// noExplicitToken (decoupled from the token value on purpose, to isolate this function's own
// branching from its real caller's invariant) drives whether a retry happens.
func TestCallHelpdeskWithRetry_401AndNoExplicitToken_RetriesOnce(t *testing.T) {
	calls := 0
	fn := func(*duplocloud.Client) (*duplocloud.HelpdeskK8sClusterConfig, duplocloud.ClientError) {
		calls++
		if calls == 1 {
			return nil, fakeClientError{status: 401}
		}
		return &duplocloud.HelpdeskK8sClusterConfig{Token: "fresh-token"}, nil
	}

	result, err := callHelpdeskWithRetry("https://helpdesk.example.com", "https://helpdesk.example.com",
		"explicit-token", false, 0, true, fn)

	if calls != 2 {
		t.Errorf("fn called %d times, want 2 (one failure, one retry)", calls)
	}
	if err != nil {
		t.Errorf("unexpected error after retry: %v", err)
	}
	if result == nil || result.Token != "fresh-token" {
		t.Errorf("result = %+v, want Token=fresh-token", result)
	}
}

// TestCallHelpdeskWithRetry_401ButExplicitToken_FailsLoudlyNoRetry verifies the safety property
// the doc comment promises: an explicit --token/DUPLO_JIT_TOKEN that the backend rejects must
// fail immediately, never silently fall back to a fresh interactive login.
func TestCallHelpdeskWithRetry_401ButExplicitToken_FailsLoudlyNoRetry(t *testing.T) {
	calls := 0
	fn := func(*duplocloud.Client) (*duplocloud.HelpdeskK8sClusterConfig, duplocloud.ClientError) {
		calls++
		return nil, fakeClientError{status: 401}
	}

	result, err := callHelpdeskWithRetry("https://helpdesk.example.com", "https://helpdesk.example.com",
		"explicit-token", false, 0, false, fn)

	if calls != 1 {
		t.Errorf("fn called %d times, want 1 (no retry for an explicit token)", calls)
	}
	if err == nil || err.Status() != 401 {
		t.Errorf("err = %v, want a 401 ClientError", err)
	}
	if result != nil {
		t.Errorf("result = %+v, want nil on error", result)
	}
}

// TestCallHelpdeskWithRetry_NonAuthError_NeverRetries verifies only a 401 triggers the
// invalidate-and-retry cycle — any other failure (e.g. a 500 or network error) must not retry,
// since re-authenticating wouldn't fix it.
func TestCallHelpdeskWithRetry_NonAuthError_NeverRetries(t *testing.T) {
	calls := 0
	fn := func(*duplocloud.Client) (*duplocloud.HelpdeskK8sClusterConfig, duplocloud.ClientError) {
		calls++
		return nil, fakeClientError{status: 500}
	}

	_, err := callHelpdeskWithRetry("https://helpdesk.example.com", "https://helpdesk.example.com",
		"explicit-token", false, 0, true, fn)

	if calls != 1 {
		t.Errorf("fn called %d times, want 1 (a 500 must not trigger a retry)", calls)
	}
	if err == nil || err.Status() != 500 {
		t.Errorf("err = %v, want a 500 ClientError", err)
	}
}

// TestArgsContainFlag_HelpdeskInference guards the "subcommand name is optional" behavior: a
// kubeconfig's exec.args can omit "helpdesk-k8s" entirely as long as --workspace-id is present
// somewhere among the remaining flags.
func TestArgsContainFlag_HelpdeskInference(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare flag present", []string{"--host", "https://x", "--workspace-id", "ws-1"}, true},
		{"equals-form present", []string{"--host", "https://x", "--workspace-id=ws-1"}, true},
		{"absent", []string{"--host", "https://x", "--cluster-id", "c-1"}, false},
		{"empty args", []string{}, false},
		{"prefix collision must not match", []string{"--workspace-id-extra", "ws-1"}, false},
		{"value-only collision must not match", []string{"ws-1", "--workspace-id-extra"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := argsContainFlag(c.args, "workspace-id")
			if got != c.want {
				t.Errorf("argsContainFlag(%v, %q) = %v, want %v", c.args, "workspace-id", got, c.want)
			}
		})
	}
}
