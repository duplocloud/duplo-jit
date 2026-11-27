package internal

import (
	"fmt"
	"log"
	"os"

	"github.com/duplocloud/duplo-jit/duplocloud"
)

// HelpdeskAuthOutput is the cached form of an interactively-acquired duplo-ai-helpdesk bearer
// token — mirrors DuploCredsOutput's role for the legacy platform, but deliberately has no
// Version/NeedOTP fields: the helpdesk interactive flow has no OTP step.
type HelpdeskAuthOutput struct {
	Token string `json:"Token"`
}

// MustHelpdeskClient resolves an API client authenticated against duplo-ai-helpdesk for the
// helpdesk-k8s mode, trying in order:
//  1. An explicitly-supplied --token flag.
//  2. The DUPLO_JIT_TOKEN environment variable — kept so the embedded-JWT kubeconfig shape
//     (KubeConfigFile.fromK8ClusterConfig in duplo-ui) keeps working unchanged: that bearer
//     token is deliberately an env var, never a flag, so it never shows up in `ps`.
//  3. A token cached from a previous interactive login on this host.
//  4. An interactive browser-based login — only when allowed — reusing the exact same
//     verify-token page and TokenViaListener machinery the legacy "duplo k8s --interactive" mode
//     uses, unmodified. This works because duplo-ai-helpdesk and the legacy portal share one
//     login session: the token that page hands back (UserSession.bearerToken) is the same one
//     BearerTokenInterceptor already attaches to every duplo-ai-helpdesk API call in duplo-ui.
//
// Unlike MustDuploClient, there is no cheap endpoint to eagerly validate a resolved token the way
// FeaturesSystem() does for the legacy platform — validity is instead checked lazily by the
// caller: on a 401 from the real jitAccess call, call InvalidateHelpdeskToken and call this
// function again (see callHelpdeskWithRetry in cmd/duplo-jit).
func MustHelpdeskClient(host, apiHost, token string, interactive bool, port int) *duplocloud.Client {
	if token != "" {
		return mustHelpdeskClientFor(apiHost, token)
	}

	if envToken := os.Getenv("DUPLO_JIT_TOKEN"); envToken != "" {
		return mustHelpdeskClientFor(apiHost, envToken)
	}

	if cached := CacheGetHelpdeskTokenUnchecked(host); cached != "" {
		return mustHelpdeskClientFor(apiHost, cached)
	}

	if !interactive {
		log.Fatalf("%s: no token available for helpdesk-k8s (set --token, DUPLO_JIT_TOKEN, or pass --interactive)", os.Args[0])
	}

	tokenResult := MustTokenInteractive(host, false, "helpdesk-k8s", port)
	if tokenResult.Token == "" {
		log.Fatalf("%s: authentication failure: failed to get token interactively", os.Args[0])
	}

	cacheWriteMustMarshal(helpdeskAuthCacheFile(host), &HelpdeskAuthOutput{Token: tokenResult.Token})
	return mustHelpdeskClientFor(apiHost, tokenResult.Token)
}

func mustHelpdeskClientFor(apiHost, token string) *duplocloud.Client {
	client, err := duplocloud.NewClient(apiHost, token)
	DieIf(err, "failed to create API client")
	return client
}

// CacheGetHelpdeskTokenUnchecked reads a cached interactive helpdesk token without validating it
// against the API — mirrors CacheGetDuploTokenUnchecked's role for the legacy platform.
func CacheGetHelpdeskTokenUnchecked(host string) string {
	if noCache || cacheDir == "" {
		return ""
	}
	creds := &HelpdeskAuthOutput{}
	if !cacheReadUnmarshal(helpdeskAuthCacheFile(host), creds) {
		return ""
	}
	return creds.Token
}

// InvalidateHelpdeskToken clears the cached interactive helpdesk token, so the next
// MustHelpdeskClient call re-authenticates instead of reusing one the backend just rejected.
func InvalidateHelpdeskToken(host string) {
	cacheKey := GetHostCacheKey(host)
	cacheRemoveFile(cacheKey, helpdeskAuthCacheFile(host))
}

func helpdeskAuthCacheFile(host string) string {
	return fmt.Sprintf("%s,helpdesk-auth-creds.json", GetHostCacheKey(host))
}
