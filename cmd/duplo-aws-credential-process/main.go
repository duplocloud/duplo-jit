package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"
	"github.com/duplocloud/duplo-aws-jit/internal"
)

func mustDuploClient(host, token string, interactive, admin bool) *duplocloud.Client {
	otp := ""

	// Possibly get a token from an interactive process.
	if token == "" {
		if !interactive {
			log.Fatalf("%s: --token not specified and --interactive mode is disabled", os.Args[0])
		}

		tokenResult := internal.MustTokenInteractive(host, admin, "duplo-aws-credential-process")
		token = tokenResult.Token
		otp = tokenResult.OTP
	}

	// Create the client.
	client, err := duplocloud.NewClientWithOtp(host, token, otp)
	internal.DieIf(err, "invalid arguments")

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
	duploOps := flag.Bool("duplo-ops", false, "Get Duplo operations credentials")
	tenantID := flag.String("tenant", "", "Get credentials for the given tenant")
	debug := flag.Bool("debug", false, "Turn on verbose (debugging) output")
	noCache := flag.Bool("no-cache", false, "Disable caching (not recommended)")
	interactive := flag.Bool("interactive", false, "Allow getting Duplo credentials via an interactive browser session")
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
	internal.MustInitCache("duplo-aws-credential-process", *noCache)

	// Get AWS credentials and output them
	var creds *internal.AwsConfigOutput
	var cacheKey string
	if *admin {

		// Build the cache key
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "admin"}, ",")

		// Try to find credentials from the cache.
		creds = internal.CacheGetAwsConfigOutput(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			client := mustDuploClient(*host, *token, *interactive, true)
			result, err := client.AdminGetJITAwsCredentials()
			internal.DieIf(err, "failed to get credentials")
			creds = internal.ConvertAwsCreds(result)
		}

	} else if *duploOps {

		// Build the cache key
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "duplo-ops"}, ",")

		// Try to find credentials from the cache.
		creds = internal.CacheGetAwsConfigOutput(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			client := mustDuploClient(*host, *token, *interactive, true)
			result, err := client.AdminAwsGetJitAccess("duplo-ops")
			internal.DieIf(err, "failed to get credentials")
			creds = internal.ConvertAwsCreds(result)
		}

	} else if tenantID == nil || *tenantID == "" {

		// Tenant credentials require an additional argument.
		internal.DieIf(errors.New("must specify --admin or --tenant=NAME or --tenant=ID"), "invalid arguments")

	} else {

		// Build the cache key.
		cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "tenant", *tenantID}, ",")

		// Try to find credentials from the cache.
		creds = internal.CacheGetAwsConfigOutput(cacheKey)

		// Otherwise, get the credentials from Duplo.
		if creds == nil {
			client := mustDuploClient(*host, *token, *interactive, false)

			// If it doesn't look like a UUID, get the tenant ID from the name.
			if len(*tenantID) < 32 {
				var err error
				tenant, err := client.GetTenantByNameForUser(*tenantID)
				if tenant == nil {
					err = errors.New("no such tenant available to your user")
				}
				internal.DieIf(err, fmt.Sprintf("%s: tenant not found", *tenantID))
				tenantID = &tenant.TenantID
			}

			// Tenant: Get the JIT AWS credentials
			result, err := client.TenantGetJITAwsCredentials(*tenantID)
			internal.DieIf(err, "failed to get credentials")
			creds = internal.ConvertAwsCreds(result)
		}
	}

	// Finally, we can output credentials.
	internal.OutputAwsCreds(creds, cacheKey)
}
