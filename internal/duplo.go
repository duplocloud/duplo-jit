package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/duplocloud/duplo-jit/duplocloud"
)

type DuploCredsOutput struct {
	Version    int    `json:"Version"`
	DuploToken string `json:"DuploToken,omitempty"`
	NeedOTP    bool   `json:"NeedOTP"`
}

// duploClientAndOtpFlag returns a duplo client if and only if the token is valid and OTP is not needed.
func duploClientAndOtpFlag(host, token, otp string, admin bool) (*duplocloud.Client, bool) {
	client, err := duplocloud.NewClientWithOtp(host, token, otp)
	DieIf(err, "invalid arguments")
	features, err := client.FeaturesSystem() // this API call is doubling as a system "ping"

	// Is the token invalid?
	if err != nil {
		return nil, false
	}

	// Do we need to retrieve a OTP?
	if admin && features.IsOtpNeeded && otp == "" {
		return nil, true
	}

	// Otherwise, the client is usable.
	return client, false
}

// MustDuploClient retrieves a duplo client (and credentials) or panics.
func MustDuploClient(host, token string, interactive, admin bool) (client *duplocloud.Client, creds *DuploCredsOutput) {
	needsOtp := false
	cacheKey := strings.TrimPrefix(host, "https://")

	// Try non-interactive auth first.
	if token != "" {
		cacheRemoveEntry(cacheKey, "duplo") // never cache explicitly passed creds
		client, needsOtp = duploClientAndOtpFlag(host, token, "", admin)

		// If OTP is needed, we can only continue if interactive auth is allowed.
		if needsOtp {
			if !interactive {
				log.Fatalf("%s: server requires MFA but --interactive mode is disabled", os.Args[0])
			}

			// The client is usable, so we can return our result.
		} else if client != nil {
			creds = &DuploCredsOutput{
				Version:    1,
				DuploToken: token,
				NeedOTP:    needsOtp,
			}
			return

			// The client is not usable, so we have an error.
		} else {
			log.Fatalf("%s: authentication failure: failed to collect system features", os.Args[0])
		}
	}

	// Non-interactive auth was not available or not sufficient.
	if !interactive {
		log.Fatalf("%s: --token not specified and --interactive mode is disabled", os.Args[0])
	}

	// Next, we load and validate Duplo credentials from the cache.
	if token == "" {
		creds = CacheGetDuploOutput(cacheKey, host)
		if creds != nil {
			client, _ = duploClientAndOtpFlag(host, creds.DuploToken, "", admin)
		}
	}

	// Cached credentials were not available or not sufficient.
	// So, finally, we try to retrieve and validate Duplo credentials interactively.
	if client == nil {

		// Clear invalid cached credentials.
		if token == "" {
			cacheRemoveEntry(cacheKey, "duplo")
		}

		// Get the token, or fail.
		tokenResult := MustTokenInteractive(host, admin, "duplo-jit")
		if tokenResult.Token == "" {
			log.Fatalf("%s: authentication failure: failed to get token interactively", os.Args[0])
		}

		// Get the client, or fail.
		client, _ = duploClientAndOtpFlag(host, tokenResult.Token, tokenResult.OTP, admin)
		if client == nil {
			log.Fatalf("%s: authentication failure: failed to collect system features", os.Args[0])
		}

		// Build credentials.
		creds = &DuploCredsOutput{
			Version:    1,
			DuploToken: tokenResult.Token,
			NeedOTP:    tokenResult.OTP != "",
		}

		// Write the creds to the cache, unless we started out with a non-interactive token
		if token == "" {
			cacheFile := fmt.Sprintf("%s,duplo-creds.json", cacheKey)
			cacheWriteMustMarshal(cacheFile, creds)
		}
	}

	return
}

func OutputDuploCreds(creds *DuploCredsOutput) {

	// Convert the source to JSON
	json, err := json.Marshal(creds)
	DieIf(err, "cannot marshal to JSON")

	// Write the creds to the output.
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")
}

func PingDuploCreds(creds *DuploCredsOutput, host string) error {
	client, err := duplocloud.NewClient(host, creds.DuploToken)
	if err != nil {
		return err
	}

	tenants, terr := client.ListTenantsForUser()
	if terr != nil {
		return terr
	}

	if len(*tenants) == 0 {
		return errors.New("PingDuploCreds: user has no tenants")
	}

	tenant := (*tenants)[0]
	_, ferr := client.GetTenantFeatures(tenant.TenantID)
	if ferr != nil {
		return ferr
	}

	return nil
}
