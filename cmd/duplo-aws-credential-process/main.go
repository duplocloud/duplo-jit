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

func outputCreds(creds *duplocloud.AwsJitCredentials, cacheKey string) {
	// Calculate the expiration time.
	now := time.Now().UTC()
	validity := creds.Validity
	if validity <= 0 {
		validity = 3600 // default is one hour
	}
	expiration := now.Add(time.Duration(validity) * time.Second)

	// Build the resulting credentials to be output.
	result := AwsConfigOutput{
		Version:         1,
		ConsoleUrl:      creds.ConsoleURL,
		AccessKeyId:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expiration:      expiration.Format(time.RFC3339),
	}

	// Convert the credentials to JSON
	json, err := json.Marshal(result)
	dieIf(err, "cannot marshal credentials to JSON")

	// Write them out
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")

	// Cache them as well
}

var cacheDir string
var cacheKey string

func main() {

	// Make sure we log to stderr - so we don't disturb the output to be collected by the AWS CLI
	log.SetOutput(os.Stderr)

	// Parse command-line arguments.
	host := flag.String("host", "", "Duplo API base URL")
	token := flag.String("token", "", "Duplo API token")
	admin := flag.Bool("admin", false, "Get admin credentials")
	tenantID := flag.String("tenant", "", "Get credentials for the given tenant")
	debug := flag.Bool("debug", false, "Turn on verbose (debugging) output")
	flag.Parse()

	// Refuse to call APIs over anything but https://
	if host == nil || !strings.HasPrefix(*host, "https://") {
		log.Fatalf("%s: %s", os.Args[0], "--host must be present and start with https://")
	}

	// Possibly enable debugging
	if *debug {
		duplocloud.LogLevel = duplocloud.TRACE
	}

	// Prepare the connection to the duplo API.
	client, err := duplocloud.NewClient(*host, *token)
	dieIf(err, "invalid arguments")

	// Prepare the cache directory
	cacheDir, err = os.UserCacheDir()
	dieIf(err, "cannot find cache directory")
	cacheDir = filepath.Join(cacheDir, "duplo-aws-credential-process")

	// Gather credentials
	var creds *duplocloud.AwsJitCredentials
	if *admin {

		// Build the cache key
		cacheKey := strings.Join([]string{strings.TrimPrefix(*host, "https://"), "admin"}, ",")

		// Admin: Get the JIT AWS credentials
		creds, err = client.AdminGetJITAwsCredentials()
		dieIf(err, "failed to get credentials")
		outputCreds(creds, cacheKey)

	} else if tenantID == nil || *tenantID == "" {

		// Tenant credentials require an additional argument.
		dieIf(errors.New("must specify --admin or --tenant=NAME or --tenant=ID"), "invalid arguments")

	} else {

		// Build the cache key.
		cacheKey := strings.Join([]string{strings.TrimPrefix(*host, "https://"), "tenant", *tenantID}, ",")

		// If it doesn't look like a UUID, get the tenant ID from the name.
		if len(*tenantID) < 32 {
			tenant, err := client.GetTenantByNameForUser(*tenantID)
			dieIf(err, fmt.Sprintf("%s: tenant not found", *tenantID))
			tenantID = &tenant.TenantID
		}

		// Tenant: Get the JIT AWS credentials
		creds, err = client.TenantGetJITAwsCredentials(*tenantID)
		dieIf(err, "failed to get credentials")
		outputCreds(creds, cacheKey)
	}
}
