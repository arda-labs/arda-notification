package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Resolver implements application.IAMResolver by calling Keycloak Admin REST API.
type Resolver struct {
	adminURL     string // e.g. "http://keycloak:8080"
	adminRealm   string // realm used for admin login, usually "master"
	clientID     string
	clientSecret string

	httpClient *http.Client

	// Simple in-memory cache to avoid hammering Keycloak on every fan-out.
	mu        sync.RWMutex
	cacheTTL  time.Duration
	cacheData map[string]cacheEntry // key: "tenant:<tenantKey>" | "role:<tenantKey>:<role>" | "platform"
}

type cacheEntry struct {
	data      any
	expiresAt time.Time
}

// New creates a Keycloak Resolver with a 30-second cache TTL.
func New(adminURL, adminRealm, clientID, clientSecret string) *Resolver {
	return &Resolver{
		adminURL:     adminURL,
		adminRealm:   adminRealm,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		cacheTTL:     30 * time.Second,
		cacheData:    make(map[string]cacheEntry),
	}
}

// keycloakUser is a minimal representation of a Keycloak user.
type keycloakUser struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// UsersByTenant returns all enabled user IDs in the given realm.
func (r *Resolver) UsersByTenant(ctx context.Context, tenantKey string) ([]string, error) {
	cacheKey := "tenant:" + tenantKey
	if cached, ok := r.fromCache(cacheKey); ok {
		return cached.([]string), nil
	}

	users, err := r.listUsers(ctx, tenantKey)
	if err != nil {
		return nil, err
	}
	ids := enabledIDs(users)
	r.toCache(cacheKey, ids)
	return ids, nil
}

// UsersByRole returns user IDs that hold roleName within the given realm.
func (r *Resolver) UsersByRole(ctx context.Context, tenantKey, roleName string) ([]string, error) {
	cacheKey := fmt.Sprintf("role:%s:%s", tenantKey, roleName)
	if cached, ok := r.fromCache(cacheKey); ok {
		return cached.([]string), nil
	}

	token, err := r.adminToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/admin/realms/%s/roles/%s/users", r.adminURL, tenantKey, roleName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak roles/%s/users: %w", roleName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak roles/%s/users: status %d", roleName, resp.StatusCode)
	}

	var users []keycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	ids := enabledIDs(users)
	r.toCache(cacheKey, ids)
	return ids, nil
}

// AllActiveUsers returns enabled users grouped by realm across all Keycloak realms.
// Each Keycloak realm is treated as a tenant.
func (r *Resolver) AllActiveUsers(ctx context.Context) (map[string][]string, error) {
	cacheKey := "platform"
	if cached, ok := r.fromCache(cacheKey); ok {
		return cached.(map[string][]string), nil
	}

	token, err := r.adminToken(ctx)
	if err != nil {
		return nil, err
	}

	// 1. List all realms (excluding master)
	type realmRep struct {
		Realm   string `json:"realm"`
		Enabled bool   `json:"enabled"`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.adminURL+"/admin/realms", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak list realms: %w", err)
	}
	defer resp.Body.Close()

	var realms []realmRep
	if err := json.NewDecoder(resp.Body).Decode(&realms); err != nil {
		return nil, err
	}

	// 2. For each realm, list enabled users.
	result := make(map[string][]string)
	for _, realm := range realms {
		if !realm.Enabled || realm.Realm == r.adminRealm {
			continue
		}
		users, err := r.listUsers(ctx, realm.Realm)
		if err != nil {
			// Log and continue rather than aborting the entire fan-out.
			continue
		}
		if ids := enabledIDs(users); len(ids) > 0 {
			result[realm.Realm] = ids
		}
	}

	r.toCache(cacheKey, result)
	return result, nil
}

// --- internal helpers ---

// adminToken fetches a short-lived admin access token from Keycloak.
func (r *Resolver) adminToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", r.adminURL, r.adminRealm)

	body := fmt.Sprintf(
		"grant_type=client_credentials&client_id=%s&client_secret=%s",
		r.clientID, r.clientSecret,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak admin token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak admin token: status %d", resp.StatusCode)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("keycloak returned empty access_token")
	}
	return tok.AccessToken, nil
}

// listUsers fetches all users from a realm (paginated, max 1000).
func (r *Resolver) listUsers(ctx context.Context, tenantKey string) ([]keycloakUser, error) {
	token, err := r.adminToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/admin/realms/%s/users?enabled=true&max=1000", r.adminURL, tenantKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak list users(%s): %w", tenantKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak list users(%s): status %d", tenantKey, resp.StatusCode)
	}

	var users []keycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}

func enabledIDs(users []keycloakUser) []string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if u.Enabled {
			ids = append(ids, u.ID)
		}
	}
	return ids
}

// fromCache retrieves a cached value if not expired.
func (r *Resolver) fromCache(key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.cacheData[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// toCache stores a value with the configured TTL.
func (r *Resolver) toCache(key string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheData[key] = cacheEntry{data: data, expiresAt: time.Now().Add(r.cacheTTL)}
}
