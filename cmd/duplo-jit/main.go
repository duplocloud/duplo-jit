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
	var workspaceID *string
	var clusterID *string
	var scopeID *string
	var namespace *string
	var envID *string
	var rgID *string
	var namespaceID *string
	var readOnly *bool

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
		fmt.Printf("%s: expected 'aws', 'duplo', 'k8s', 'helpdesk-k8s' or 'clear-cache' subcommands\n", os.Args[0])
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
	} else if cmd == "clear-cache" {
		internal.ClearAllCaches()
		os.Exit(0)
	} else if cmd != "aws" && cmd != "duplo" && cmd != "k8s" && cmd != "helpdesk-k8s" {
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
		if cmd == "helpdesk-k8s" {
			workspaceID = flag.String("workspace-id", "", "Helpdesk workspace ID")
			clusterID = flag.String("cluster-id", "", "Cluster ID (mutually exclusive with --scope-id and --namespace-id)")
			scopeID = flag.String("scope-id", "", "Scope ID (mutually exclusive with --cluster-id and --namespace-id)")
			namespace = flag.String("namespace", "", "Namespace (only used with --scope-id)")
			envID = flag.String("env-id", "", "Environment ID (used with --namespace-id)")
			rgID = flag.String("rg-id", "", "Resource Group ID (used with --namespace-id)")
			namespaceID = flag.String("namespace-id", "", "Namespace ID (mutually exclusive with --cluster-id and --scope-id; requires --env-id and --rg-id)")
			readOnly = flag.Bool("read-only", false, "Request the namespace's read-only ServiceAccount (only used with --namespace-id)")
		}
	}

	// Parse command-line arguments.
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		fmt.Printf("%s: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}

	// Validate the host.
	const fatalFmt = "%s: %s"
	if *host == "" {
		log.Fatalf(fatalFmt, os.Args[0], "--host must be present")
	} else if strings.HasPrefix(*host, "http://localhost") {
		fmt.Fprintf(os.Stderr, "Using developer host %s\n", *host)
	} else if !strings.HasPrefix(*host, "https://") {
		// Refuse to call APIs over anything but https://
		log.Fatalf(fatalFmt, os.Args[0], "--host must start with https://")
	}

	// Trim a trailing slash.
	*host = strings.TrimSuffix(*host, "/")

	// Validate the api-host if provided.
	if *apiHost != "" {
		if strings.HasPrefix(*apiHost, "http://localhost") {
			fmt.Printf("Using developer api-host %s\n", *apiHost)
		} else if !strings.HasPrefix(*apiHost, "https://") {
			// Refuse to call APIs over anything but https://
			log.Fatalf(fatalFmt, os.Args[0], "--api-host must start with https://")
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
	conn := duploConn{host: *host, apiHost: *apiHost, token: *token, interactive: *interactive, port: *port}
	cacheKey := internal.GetHostCacheKey(*host)

	switch cmd {
	case "aws":
		var creds *internal.AwsConfigOutput
		if *admin {

			creds, cacheKey = getAdminAwsCreds(conn, cacheKey, "admin", *tenantID)

		} else if *duploOps {

			creds, cacheKey = getAdminAwsCreds(conn, cacheKey, "duplo-ops", *tenantID)

		} else if tenantID == nil || *tenantID == "" {

			// Tenant credentials require an additional argument.
			internal.DieIf(errors.New("must specify --admin or --tenant=NAME or --tenant=ID"), "invalid arguments")

		} else {

			// Identify the tenant name to use for the cache key.
			var tenantName string
			client := conn.client(false)
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
				client := conn.client(true)
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
			client := conn.client(false)
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

	case "helpdesk-k8s":
		// Distinct from "k8s" above: this mode talks to the new duplo-ai-helpdesk backend
		// (workspace/cluster/scope identifiers), not the legacy Duplo platform (tenant/plan).
		// It never logs in interactively — the bearer token is handed to it ready-made, via
		// DUPLO_JIT_TOKEN, by the kubeconfig's exec.env (see KubeConfigFile.fromK8ClusterConfig
		// in duplo-ui), never a CLI flag, so it never shows up in `ps`.
		if workspaceID == nil || *workspaceID == "" {
			internal.DieIf(errors.New("must specify --workspace-id"), "invalid arguments")
		}
		hasCluster := clusterID != nil && *clusterID != ""
		hasScope := scopeID != nil && *scopeID != ""
		hasNamespace := namespaceID != nil && *namespaceID != ""
		switch {
		case !hasCluster && !hasScope && !hasNamespace:
			internal.DieIf(errors.New("must specify --cluster-id, --scope-id, or --namespace-id"), "invalid arguments")
		case (hasCluster && hasScope) || (hasCluster && hasNamespace) || (hasScope && hasNamespace):
			internal.DieIf(errors.New("--cluster-id, --scope-id, and --namespace-id are mutually exclusive"), "invalid arguments")
		case hasNamespace && (envID == nil || *envID == "" || rgID == nil || *rgID == ""):
			internal.DieIf(errors.New("--namespace-id requires --env-id and --rg-id"), "invalid arguments")
		}

		jitToken := os.Getenv("DUPLO_JIT_TOKEN")
		if jitToken == "" {
			internal.DieIf(errors.New("DUPLO_JIT_TOKEN environment variable is required"), "invalid arguments")
		}

		client, clientErr := duplocloud.NewClient(*apiHost, jitToken)
		internal.DieIf(clientErr, "failed to create API client")

		var creds *clientauthv1beta1.ExecCredential
		switch {
		case hasCluster:
			cacheKey = strings.Join([]string{cacheKey, "helpdesk-cluster", *workspaceID, *clusterID}, ",")
			creds = internal.CacheGetHelpdeskK8sConfigOutput(cacheKey)

			if creds == nil {
				result, err := client.HelpdeskGetClusterJitAccess(*workspaceID, *clusterID)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertHelpdeskK8sCreds(result)
			}
		case hasScope:
			cacheKey = strings.Join([]string{cacheKey, "helpdesk-scope", *workspaceID, *scopeID, *namespace}, ",")
			creds = internal.CacheGetHelpdeskK8sConfigOutput(cacheKey)

			if creds == nil {
				result, err := client.HelpdeskGetScopeJitAccess(*workspaceID, *scopeID, *namespace)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertHelpdeskK8sCreds(result)
			}
		default:
			cacheKey = strings.Join([]string{cacheKey, "helpdesk-namespace", *workspaceID, *envID, *rgID, *namespaceID, fmt.Sprintf("%t", *readOnly)}, ",")
			creds = internal.CacheGetHelpdeskK8sConfigOutput(cacheKey)

			if creds == nil {
				result, err := client.HelpdeskGetNamespaceJitAccess(*workspaceID, *envID, *rgID, *namespaceID, *readOnly)
				internal.DieIf(err, "failed to get credentials")
				creds = internal.ConvertHelpdeskK8sCreds(result)
			}
		}

		// Finally, we can output credentials.
		internal.OutputK8sCreds(creds, cacheKey)

	}
}

// duploConn carries the connection settings every Duplo API path needs, so helpers
// can take them as one argument instead of five.
type duploConn struct {
	host, apiHost, token string
	interactive          bool
	port                 int
}

// client builds an authenticated Duplo API client (interactive login if allowed).
func (c duploConn) client(admin bool) *duplocloud.Client {
	client, _ := internal.MustDuploClient(c.host, c.apiHost, c.token, c.interactive, admin, c.port)
	return client
}

// getAdminAwsCreds retrieves JIT AWS credentials for an admin role ("admin" or
// "duplo-ops"). When a tenant is supplied, the credentials' region and console URL
// are overridden to point at the tenant's region (the admin JIT API otherwise
// returns the master account's default region), and the tenant name is folded into
// the cache key so credentials are cached per-tenant. It returns the credentials
// and the cache key they live under — empty when they must not be cached.
func getAdminAwsCreds(conn duploConn, hostCacheKey, role, tenantIDorName string) (*internal.AwsConfigOutput, string) {
	cacheKey := strings.Join([]string{hostCacheKey, role}, ",")

	var client *duplocloud.Client
	var tenantID, tenantName string

	// If a tenant is selected, scope the cache to it. Resolving the tenant name needs
	// a client, so this runs ahead of the cache read, as the plain tenant path does.
	if tenantIDorName != "" {
		client = conn.client(true)
		tenantID, tenantName = getTenantIDAndName(tenantIDorName, client)
		cacheKey = strings.Join([]string{cacheKey, "tenant", tenantName}, ",")
	}

	// Try to find credentials from the cache.
	creds := internal.CacheGetAwsConfigOutput(cacheKey)

	// Otherwise, get the credentials from Duplo.
	if creds == nil {
		if client == nil {
			client = conn.client(true)
		}
		result, err := client.AdminAwsGetJitAccess(role)
		internal.DieIf(err, "failed to get credentials")
		creds = internal.ConvertAwsCreds(result)

		// Open the console in the tenant's region rather than the master default. The
		// region is baked into the cached credentials, so it is only looked up on a miss.
		if tenantID != "" && !applyTenantRegion(client, creds, tenantID, tenantName) {
			// The credentials are valid regardless of region, so a failed region lookup
			// must not fail a credential_process invocation after the JIT credentials
			// were already minted. Fall back to the master default — but don't cache the
			// degraded result, so the next invocation retries the lookup rather than
			// serving a wrong console region silently for the whole cache lifetime.
			cacheKey = ""
		}
	}

	return creds, cacheKey
}

// applyTenantRegion looks up the tenant's region and points the credentials at it.
// Returns false — after warning on stderr — when the lookup failed and the master
// default region was left in place. A tenant with no region configured, or a console
// URL that could not be rewritten, is also warned about but counts as handled.
func applyTenantRegion(client *duplocloud.Client, creds *internal.AwsConfigOutput, tenantID, tenantName string) bool {
	features, err := client.GetTenantFeatures(tenantID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not look up the region for tenant %q (%s); using the master account default region\n", tenantName, err)
		return false
	}
	if features.Region == "" {
		fmt.Fprintf(os.Stderr, "warning: tenant %q has no region configured; using the master account default region\n", tenantName)
	} else if !internal.ApplyTenantRegion(creds, features.Region) {
		fmt.Fprintf(os.Stderr, "warning: could not rewrite the console URL for region %s; the console may open in a different region\n", features.Region)
	}
	return true
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
