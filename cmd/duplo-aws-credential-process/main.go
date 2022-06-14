package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
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

	// Write the creds to the cache.
	cacheFile := fmt.Sprintf("%s,aws-creds.json", cacheKey)
	json := cacheWriteMustMarshal(cacheFile, creds)

	// Write the creds to the output.
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")
}

func mustDuploClient(host, token string, interactive, admin bool) *duplocloud.Client {
	otp := ""

	// Possibly get a token from an interactive process.
	if token == "" {
		if !interactive {
			log.Fatalf("%s: --token not specified and --interactive mode is disabled", os.Args[0])
		}

		tokenResult := mustTokenInteractive(host, admin)
		token = tokenResult.Token
		otp = ""
	}

	// Create the client.
	client, err := duplocloud.NewClientWithOtp(host, token, otp)
	dieIf(err, "invalid arguments")

	return client
}

var commit string
var version string

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
	interactive := flag.Bool("interactive", false, "Allow getting Duplo credentials via an interactive browser session (experimental)")
	showVersion := flag.Bool("version", false, "Output version information and exit")
	flag.Parse()

	// Output version information
	if *showVersion {
		if version == "" {
			version = "(dev build)"
		}
		if commit == "" {
			commit = "unknown"
		}
		fmt.Printf("%s version %s (git commit %s)\n", os.Args[0], version, commit)
		os.Exit(0)
	}

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

	// Prepare the cache directory
	mustInitCache()

	// Get AWS credentials and output them
	var creds *AwsConfigOutput
	var cacheKey string
	if *admin {

		// Build the cache key
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "admin"}, ",")

		// Try to find credentials from the cache.
		creds = cacheGetAwsConfigOutput(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			client := mustDuploClient(*host, *token, *interactive, true)
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
		creds = cacheGetAwsConfigOutput(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			client := mustDuploClient(*host, *token, *interactive, false)

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
