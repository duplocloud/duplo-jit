package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/duplocloud/duplo-jit/duplocloud"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	status.ExpirationTimestamp = &metav1.Time{Time: expiration}

	return &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
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
	_, _ = os.Stdout.Write(json)
	_, _ = os.Stdout.WriteString("\n")
}

func PingK8sCreds(creds *clientauthv1beta1.ExecCredential, tenantName string) error {
	config := &rest.Config{
		Host: creds.Spec.Cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		BearerToken: creds.Status.Token,
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	namespace := tenantName
	if namespace == "" {
		namespace = "kube-system"
	} else {
		namespace = fmt.Sprintf("duploservices-%s", namespace)
	}

	_, err = clientset.CoreV1().ServiceAccounts(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return nil
}

// ConvertHelpdeskK8sCreds converts a HelpdeskK8sClusterConfig (the new duplo-ai-helpdesk
// backend's response shape) into the standard kubectl ExecCredential format. Parallel to
// ConvertK8sCreds above, which does the same for the legacy platform's DuploPlanK8ClusterConfig.
func ConvertHelpdeskK8sCreds(creds *duplocloud.HelpdeskK8sClusterConfig) *clientauthv1beta1.ExecCredential {
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

	// Populate expiration time. Unlike ConvertK8sCreds above (which defaults to a 55-minute
	// guess when the legacy platform doesn't supply one), an absent TokenExpiresAtUtc here
	// specifically means the backend could not determine a safe expiry for this credential
	// (e.g. AKS) — refresh soon rather than assume a long remaining lifetime.
	var expiration time.Time
	if creds.TokenExpiresAtUtc != nil {
		expiration = *creds.TokenExpiresAtUtc
	} else {
		expiration = time.Now().Add(5 * time.Minute)
	}
	status.ExpirationTimestamp = &metav1.Time{Time: expiration}

	return &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: "client.authentication.k8s.io/v1beta1",
		},
		Spec: clientauthv1beta1.ExecCredentialSpec{
			Cluster: &cluster,
		},
		Status: &status,
	}
}

// PingHelpdeskK8sCreds validates cached helpdesk-mode credentials are still good. Unlike
// PingK8sCreds above, it does not reconstruct a "duploservices-{tenantName}" namespace — that
// convention belongs to the legacy platform's shared-cluster-per-tenant model and has no
// meaning here: a helpdesk-mode token is already scoped to exactly one cluster/scope by the
// backend that minted it, so any reachable namespace is enough to confirm the token still works.
func PingHelpdeskK8sCreds(creds *clientauthv1beta1.ExecCredential) error {
	config := &rest.Config{
		Host: creds.Spec.Cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		BearerToken: creds.Status.Token,
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().ServiceAccounts("default").List(context.TODO(), metav1.ListOptions{})
	return err
}
