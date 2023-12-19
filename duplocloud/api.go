package duplocloud

import (
	"fmt"
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

// DuploInfrastructureConfig represents an infrastructure configuration
type DuploInfrastructure struct {
	Name                    string `json:"Name,omitempty"`
	Region                  string `json:"Region,omitempty"`
	EnableK8Cluster         bool   `json:"EnableK8Cluster,omitempty"`
	EnableECSCluster        bool   `json:"EnableECSCluster,omitempty"`
	EnableContainerInsights bool   `json:"EnableContainerInsights,omitempty"`
	ProvisioningStatus      string `json:"ProvisioningStatus,omitempty"`
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

// AdminGetJITAwsCredentials retrieves just-in-time admin AWS credentials via the Duplo API.
func (c *Client) AdminGetJitAwsCredentials() (*AwsJitCredentials, ClientError) {
	return c.AdminAwsGetJitAccess("admin")
}

// TenantGetJITAwsCredentials retrieves just-in-time AWS credentials for a tenant via the Duplo API.
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

// ListTenantsForUser retrieves a list of tenants for the current user via the Duplo API.
func (c *Client) ListTenantsForUser() (*[]UserTenant, ClientError) {
	list := []UserTenant{}
	err := c.getAPI("GetTenantsForUser()", "admin/GetTenantsForUser", &list)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *Client) AdminGetInfrastructure(infraName string) (*DuploInfrastructure, ClientError) {
	config := DuploInfrastructure{}
	err := c.getAPI(
		fmt.Sprintf("AdminGetInfrastructure(%s)", infraName),
		fmt.Sprintf("v3/admin/infrastructure/%s", infraName),
		&config,
	)
	if err != nil {
		return nil, err
	}
	return &config, nil
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
