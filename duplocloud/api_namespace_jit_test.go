package duplocloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHelpdeskGetNamespaceJitAccess_BuildsExpectedRequest verifies the request this CLI sends
// mirrors the backend's actual route exactly (method, path, query params) and that the bearer
// token on the request is the one the Client was constructed with — the DUPLO_JIT_TOKEN from the
// kubeconfig's exec.env, in the real CLI path (see the "helpdesk-k8s" case in cmd/duplo-jit).
func TestHelpdeskGetNamespaceJitAccess_BuildsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{
			Success: true,
			Data: HelpdeskK8sClusterConfig{
				ApiServer:        "https://cluster.example.com",
				Token:            "sa-token",
				DefaultNamespace: "team-a",
			},
		})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	result, err := client.HelpdeskGetNamespaceJitAccess("ws-1", "env-1", "rg-1", "ns-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws-1/environments/env-1/resource-groups/rg-1/namespaces/ns-1/k8s/jitAccess?isReadOnly=false"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}

	if gotAuth != "Bearer bearer-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer bearer-token")
	}

	if result.Token != "sa-token" || result.DefaultNamespace != "team-a" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestHelpdeskGetNamespaceJitAccess_ReadOnlyTrue_SetsQueryParam guards the one flag this method
// forwards to the backend — a caller passing isReadOnly=true must see it on the wire, since the
// backend narrows (never broadens) access based on it.
func TestHelpdeskGetNamespaceJitAccess_ReadOnlyTrue_SetsQueryParam(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{Success: true})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	_, err := client.HelpdeskGetNamespaceJitAccess("ws-1", "env-1", "rg-1", "ns-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws-1/environments/env-1/resource-groups/rg-1/namespaces/ns-1/k8s/jitAccess?isReadOnly=true"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}

// TestHelpdeskGetNamespaceJitAccess_ErrorResponse_PropagatesAsClientError verifies a non-2xx
// response is surfaced as a ClientError rather than silently producing a zero-value result —
// the caller (cmd/duplo-jit) relies on internal.DieIf(err, ...) to stop on exactly this signal.
func TestHelpdeskGetNamespaceJitAccess_ErrorResponse_PropagatesAsClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"Message": "namespace not found"})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	result, err := client.HelpdeskGetNamespaceJitAccess("ws-1", "env-1", "rg-1", "ns-missing", false)
	if err == nil {
		t.Fatal("expected a ClientError for a 404 response, got nil")
	}
	if result != nil {
		t.Errorf("expected a nil result on error, got %+v", result)
	}
	if err.Status() != http.StatusNotFound {
		t.Errorf("Status() = %d, want %d", err.Status(), http.StatusNotFound)
	}
}
