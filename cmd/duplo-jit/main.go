package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/duplocloud/duplo-jit/duplocloud"
	"github.com/duplocloud/duplo-jit/internal"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

var commit string
var version string

func main() {
	var admin *bool
	var duploOps *bool
	var tenantID *string
	var planID *string

	// Make sure we log to stderr - so we don't disturb the output to be collected by the AWS CLI
	log.SetOutput(os.Stderr)

	// Common command-line arguments.
	host := flag.String("host", "", "DuploCloud base URL")
	token := flag.String("token", "", "DuploCloud API token")
	debug := flag.Bool("debug", false, "Turn on verbose (debugging) output")
	noCache := flag.Bool("no-cache", false, "Disable caching (not recommended)")
	interactive := flag.Bool("interactive", false, "Allow getting Duplo credentials via an interactive browser session")
	port := flag.Int("port", 0, "Port to use for the local web server")
	showVersion := flag.Bool("version", false, "Output version information and exit")
	apiHost := flag.String("api-host", "", "Specify an alternate DuploCloud API base URL if it differs from the UI host (defaults to the value of --host if omitted)")
	admin = new(bool)
	duploOps = new(bool)

	// Parse the subcommand
	if len(os.Args) < 2 {
		fmt.Printf("%s: expected 'aws', 'duplo' or 'k8s' subcommands\n", os.Args[0])
		os.Exit(1)
	}
	cmd := os.Args[1]
	if cmd == "help" {
		fmt.Printf("%s: %s\n", os.Args[0], flag.ErrHelp.Error())
		os.Exit(0)
	} else if cmd == "version" || *showVersion {
		if version == "" {
			version = "(dev build)"
		}
		if commit == "" {
			commit = "unknown"
		}
		fmt.Printf("%s version %s (git commit %s)\n", os.Args[0], version, commit)
		os.Exit(0)
	} else if cmd != "aws" && cmd != "duplo" && cmd != "k8s" {
		fmt.Printf("%s: %s: subcommand not implemented\n", os.Args[0], cmd)
		os.Exit(1)
	} else {
		if cmd == "aws" {
			admin = flag.Bool("admin", false, "Get admin credentials")
			duploOps = flag.Bool("duplo-ops", false, "Get Duplo operations credentials")
		}
		if cmd == "k8s" {
			planID = flag.String("plan", "", "Get credentials for the given plan")
		}
		if cmd == "k8s" || cmd == "aws" {
			tenantID = flag.String("tenant", "", "Get credentials for the given tenant")
		}
	}

	// Parse command-line arguments.
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		fmt.Printf("%s: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}

	// Validate the host.
	if *host == "" {
		log.Fatalf("%s: %s", os.Args[0], "--host must be present")
	} else if strings.HasPrefix(*host, "http://localhost") {
		fmt.Fprintf(os.Stderr, "Using developer host %s\n", *host)
	} else if !strings.HasPrefix(*host, "https://") {
		// Refuse to call APIs over anything but https://
		log.Fatalf("%s: %s", os.Args[0], "--host must start with https://")
	}

	// Trim a trailing slash.
	*host = strings.TrimSuffix(*host, "/")

	// Validate the api-host if provided.
	if *apiHost != "" {
		if strings.HasPrefix(*apiHost, "http://localhost") {
			fmt.Printf("Using developer api-host %s\n", *apiHost)
		} else if !strings.HasPrefix(*apiHost, "https://") {
			// Refuse to call APIs over anything but https://
			log.Fatalf("%s: %s", os.Args[0], "--api-host must start with https://")
		}
		// Trim a trailing slash.
		*apiHost = strings.TrimSuffix(*apiHost, "/")
	} else {
		// By default, the api host is the same as the UI host.
		apiHost = host
	}

	// Possibly enable debugging
	if *debug {
		duplocloud.LogLevel = duplocloud.TRACE
	}

	// Prepare the cache directory
	internal.MustInitCache("duplo-jit", *noCache)

	// Get AWS credentials and output them
	cacheKey := internal.GetHostCacheKey(*host)

	switch cmd {
	case "aws":
		var creds *internal.AwsConfigOutput
		if *admin {

			// Build the cache key
			cacheKey = strings.Join([]string{cacheKey, "admin"}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetAwsConfigOutput(cacheKey)

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				client, _ := internal.MustDuploClient(*host, *apiHost, *token, *interactive, true, *port)
				result, err := client.AdminGetJitAwsCredentials()
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertAwsCreds(result)
			}

		} else if *duploOps {

			// Build the cache key
			cacheKey = strings.Join([]string{cacheKey, "duplo-ops"}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetAwsConfigOutput(cacheKey)

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				client, _ := internal.MustDuploClient(*host, *apiHost, *token, *interactive, true, *port)
				result, err := client.AdminAwsGetJitAccess("duplo-ops")
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertAwsCreds(result)
			}

		} else if tenantID == nil || *tenantID == "" {

			// Tenant credentials require an additional argument.
			internal.DieIf(errors.New("must specify --admin or --tenant=NAME or --tenant=ID"), "invalid arguments")

		} else {

			// Identify the tenant name to use for the cache key.
			var tenantName string
			client, _ := internal.MustDuploClient(*host, *apiHost, *token, *interactive, false, *port)
			*tenantID, tenantName = getTenantIDAndName(*tenantID, client)

			// Build the cache key.
			cacheKey = strings.Join([]string{cacheKey, "tenant", tenantName}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetAwsConfigOutput(cacheKey)

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				// Tenant: Get the JIT AWS credentials
				result, err := client.TenantGetJitAwsCredentials(*tenantID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertAwsCreds(result)
			}
		}

		// Finally, we can output credentials.
		internal.OutputAwsCreds(creds, cacheKey)

	case "duplo":
		_, creds := internal.MustDuploClient(*host, *apiHost, *token, *interactive, true, *port)
		internal.OutputDuploCreds(creds)

	case "k8s":
		var creds *clientauthv1beta1.ExecCredential
		if planID != nil && *planID != "" {

			// Build the cache key
			cacheKey = strings.Join([]string{cacheKey, "plan", *planID}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetK8sConfigOutput(cacheKey, "")

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				client, _ := internal.MustDuploClient(*host, *apiHost, *token, *interactive, true, *port)
				result, err := client.AdminGetK8sJitAccess(*planID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertK8sCreds(result)
			}

		} else if tenantID == nil || *tenantID == "" {

			// Tenant credentials require an additional argument.
			internal.DieIf(errors.New("must specify --plan=ID or --tenant=NAME or --tenant=ID"), "invalid arguments")

		} else {

			// Identify the tenant name to use for the cache key.
			var tenantName string
			client, _ := internal.MustDuploClient(*host, *apiHost, *token, *interactive, false, *port)
			*tenantID, tenantName = getTenantIDAndName(*tenantID, client)

			// Build the cache key.
			cacheKey = strings.Join([]string{cacheKey, "tenant", tenantName}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetK8sConfigOutput(cacheKey, tenantName)

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				// Tenant: Get the JIT AWS credentials
				result, err := client.TenantGetK8sJitAccess(*tenantID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertK8sCreds(result)
			}

		}

		// Finally, we can output credentials.
		internal.OutputK8sCreds(creds, cacheKey)

	}
}

func getTenantIDAndName(tenantIDorName string, client *duplocloud.Client) (string, string) {
	var tenantID string
	var tenantName string

	// If it doesn't look like a UUID, assume it is a name and get the tenant ID using its name.
	if len(tenantIDorName) < 32 {
		var err error
		tenantName = tenantIDorName
		tenant, err := client.GetTenantByNameForUser(tenantName)
		if tenant == nil || err != nil {
			internal.Fatal(fmt.Sprintf("tenant '%s' missing or not allowed", tenantName), err)
		} else {
			tenantID = tenant.TenantID
		}
	} else {
		// It looks like a UUID, assume it is one and get the tenant name using its ID.
		var err error
		tenantID = tenantIDorName
		tenant, err := client.GetTenantForUser(tenantIDorName)
		if tenant == nil || err != nil {
			internal.Fatal(fmt.Sprintf("tenant '%s' missing or not allowed", tenantID), err)
		} else {
			tenantName = tenant.AccountName
		}
	}

	return tenantID, tenantName
}
