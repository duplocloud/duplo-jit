package duplocloud

import (
	"fmt"
	"net/url"
	"time"
)

// DuploSystemFeatures represents configured features in the system
type DuploSystemFeatures struct {
	IsKatkitEnabled      bool     `json:"IsKatkitEnabled"`
	IsSignupEnabled      bool     `json:"IsSignupEnabled"`
	IsComplianceEnabled  bool     `json:"IsComplianceEnabled"`
	IsBillingEnabled     bool     `json:"IsBillingEnabled"`
	IsSiemEnabled        bool     `json:"IsSiemEnabled"`
	IsAwsCloudEnabled    bool     `json:"IsAwsCloudEnabled"`
	AwsRegions           []string `json:"AwsRegions"`
	DefaultAwsAccount    string   `json:"DefaultAwsAccount"`
	DefaultAwsRegion     string   `json:"DefaultAwsRegion"`
	IsAzureCloudEnabled  bool     `json:"IsAzureCloudEnabled"`
	AzureRegions         []string `json:"AzureRegions"`
	IsGoogleCloudEnabled bool     `json:"IsGoogleCloudEnabled"`
	EksVersions          struct {
		DefaultVersion    string   `json:"DefaultVersion"`
		SupportedVersions []string `json:"SupportedVersions"`
	} `json:"EksVersions"`
	IsOtpNeeded           bool   `json:"IsOtpNeeded"`
	IsAwsAdminJITEnabled  bool   `json:"IsAwsAdminJITEnabled"`
	IsDuploOpsEnabled     bool   `json:"IsDuploOpsEnabled"`
	DevopsManagerHostname string `json:"DevopsManagerHostname"`
	TenantNameMaxLength   int    `json:"TenantNameMaxLength"`
}

// DuploTenantFeatures represents configured features for the tenant
type DuploTenantFeatures struct {
	Region              string `json:"Region,omitempty"`
	IsKubernetesEnabled bool   `json:"IsKubernetesEnabled"`
	UseLbIndex          bool   `json:"UseLbIndex"`
}

// DuploPlanK8ClusterConfig represents a k8s system configuration
type DuploPlanK8ClusterConfig struct {
	Name                           string     `json:"Name,omitempty"`
	ApiServer                      string     `json:"ApiServer,omitempty"`
	Token                          string     `json:"Token,omitempty"`
	K8Provider                     int        `json:"K8Provider,omitempty"`
	AwsRegion                      string     `json:"AwsRegion,omitempty"`
	K8sVersion                     string     `json:"K8sVersion,omitempty"`
	CertificateAuthorityDataBase64 string     `json:"CertificateAuthorityDataBase64,omitempty"`
	LastTokenRefreshTime           *time.Time `json:"LastTokenRefreshTime,omitempty"`
}

// AwsJitCredentials represents just-in-time AWS credentials from Duplo
type AwsJitCredentials struct {
	ConsoleURL      string `json:"ConsoleUrl,omitempty"`
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	Region          string `json:"Region"`
	SessionToken    string `json:"SessionToken,omitempty"`
	Validity        int    `json:"Validity,omitempty"`
}

// UserTenant represents a user's view of a Duplo tenant.
type UserTenant struct {
	TenantID    string `json:"TenantId,omitempty"`
	AccountName string `json:"AccountName"`
	PlanID      string `json:"PlanID"`
}

// FeaturesSystem retrieves the configured system features.
func (c *Client) FeaturesSystem() (*DuploSystemFeatures, ClientError) {
	features := DuploSystemFeatures{}
	err := c.getAPI("FeaturesSystem()", "v3/features/system", &features)
	if err != nil {
		return nil, err
	}
	return &features, nil
}

