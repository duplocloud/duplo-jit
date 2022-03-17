package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"
)

type AwsConfigOutput struct {
	Version         int    `json:"Version"`
	ConsoleUrl      string `json:"ConsoleUrl"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken,omitempty"`
	Expiration      string `json:"Expiration,omitempty"`
}

func dieIf(err error, msg string) {
	if err != nil {
		fatal(msg, err)
	}
}

func fatal(msg string, err error) {
	log.Fatalf("%s: %s: %s", os.Args[0], msg, err)
}

func convertCreds(creds *duplocloud.AwsJitCredentials) *AwsConfigOutput {
	// Calculate the expiration time.
	now := time.Now().UTC()
	validity := creds.Validity
	if validity <= 0 {
		validity = 3600 // default is one hour
	}
	expiration := now.Add(time.Duration(validity) * time.Second)

	// Build the resulting credentials to be output.
	return &AwsConfigOutput{
		Version:         1,
		ConsoleUrl:      creds.ConsoleURL,
		AccessKeyId:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expiration:      expiration.Format(time.RFC3339),
	}
}

func outputCreds(creds *AwsConfigOutput, cacheKey string) {

	// Convert the credentials to JSON
	json, err := json.Marshal(creds)
	dieIf(err, "cannot marshal credentials to JSON")

	// Write them out
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")

	// Cache them as well.
	if (noCache == nil || !*noCache) && cacheDir != "" && cacheKey != "" {
		credsCache := filepath.Join(cacheDir, fmt.Sprintf("%s,aws-creds.json", cacheKey))

		err = os.WriteFile(credsCache, json, 0600)
		if err != nil {
			log.Printf("warning: %s: unable to write to credentials cache", cacheKey)
		}
	}
}

func getCachedCredentials(cacheKey string) (creds *AwsConfigOutput) {
	var cacheFile string

	// Read credentials from the cache.
	if (noCache == nil || !*noCache) && cacheDir != "" && cacheKey != "" {
		cacheFile = filepath.Join(cacheDir, fmt.Sprintf("%s,aws-creds.json", cacheKey))

		bytes, err := os.ReadFile(cacheFile)
		if err == nil {
			creds = &AwsConfigOutput{}
			err = json.Unmarshal(bytes, creds)
			if err != nil {
				log.Printf("warning: %s: invalid JSON in credentials cache: %s", cacheKey, err)
				creds = nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: %s: unable to read from credentials cache", cacheKey)
		}
	}

	// Check credentials for expiry.
	if creds != nil {
		five_minutes_from_now := time.Now().UTC().Add(5 * time.Minute)
		expiration, err := time.Parse(time.RFC3339, creds.Expiration)

		// Invalid expiration?
		if err != nil {
			log.Printf("warning: %s: invalid Expiration time in credentials cache: %s", cacheKey, creds.Expiration)
			creds = nil

			// Expires in five minutes or less?
		} else if five_minutes_from_now.After(expiration) {
			creds = nil
		}

		// Clear the cache if the creds expired.
		if creds == nil {
			err = os.Remove(cacheFile)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("warning: %s: unable to remove from credentials cache", cacheKey)
			}
		}
	}

	return
}

var cacheDir string
var noCache *bool

func main() {

	// Make sure we log to stderr - so we don't disturb the output to be collected by the AWS CLI
	log.SetOutput(os.Stderr)

	// Parse command-line arguments.
	host := flag.String("host", "", "Duplo API base URL")
	token := flag.String("token", "", "Duplo API token")
	admin := flag.Bool("admin", false, "Get admin credentials")
	tenantID := flag.String("tenant", "", "Get credentials for the given tenant")
	debug := flag.Bool("debug", false, "Turn on verbose (debugging) output")
	noCache = flag.Bool("no-cache", false, "Disable caching (not recommended)")
	flag.Parse()

	// Refuse to call APIs over anything but https://
	// Trim a trailing slash.
	if host == nil || !strings.HasPrefix(*host, "https://") {
		log.Fatalf("%s: %s", os.Args[0], "--host must be present and start with https://")
	}
	*host = strings.TrimSuffix(*host, "/")

	// Possibly enable debugging
	if *debug {
		duplocloud.LogLevel = duplocloud.TRACE
	}

	// Prepare the connection to the duplo API.
	client, err := duplocloud.NewClient(*host, *token)
	dieIf(err, "invalid arguments")

	// Prepare the cache directory
	if noCache == nil || !*noCache {
		cacheDir, err = os.UserCacheDir()
		dieIf(err, "cannot find cache directory")
		cacheDir = filepath.Join(cacheDir, "duplo-aws-credential-process")
		err = os.MkdirAll(cacheDir, 0700)
		dieIf(err, "cannot create cache directory")
	}

	// Gather credentials
	var creds *AwsConfigOutput
	var cacheKey string
	if *admin {

		// Build the cache key
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "admin"}, ",")

		// Try to find credentials from the cache.
		creds = getCachedCredentials(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			result, err := client.AdminGetJITAwsCredentials()
			dieIf(err, "failed to get credentials")
			creds = convertCreds(result)
		}

	} else if tenantID == nil || *tenantID == "" {

		// Tenant credentials require an additional argument.
		dieIf(errors.New("must specify --admin or --tenant=NAME or --tenant=ID"), "invalid arguments")

	} else {

		// Build the cache key.
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "tenant", *tenantID}, ",")

		// Try to find credentials from the cache.
		creds = getCachedCredentials(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {

			// If it doesn't look like a UUID, get the tenant ID from the name.
			if len(*tenantID) < 32 {
				tenant, err := client.GetTenantByNameForUser(*tenantID)
				dieIf(err, fmt.Sprintf("%s: tenant not found", *tenantID))
				tenantID = &tenant.TenantID
			}

			// Tenant: Get the JIT AWS credentials
			result, err := client.TenantGetJITAwsCredentials(*tenantID)
			dieIf(err, "failed to get credentials")
			creds = convertCreds(result)
		}
	}

	// Finally, we can output credentials.
	outputCreds(creds, cacheKey)
}
