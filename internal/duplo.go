package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"
)

type DuploCredsOutput struct {
	Version    int    `json:"Version"`
	DuploToken string `json:"DuploToken,omitempty"`
	NeedOTP    bool   `json:"NeedOTP"`
}

func MustDuploClient(host, token string, interactive, admin bool) (client *duplocloud.Client, creds *DuploCredsOutput) {
	var err error
	otp := ""

	// Possibly get a token from an interactive process.
	if token == "" {
		if !interactive {
			log.Fatalf("%s: --token not specified and --interactive mode is disabled", os.Args[0])
		}

		// Try to find credentials from the cache.
		cacheKey := strings.TrimPrefix(host, "https://")
		creds = CacheGetDuploOutput(cacheKey, host)

		// If we have valid credentials, and we do not need OTP, return the new client.
		if creds != nil && (!admin || !creds.NeedOTP) {
			client, err = duplocloud.NewClient(host, creds.DuploToken)
			DieIf(err, "invalid arguments")

			// Otherwise, get the token the interactive way.
		} else {
			tokenResult := MustTokenInteractive(host, admin, "duplo-jit")
			token = tokenResult.Token
			otp = tokenResult.OTP

			// Write the creds to the cache.
			cacheFile := fmt.Sprintf("%s,duplo-creds.json", cacheKey)
			creds = &DuploCredsOutput{
				Version:    1,
				DuploToken: token,
				NeedOTP:    otp != "",
			}
			cacheWriteMustMarshal(cacheFile, creds)
		}
	}

	// Create the client.
	if client == nil {
		client, err = duplocloud.NewClientWithOtp(host, token, otp)
		DieIf(err, "invalid arguments")
	}

	// Ensure we have a representation off credentials
	if creds == nil {
		creds = &DuploCredsOutput{
			Version:    1,
			DuploToken: token,
			NeedOTP:    client.OTP != "",
		}

		// Ensure we are aware of any OTP requirements
		if admin && !creds.NeedOTP {
			features, err := client.FeaturesSystem()
			DieIf(err, "failed to collect system features from Duplo")
			creds.NeedOTP = features.IsOtpNeeded
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
