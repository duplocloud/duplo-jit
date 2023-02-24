package internal

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

func ConvertK8sCreds(creds *duplocloud.DuploPlanK8ClusterConfig) *clientauthv1beta1.ExecCredential {
	// Populate cluster info.
	cluster := clientauthv1beta1.Cluster{Server: creds.ApiServer}

	// Populate CA certificate data.
	if creds.CertificateAuthorityDataBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(creds.CertificateAuthorityDataBase64)
		DieIf(err, "failed to base64 decode CA certificate data")
		cluster.CertificateAuthorityData = data
	} else {
		cluster.InsecureSkipTLSVerify = true
	}

	// Populate token.
	status := clientauthv1beta1.ExecCredentialStatus{Token: creds.Token}

	// Populate expiration time.
	var expiration time.Time
	if creds.LastTokenRefreshTime != nil {
		expiration = *creds.LastTokenRefreshTime

		// Default expiration time: 55 minutes
	} else {
		expiration = time.Now().Add(time.Duration(60*55) * time.Second)
	}
	status.ExpirationTimestamp = &v1.Time{Time: expiration}

	return &clientauthv1beta1.ExecCredential{
		TypeMeta: v1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Spec: clientauthv1beta1.ExecCredentialSpec{
			Cluster: &cluster,
		},
		Status: &status,
	}
}

func OutputK8sCreds(creds *clientauthv1beta1.ExecCredential, cacheKey string) {

	// Write the creds to the cache.
	cacheFile := fmt.Sprintf("%s,k8s-creds.json", cacheKey)
	json := cacheWriteMustMarshal(cacheFile, creds)

	// Write the creds to the output.
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")
}
