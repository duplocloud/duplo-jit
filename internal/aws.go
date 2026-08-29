package internal

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/duplocloud/duplo-jit/duplocloud"
)

type AwsConfigOutput struct {
	Version         int    `json:"Version"`
	ConsoleUrl      string `json:"ConsoleUrl"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	Region          string `json:"Region"`
	SessionToken    string `json:"SessionToken,omitempty"`
	Expiration      string `json:"Expiration,omitempty"`
}

func ConvertAwsCreds(creds *duplocloud.AwsJitCredentials) *AwsConfigOutput {
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
		Region:          creds.Region,
		SessionToken:    creds.SessionToken,
		Expiration:      expiration.Format(time.RFC3339),
	}
}

// OverrideConsoleRegion returns the AWS federation console URL with the region of
// its Destination target replaced, and whether a rewrite happened. Admin / duplo-ops
// JIT URLs open in the master account's default region; when a tenant is selected we
// want the console to open in that tenant's region instead.
//
// The URL comes back unchanged (false) when it is empty, cannot be fully parsed, has
// no Destination, or the Destination carries no region query param (for example a
// region encoded in the host). Only the Destination query is rewritten; a region
// inside its fragment is carried through as-is. The SigninToken value is preserved
// (re-encoded equivalently).
func OverrideConsoleRegion(consoleUrl, region string) (string, bool) {
	if consoleUrl == "" || region == "" {
		return consoleUrl, false
	}
	u, err := url.Parse(consoleUrl)
	if err != nil {
		return consoleUrl, false
	}
	// u.Query() silently drops malformed pairs, and re-encoding would then lose them
	// (SigninToken included), so never re-serialize a query that did not fully parse.
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return consoleUrl, false
	}
	dest := q.Get("Destination")
	if dest == "" {
		return consoleUrl, false
	}
	d, err := url.Parse(dest)
	if err != nil {
		return consoleUrl, false
	}
	dq, err := url.ParseQuery(d.RawQuery)
	if err != nil {
		return consoleUrl, false
	}
	if dq.Get("region") == "" {
		return consoleUrl, false
	}
	dq.Set("region", region)
	d.RawQuery = dq.Encode()
	q.Set("Destination", d.String())
	u.RawQuery = q.Encode()
	return u.String(), true
}

// ApplyTenantRegion overrides the region reported by admin / duplo-ops JIT
// credentials so that both the Region field and the console URL point at the
// selected tenant's region. It returns false when nothing was applied, or when the
// credentials carry a console URL that could not be rewritten and so no longer
// agrees with Region.
func ApplyTenantRegion(creds *AwsConfigOutput, region string) bool {
	if creds == nil || region == "" {
		return false
	}
	creds.Region = region
	if creds.ConsoleUrl == "" {
		return true
	}
	var ok bool
	creds.ConsoleUrl, ok = OverrideConsoleRegion(creds.ConsoleUrl, region)
	return ok
}

// OutputAwsCreds writes the credentials to the cache under cacheKey and to stdout.
// An empty cacheKey skips the cache write: the caller produced credentials it does
// not want served again (e.g. a tenant-region lookup failed and the master default
// region was left in place).
func OutputAwsCreds(creds *AwsConfigOutput, cacheKey string) {
	var json []byte
	if cacheKey == "" {
		json = mustMarshal(creds)
	} else {
		// Write the creds to the cache.
		cacheFile := fmt.Sprintf("%s,aws-creds.json", cacheKey)
		json = cacheWriteMustMarshal(cacheFile, creds)
	}

	// Write the creds to the output.
	_, _ = os.Stdout.Write(json)
	_, _ = os.Stdout.WriteString("\n")
}

func PingAWSCreds(creds *AwsConfigOutput) error {
	credsProvider := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken))

	// Create an AWS config using the creds.
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return err
	}

	// Create an STS client with the AWS config.
	stsClient := sts.NewFromConfig(cfg)

	// Call the STS client API for get-caller-identity to test cred validity.
	_, err = stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}

	return nil
}
