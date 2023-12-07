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
	host := flag.String("host", "", "Duplo API base URL")
	token := flag.String("token", "", "Duplo API token")
	debug := flag.Bool("debug", false, "Turn on verbose (debugging) output")
	noCache := flag.Bool("no-cache", false, "Disable caching (not recommended)")
	interactive := flag.Bool("interactive", false, "Allow getting Duplo credentials via an interactive browser session")
	showVersion := flag.Bool("version", false, "Output version information and exit")
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
	internal.MustInitCache("duplo-jit", *noCache)

	// Get AWS credentials and output them
	switch cmd {
	case "aws":
		var creds *internal.AwsConfigOutput
		var cacheKey string
		if *admin {

			// Build the cache key
			cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "admin"}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetAwsConfigOutput(cacheKey)

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				client, _ := internal.MustDuploClient(*host, *token, *interactive, true)
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
				client, _ := internal.MustDuploClient(*host, *token, *interactive, true)
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
				client, _ := internal.MustDuploClient(*host, *token, *interactive, false)

				// If it doesn't look like a UUID, get the tenant ID from the name.
				if len(*tenantID) < 32 {
					var err error
					tenant, err := client.GetTenantByNameForUser(*tenantID)
					if tenant == nil || err != nil {
						internal.Fatal(fmt.Sprintf("%s: tenant missing or not allowed", *tenantID), err)
					} else {
						tenantID = &tenant.TenantID
					}
				}

				// Tenant: Get the JIT AWS credentials
				result, err := client.TenantGetJITAwsCredentials(*tenantID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertAwsCreds(result)
			}
		}

		// Finally, we can output credentials.
		internal.OutputAwsCreds(creds, cacheKey)

	case "duplo":
		_, creds := internal.MustDuploClient(*host, *token, *interactive, true)
		internal.OutputDuploCreds(creds)

	case "k8s":
		var creds *clientauthv1beta1.ExecCredential
		var cacheKey string
		if planID != nil && *planID != "" {

			// Build the cache key
			cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "plan", *planID}, ",")

			// Try to find credentials from the cache.
			creds = internal.CacheGetK8sConfigOutput(cacheKey, "")

			// Otherwise, get the credentials from Duplo.
			if creds == nil {
				client, _ := internal.MustDuploClient(*host, *token, *interactive, true)
				result, err := client.AdminGetK8sJitAccess(*planID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertK8sCreds(result)
			}

		} else if tenantID == nil || *tenantID == "" {

			// Tenant credentials require an additional argument.
			internal.DieIf(errors.New("must specify --plan=ID or --tenant=NAME or --tenant=ID"), "invalid arguments")

		} else {

			var tenantName string
			client, _ := internal.MustDuploClient(*host, *token, *interactive, false)

			// If it doesn't look like a UUID, get the tenant ID from the name.
			if len(*tenantID) < 32 {
				var err error
				tenant, err := client.GetTenantByNameForUser(*tenantID)
				if tenant == nil || err != nil {
					internal.Fatal(fmt.Sprintf("%s: tenant missing or not allowed", *tenantID), err)
				} else {
					tenantName = *tenantID
					tenantID = &tenant.TenantID
				}
			} else { // It looks like a UUID, get the tenant name from the ID.
				var err error
				tenant, err := client.GetTenantForUser(*tenantID)
				if tenant == nil || err != nil {
					internal.Fatal(fmt.Sprintf("%s: tenant missing or not allowed", *tenantID), err)
				} else {
					tenantName = tenant.AccountName
				}
			}

			// Build the cache key.
			cacheKey = strings.Join([]string{strings.TrimPrefix(*host, "https://"), "tenant", tenantName}, ",")

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