// AdminAwsGetJitAccess retrieves just-in-time admin AWS credentials for the requested role via the Duplo API.
func (c *Client) AdminAwsGetJitAccess(role string) (*AwsJitCredentials, ClientError) {
	creds := AwsJitCredentials{}
	err := c.getAPI("AdminAwsGetJitAccess()", fmt.Sprintf("v3/admin/aws/jitAccess/%s", role), &creds)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// AdminGetK8sJitAccess retrieves just-in-time admin AWS credentials for the requested role via the Duplo API.
func (c *Client) AdminGetK8sJitAccess(plan string) (*DuploPlanK8ClusterConfig, ClientError) {
	creds := DuploPlanK8ClusterConfig{}
	err := c.getAPI(
		fmt.Sprintf("AdminGetK8sJitAccess(%s)", plan),
		fmt.Sprintf("v3/admin/plans/%s/k8sConfig", plan),
		&creds,
	)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// AdminGetJitAwsCredentials retrieves just-in-time admin AWS credentials via the Duplo API.
func (c *Client) AdminGetJitAwsCredentials() (*AwsJitCredentials, ClientError) {
	return c.AdminAwsGetJitAccess("admin")
}

// TenantGetJitAwsCredentials retrieves just-in-time AWS credentials for a tenant via the Duplo API.
func (c *Client) TenantGetJitAwsCredentials(tenantID string) (*AwsJitCredentials, ClientError) {
	creds := AwsJitCredentials{}
	err := c.getAPI(
		fmt.Sprintf("TenantGetJitAwsCredentials(%s)", tenantID),
		fmt.Sprintf("subscriptions/%s/GetAwsConsoleTokenUrl", tenantID),
		&creds,
	)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// TenantGetK8sJitAccess retrieves just-in-time admin AWS credentials for the requested role via the Duplo API.
func (c *Client) TenantGetK8sJitAccess(tenantID string) (*DuploPlanK8ClusterConfig, ClientError) {
	creds := DuploPlanK8ClusterConfig{}
	err := c.getAPI(
		fmt.Sprintf("TenantGetK8sJitAccess(%s)", tenantID),
		fmt.Sprintf("v3/subscriptions/%s/k8s/jitAccess", tenantID),
		&creds,
	)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// HelpdeskK8sClusterConfig mirrors the duplo-ai-helpdesk backend's camelCase K8sClusterConfig
// response — a different shape from the legacy DuploPlanK8ClusterConfig above (that one is
// PascalCase JSON, with K8Provider as an int; this backend serializes camelCase, with
// K8Provider as a string, e.g. "EKS"/"GKE"/"AKS"/"OKE").
type HelpdeskK8sClusterConfig struct {
	Name                           string     `json:"name,omitempty"`
	ApiServer                      string     `json:"apiServer,omitempty"`
	Token                          string     `json:"token,omitempty"`
	K8Provider                     string     `json:"k8Provider,omitempty"`
	K8sVersion                     string     `json:"k8sVersion,omitempty"`
	CertificateAuthorityDataBase64 string     `json:"certificateAuthorityDataBase64,omitempty"`
	DefaultNamespace               string     `json:"defaultNamespace,omitempty"`
	TokenExpiresAtUtc              *time.Time `json:"tokenExpiresAtUtc,omitempty"`
}

// helpdeskApiEnvelope is the {"success":..,"data":{...},"message":..} wrapper every
// duplo-ai-helpdesk response carries its payload in (that backend's ApiResponse<T>).
type helpdeskApiEnvelope struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message,omitempty"`
	Data    HelpdeskK8sClusterConfig `json:"data"`
}

// HelpdeskGetClusterJitAccess retrieves JIT Kubernetes credentials for a managed Cluster
// resource from the new duplo-ai-helpdesk backend. Distinct from TenantGetK8sJitAccess/
// AdminGetK8sJitAccess above, which call the legacy Duplo platform.
func (c *Client) HelpdeskGetClusterJitAccess(workspaceID, clusterID string) (*HelpdeskK8sClusterConfig, ClientError) {
	env := helpdeskApiEnvelope{}
	err := c.getAPI(
		fmt.Sprintf("HelpdeskGetClusterJitAccess(%s,%s)", workspaceID, clusterID),
		fmt.Sprintf("v1/aiservicedesk/user/data/workspaces/%s/environment/Clusters/%s/jitAccess", workspaceID, clusterID),
		&env,
	)
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// HelpdeskGetScopeJitAccess retrieves JIT Kubernetes credentials for a Kubernetes scope
// (Provider -> K8s -> Scope) from the new duplo-ai-helpdesk backend.
func (c *Client) HelpdeskGetScopeJitAccess(workspaceID, scopeID, namespace string) (*HelpdeskK8sClusterConfig, ClientError) {
	path := fmt.Sprintf("v1/aiservicedesk/user/data/workspaces/%s/scopes/%s/k8s/jitAccess", workspaceID, scopeID)
	if namespace != "" {
		path = fmt.Sprintf("%s?namespace=%s", path, url.QueryEscape(namespace))
	}
	env := helpdeskApiEnvelope{}
	err := c.doAPI("POST", fmt.Sprintf("HelpdeskGetScopeJitAccess(%s,%s)", workspaceID, scopeID), path, &env)
	if err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ListTenantsForUser retrieves a list of tenants for the current user via the Duplo API.
func (c *Client) ListTenantsForUser() (*[]UserTenant, ClientError) {
	list := []UserTenant{}
	err := c.getAPI("GetTenantsForUser()", "admin/GetTenantsForUser", &list)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// GetTenantFeatures retrieves a tenant's current configuration via the Duplo API.
func (c *Client) GetTenantFeatures(tenantID string) (*DuploTenantFeatures, ClientError) {
	features := DuploTenantFeatures{}
	err := c.getAPI(
		fmt.Sprintf("GetTenantFeatures(%s)", tenantID),
		fmt.Sprintf("v3/features/tenant/%s", tenantID),
		&features,
	)
	if err != nil {
		return nil, err
	}

	return &features, nil
}

// GetTenantByNameForUser retrieves a single tenant by name for the current user via the Duplo API.
func (c *Client) GetTenantByNameForUser(name string) (*UserTenant, ClientError) {
	// Get all tenants.
	allTenants, err := c.ListTenantsForUser()
	if err != nil {
		return nil, err
	}

	// Find and return the tenant with the specific name.
	for _, tenant := range *allTenants {
		if tenant.AccountName == name {
			return &tenant, nil
		}
	}

	// No tenant was found.
	return nil, nil
}

// GetTenantForUser retrieves a single tenant by ID for the current user via the Duplo API.
func (c *Client) GetTenantForUser(tenantID string) (*UserTenant, ClientError) {
	// Get all tenants.
	allTenants, err := c.ListTenantsForUser()
	if err != nil {
		return nil, err
	}

	// Find and return the tenant with the specific name.
	for _, tenant := range *allTenants {
		if tenant.TenantID == tenantID {
			return &tenant, nil
		}
	}

	// No tenant was found.
	return nil, nil
}
