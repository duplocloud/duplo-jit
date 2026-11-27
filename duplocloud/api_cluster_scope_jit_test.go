package duplocloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHelpdeskGetClusterJitAccess_BuildsExpectedRequest mirrors
// TestHelpdeskGetNamespaceJitAccess_BuildsExpectedRequest for the cluster variant — nothing
// verified its method/path/auth-header shape before this test existed.
func TestHelpdeskGetClusterJitAccess_BuildsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{
			Success: true,
			Data: HelpdeskK8sClusterConfig{
				ApiServer: "https://cluster.example.com",
				Token:     "sa-token",
			},
		})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	result, err := client.HelpdeskGetClusterJitAccess("ws-1", "cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
	}

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws-1/environment/Clusters/cluster-1/jitAccess"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}

	if gotAuth != "Bearer bearer-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer bearer-token")
	}

	if result.Token != "sa-token" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestHelpdeskGetClusterJitAccess_EscapesPathSegments guards the url.PathEscape fix: an id
// containing a path separator must not be able to smuggle extra segments into the request path.
func TestHelpdeskGetClusterJitAccess_EscapesPathSegments(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath(), not Path (which net/url always decodes) — this must reflect what
		// actually went out on the wire, exactly like RequestURI() in the other tests here.
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{Success: true})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	_, _ = client.HelpdeskGetClusterJitAccess("ws/1", "cluster-1")

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws%2F1/environment/Clusters/cluster-1/jitAccess"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q (workspaceID's '/' must be percent-encoded, not treated as a path separator)", gotPath, wantPath)
	}
}

// TestHelpdeskGetScopeJitAccess_BuildsExpectedRequest mirrors the namespace variant's request-
// shape test for the scope-level method — previously the only one of the three new methods with
// no coverage at all besides the retry/auth-resolution tests in cmd/duplo-jit.
func TestHelpdeskGetScopeJitAccess_BuildsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{
			Success: true,
			Data:    HelpdeskK8sClusterConfig{ApiServer: "https://cluster.example.com", Token: "sa-token"},
		})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	result, err := client.HelpdeskGetScopeJitAccess("ws-1", "scope-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws-1/scopes/scope-1/k8s/jitAccess"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}

	if result.Token != "sa-token" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestHelpdeskGetScopeJitAccess_WithNamespace_SetsQueryParam guards the optional namespace query
// param this method forwards — already escaped via url.QueryEscape before this PR's other two
// methods were fixed to escape their path segments too.
func TestHelpdeskGetScopeJitAccess_WithNamespace_SetsQueryParam(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(helpdeskApiEnvelope{Success: true})
	}))
	defer server.Close()

	client := &Client{HostURL: server.URL, Token: "bearer-token", HTTPClient: server.Client()}

	_, err := client.HelpdeskGetScopeJitAccess("ws-1", "scope-1", "team a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPath := "/v1/aiservicedesk/user/data/workspaces/ws-1/scopes/scope-1/k8s/jitAccess?namespace=team+a"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}
