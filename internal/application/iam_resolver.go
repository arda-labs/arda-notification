package application

import "context"

// IAMResolver resolves a TargetScope to a concrete list of (tenantKey, userID) pairs.
// The default implementation calls Keycloak Admin REST API.
// A no-op implementation can be used for testing.
type IAMResolver interface {
	// UsersByTenant returns all active user IDs in the given tenant realm.
	UsersByTenant(ctx context.Context, tenantKey string) ([]string, error)

	// UsersByRole returns user IDs that hold roleName within a tenant realm.
	UsersByRole(ctx context.Context, tenantKey, roleName string) ([]string, error)

	// AllActiveUsers returns active users grouped by tenantKey across all tenants.
	// Used for PLATFORM-scope fan-out.
	AllActiveUsers(ctx context.Context) (map[string][]string, error)
}
