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

// A realistic federation URL with a URL-encoded Destination (lowercase %3d,
// matching what the Duplo API returns).
const realisticConsoleUrl = "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc-DEF_123&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2"

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
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc&Destination=https%3a%2f%2fus-west-2.console.aws.amazon.com%2fconsole%2fhome",
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
				if ok {
					t.Fatalf("expected ok=false")
				}
				if got != tt.consoleUrl {
					t.Fatalf("expected url unchanged, got %q", got)
				}
				return
			}

			if !ok {
				t.Fatalf("expected ok=true")
			}
			if r := destinationRegion(t, got); r != tt.wantRegion {
				t.Fatalf("destination region = %q, want %q", r, tt.wantRegion)
			}

			// The SigninToken must survive the rewrite intact.
			u, err := url.Parse(got)
			if err != nil {
				t.Fatalf("parse result: %s", err)
			}
			if tok := u.Query().Get("SigninToken"); tok != "abc-DEF_123" {
				t.Fatalf("SigninToken = %q, want abc-DEF_123", tok)
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
		if creds.Region != "us-east-2" || creds.ConsoleUrl != "" {
			t.Fatalf("got region=%q url=%q", creds.Region, creds.ConsoleUrl)
		}
	})

	t.Run("console url that cannot be rewritten reports false", func(t *testing.T) {
		hostRegion := "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc&Destination=https%3a%2f%2fus-west-2.console.aws.amazon.com%2fconsole%2fhome"
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: hostRegion}
		if ApplyTenantRegion(creds, "us-east-2") {
			t.Fatalf("expected false")
		}
		if creds.Region != "us-east-2" || creds.ConsoleUrl != hostRegion {
			t.Fatalf("got region=%q url=%q", creds.Region, creds.ConsoleUrl)
		}
	})

	t.Run("empty region is a no-op", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: realisticConsoleUrl}
		if ApplyTenantRegion(creds, "") {
			t.Fatalf("expected false")
		}
		if creds.Region != "us-west-2" || creds.ConsoleUrl != realisticConsoleUrl {
			t.Fatalf("expected creds unchanged, got region=%q", creds.Region)
		}
	})

	t.Run("nil creds does not panic", func(t *testing.T) {
		if ApplyTenantRegion(nil, "us-east-2") {
			t.Fatalf("expected false")
		}
	})
}
