package internal

import "testing"

// withTestCache points the package-level cache at a fresh temp dir for the duration of the test,
// restoring the previous state afterward — these globals are normally set once by MustInitCache.
func withTestCache(t *testing.T) {
	t.Helper()
	prevDir, prevNoCache := cacheDir, noCache
	cacheDir = t.TempDir()
	noCache = false
	t.Cleanup(func() {
		cacheDir, noCache = prevDir, prevNoCache
	})
}

func TestHelpdeskTokenCache_RoundTripsAndInvalidates(t *testing.T) {
	withTestCache(t)

	const host = "https://helpdesk.example.com"

	if got := CacheGetHelpdeskTokenUnchecked(host); got != "" {
		t.Fatalf("expected no cached token before any write, got %q", got)
	}

	cacheWriteMustMarshal(helpdeskAuthCacheFile(host), &HelpdeskAuthOutput{Token: "jwt-1"})

	if got := CacheGetHelpdeskTokenUnchecked(host); got != "jwt-1" {
		t.Errorf("CacheGetHelpdeskTokenUnchecked() = %q, want %q", got, "jwt-1")
	}

	InvalidateHelpdeskToken(host)

	if got := CacheGetHelpdeskTokenUnchecked(host); got != "" {
		t.Errorf("expected cache cleared after InvalidateHelpdeskToken, got %q", got)
	}
}

func TestHelpdeskTokenCache_IsolatedPerHost(t *testing.T) {
	withTestCache(t)

	cacheWriteMustMarshal(helpdeskAuthCacheFile("https://a.example.com"), &HelpdeskAuthOutput{Token: "jwt-a"})
	cacheWriteMustMarshal(helpdeskAuthCacheFile("https://b.example.com"), &HelpdeskAuthOutput{Token: "jwt-b"})

	if got := CacheGetHelpdeskTokenUnchecked("https://a.example.com"); got != "jwt-a" {
		t.Errorf("host a: got %q, want %q", got, "jwt-a")
	}
	if got := CacheGetHelpdeskTokenUnchecked("https://b.example.com"); got != "jwt-b" {
		t.Errorf("host b: got %q, want %q", got, "jwt-b")
	}

	InvalidateHelpdeskToken("https://a.example.com")

	if got := CacheGetHelpdeskTokenUnchecked("https://a.example.com"); got != "" {
		t.Errorf("host a should be cleared, got %q", got)
	}
	if got := CacheGetHelpdeskTokenUnchecked("https://b.example.com"); got != "jwt-b" {
		t.Errorf("host b must be unaffected by host a's invalidation, got %q", got)
	}
}

func TestMustHelpdeskClient_ExplicitTokenNeverTouchesCache(t *testing.T) {
	withTestCache(t)

	const host = "https://helpdesk.example.com"
	client := MustHelpdeskClient(host, host, "explicit-token", false, 0)

	if client.Token != "explicit-token" {
		t.Errorf("client.Token = %q, want %q", client.Token, "explicit-token")
	}
	if got := CacheGetHelpdeskTokenUnchecked(host); got != "" {
		t.Errorf("an explicitly-supplied token must never be written to the interactive cache, got %q", got)
	}
}

func TestMustHelpdeskClient_EnvVarFallback_NeverTouchesCache(t *testing.T) {
	withTestCache(t)
	t.Setenv("DUPLO_JIT_TOKEN", "env-token")

	const host = "https://helpdesk.example.com"
	client := MustHelpdeskClient(host, host, "", false, 0)

	if client.Token != "env-token" {
		t.Errorf("client.Token = %q, want %q", client.Token, "env-token")
	}
	if got := CacheGetHelpdeskTokenUnchecked(host); got != "" {
		t.Errorf("DUPLO_JIT_TOKEN must never be written to the interactive cache, got %q", got)
	}
}

func TestMustHelpdeskClient_UsesCachedTokenWhenPresent(t *testing.T) {
	withTestCache(t)

	const host = "https://helpdesk.example.com"
	cacheWriteMustMarshal(helpdeskAuthCacheFile(host), &HelpdeskAuthOutput{Token: "cached-token"})

	client := MustHelpdeskClient(host, host, "", false, 0)

	if client.Token != "cached-token" {
		t.Errorf("client.Token = %q, want %q", client.Token, "cached-token")
	}
}
