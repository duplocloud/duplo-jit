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

func TestOverrideConsoleRegion(t *testing.T) {
	// A realistic federation URL with a URL-encoded Destination (lowercase %3d,
	// matching what the Duplo API returns).
	realistic := "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc-DEF_123&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2"

	tests := []struct {
		name       string
		consoleUrl string
		region     string
		wantRegion string // expected Destination region; "" means URL should be unchanged
		unchanged  bool
	}{
		{
			name:       "overrides region in destination",
			consoleUrl: realistic,
			region:     "us-east-2",
			wantRegion: "us-east-2",
		},
		{
			name:       "empty region leaves url unchanged",
			consoleUrl: realistic,
			region:     "",
			unchanged:  true,
		},
		{
			name:       "empty url returns empty",
			consoleUrl: "",
			region:     "us-east-2",
			unchanged:  true,
		},
		{
			name:       "no destination param leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&SigninToken=abc",
			region:     "us-east-2",
			unchanged:  true,
		},
		{
			name:       "destination without region param leaves url unchanged",
			consoleUrl: "https://signin.aws.amazon.com/federation?Action=login&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome",
			region:     "us-east-2",
			unchanged:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OverrideConsoleRegion(tt.consoleUrl, tt.region)

			if tt.unchanged {
				if got != tt.consoleUrl {
					t.Fatalf("expected url unchanged, got %q", got)
				}
				return
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
	consoleUrl := "https://signin.aws.amazon.com/federation?Action=login&SigninToken=tok&Destination=https%3a%2f%2fconsole.aws.amazon.com%2fconsole%2fhome%3fregion%3dus-west-2"

	t.Run("overrides region and console url", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: consoleUrl}
		ApplyTenantRegion(creds, "us-east-2")

		if creds.Region != "us-east-2" {
			t.Fatalf("Region = %q, want us-east-2", creds.Region)
		}
		if r := destinationRegion(t, creds.ConsoleUrl); r != "us-east-2" {
			t.Fatalf("console destination region = %q, want us-east-2", r)
		}
	})

	t.Run("empty region is a no-op", func(t *testing.T) {
		creds := &AwsConfigOutput{Region: "us-west-2", ConsoleUrl: consoleUrl}
		ApplyTenantRegion(creds, "")

		if creds.Region != "us-west-2" || creds.ConsoleUrl != consoleUrl {
			t.Fatalf("expected creds unchanged, got region=%q", creds.Region)
		}
	})

	t.Run("nil creds does not panic", func(t *testing.T) {
		ApplyTenantRegion(nil, "us-east-2")
	})
}
