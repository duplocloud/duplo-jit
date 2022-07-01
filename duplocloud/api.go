package duplocloud

import "fmt"

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

// AdminAwsGetJitAccess retrieves just-in-time admin AWS credentials for the requested role via the Duplo API.
func (c *Client) AdminAwsGetJitAccess(role string) (*AwsJitCredentials, ClientError) {
	creds := AwsJitCredentials{}
	err := c.getAPI("AdminAwsGetJitAccess()", fmt.Sprintf("v3/admin/aws/jitAccess/%s", role), &creds)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// AdminGetJITAwsCredentials retrieves just-in-time admin AWS credentials via the Duplo API.
func (c *Client) AdminGetJITAwsCredentials() (*AwsJitCredentials, ClientError) {
	creds, err := c.AdminAwsGetJitAccess("admin")

	// Fallback to legacy API
	if err != nil && err.Status() == 404 {
		creds, err = c.LegacyAdminGetJITAwsCredentials()
	}

	if err != nil {
		return nil, err
	}
	return creds, err
}

// LegacyAdminGetJITAwsCredentials retrieves just-in-time admin AWS credentials via the Duplo API.
func (c *Client) LegacyAdminGetJITAwsCredentials() (*AwsJitCredentials, ClientError) {
	creds := AwsJitCredentials{}
	err := c.getAPI("AdminGetJITAwsCredentials()", "adminproxy/GetJITAwsConsoleAccessUrl", &creds)
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// TenantGetJITAwsCredentials retrieves just-in-time AWS credentials for a tenant via the Duplo API.
func (c *Client) TenantGetJITAwsCredentials(tenantID string) (*AwsJitCredentials, ClientError) {
	creds := AwsJitCredentials{}
	err := c.getAPI(
		fmt.Sprintf("TenantGetAwsCredentials(%s)", tenantID),
		fmt.Sprintf("subscriptions/%s/GetAwsConsoleTokenUrl", tenantID),
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
