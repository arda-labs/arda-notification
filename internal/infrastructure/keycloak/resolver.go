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
	// adminUser/adminPassword are used for Resource Owner Password Grant (dev fallback).
	adminUser     string
	adminPassword string

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
// If clientSecret is empty, it falls back to Resource Owner Password grant
// using adminUser/adminPassword (useful for local development).
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

// NewWithPassword creates a Keycloak Resolver that uses Resource Owner Password Grant.
// Use this in local development when a dedicated service-account client is not set up.
func NewWithPassword(adminURL, adminRealm, clientID, adminUser, adminPassword string) *Resolver {
	return &Resolver{
		adminURL:      adminURL,
		adminRealm:    adminRealm,
		clientID:      clientID,
		adminUser:     adminUser,
		adminPassword: adminPassword,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		cacheTTL:      30 * time.Second,
		cacheData:     make(map[string]cacheEntry),
	}
}

// SetPasswordFallback sets the admin username/password used when clientSecret is empty.
// Call this after New() to configure the dev fallback.
func (r *Resolver) SetPasswordFallback(user, password string) {
	r.adminUser = user
	r.adminPassword = password
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
// Uses client_credentials if clientSecret is set, otherwise falls back
// to Resource Owner Password Grant (for local dev with admin-cli).
func (r *Resolver) adminToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", r.adminURL, r.adminRealm)

	var body string
	if r.clientSecret != "" {
		// Production: client_credentials grant
		body = fmt.Sprintf(
			"grant_type=client_credentials&client_id=%s&client_secret=%s",
			r.clientID, r.clientSecret,
		)
	} else {
		// Dev fallback: Resource Owner Password Grant with admin-cli
		body = fmt.Sprintf(
			"grant_type=password&client_id=%s&username=%s&password=%s",
			r.clientID, r.adminUser, r.adminPassword,
		)
	}

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

// UserEmail returns the email address for a user in the given realm.
func (r *Resolver) UserEmail(ctx context.Context, tenantKey, userID string) (string, error) {
	token, err := r.adminToken(ctx)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/admin/realms/%s/users/%s", r.adminURL, tenantKey, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak get user %s: %w", userID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak get user %s: status %d", userID, resp.StatusCode)
	}

	var user struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	return user.Email, nil
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
