package internal

import (
	"net/url"
	"testing"
)

// destinationRegion parses the federation console URL and returns the region
// query param of its Destination target.
func destinationRegion(t *testing.T, consoleUrl string) string {
	t.Helper()
	u, err := url.Parse(consoleUrl)
	if err != nil {
		t.Fatalf("parse console url: %s", err)
	}
	dest := u.Query().Get("Destination")
	if dest == "" {
		return ""
	}
	d, err := url.Parse(dest)
	if err != nil {
		t.Fatalf("parse destination: %s", err)
	}
	return d.Query().Get("region")
}

// signinToken returns the SigninToken query param of a federation console URL.
func signinToken(t *testing.T, consoleUrl string) string {
	t.Helper()
	u, err := url.Parse(consoleUrl)
	if err != nil {
		t.Fatalf("parse console url: %s", err)
	}
	return u.Query().Get("SigninToken")
}

// assertUnchanged checks that OverrideConsoleRegion declined to rewrite and returned its input.
func assertUnchanged(t *testing.T, in, got string, ok bool) {
	t.Helper()
	if ok {
		t.Fatalf("expected ok=false")
	}
	if got != in {
		t.Fatalf("expected url unchanged, got %q", got)
	}
}

// assertRewritten checks that OverrideConsoleRegion rewrote the Destination region and kept the SigninToken.
func assertRewritten(t *testing.T, got string, ok bool, wantRegion, wantToken string) {
	t.Helper()
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if r := destinationRegion(t, got); r != wantRegion {
		t.Fatalf("destination region = %q, want %q", r, wantRegion)
	}
	if tok := signinToken(t, got); tok != wantToken {
		t.Fatalf("SigninToken = %q, want %q", tok, wantToken)
	}
}

// assertCreds checks the Region and ConsoleUrl left on the credentials by ApplyTenantRegion.
func assertCreds(t *testing.T, creds *AwsConfigOutput, wantRegion, wantUrl string) {
	t.Helper()
	if creds.Region != wantRegion || creds.ConsoleUrl != wantUrl {
		t.Fatalf("got region=%q url=%q, want region=%q url=%q", creds.Region, creds.ConsoleUrl, wantRegion, wantUrl)
	}
}

// A realistic federation URL with a URL-encoded Destination (lowercase %3d,
// matching what the Duplo API returns).
const realisticConsoleUrl = "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc-DEF_123&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2"

// A Destination with the region encoded in the host rather than the query, which the rewrite cannot handle.
const hostRegionConsoleUrl = "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc&Destination=https%3a%2f%2fus-west-2.console.aws.amazon.com%2fconsole%2fhome"

func TestOverrideConsoleRegion(t *testing.T) {
	tests := []struct {
		name       string
		consoleUrl string
		region     string
		wantRegion string // expected Destination region; "" means the URL must come back unchanged
	}{
		{
			name:       "overrides region in destination",
			consoleUrl: realisticConsoleUrl,
			region:     "us-east-2",
			wantRegion: "us-east-2",
		},
		{
			name:       "empty region leaves url unchanged",
			consoleUrl: realisticConsoleUrl,
			region:     "",
		},
		{
			name:       "empty url returns empty",
			consoleUrl: "",
			region:     "us-east-2",
		},
		{
			name:       "unparseable url is left unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login\x7f",
			region:     "us-east-2",
		},
		{
			name:       "no destination param leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc",
			region:     "us-east-2",
		},
		{
			name:       "destination without region param leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome",
			region:     "us-east-2",
		},
		{
			name:       "region in destination host is not rewritten",
			consoleUrl: hostRegionConsoleUrl,
			region:     "us-east-2",
		},
		{
			// url.Parse accepts these, but the query does not fully parse; re-encoding
			// would drop the SigninToken, so the input must come back untouched.
			name:       "malformed escape in query leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&SigninToken=ab%zz&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2",
			region:     "us-east-2",
		},
		{
			name:       "semicolon in query leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc;def&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2",
			region:     "us-east-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OverrideConsoleRegion(tt.consoleUrl, tt.region)
			if tt.wantRegion == "" {
				assertUnchanged(t, tt.consoleUrl, got, ok)
			} else {
				assertRewritten(t, got, ok, tt.wantRegion, "abc-DEF_123")
			}
		})
	}
}

func TestApplyTenantRegion(t *testing.T) {
	t.Run("overrides region and console url", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: realisticConsoleUrl}
		if !ApplyTenantRegion(creds, "us-east-2") {
			t.Fatalf("expected true")
		}
		if creds.Region != "us-east-2" {
			t.Fatalf("Region = %q, want us-east-2", creds.Region)
		}
		if r := destinationRegion(t, creds.ConsoleUrl); r != "us-east-2" {
			t.Fatalf("console destination region = %q, want us-east-2", r)
		}
	})

	t.Run("empty console url sets region only", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2"}
		if !ApplyTenantRegion(creds, "us-east-2") {
			t.Fatalf("expected true")
		}
		assertCreds(t, creds, "us-east-2", "")
	})

	t.Run("console url that cannot be rewritten reports false", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: hostRegionConsoleUrl}
		if ApplyTenantRegion(creds, "us-east-2") {
			t.Fatalf("expected false")
		}
		assertCreds(t, creds, "us-east-2", hostRegionConsoleUrl)
	})

	t.Run("empty region is a no-op", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: realisticConsoleUrl}
		if ApplyTenantRegion(creds, "") {
			t.Fatalf("expected false")
		}
		assertCreds(t, creds, "us-west-2", realisticConsoleUrl)
	})

	t.Run("nil creds does not panic", func(t *testing.T) {
		if ApplyTenantRegion(nil, "us-east-2") {
			t.Fatalf("expected false")
		}
	})
}
