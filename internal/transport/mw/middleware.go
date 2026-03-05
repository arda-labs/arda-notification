package mw

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// InternalJWTClaims are the claims set by APISIX Lua signer.
type InternalJWTClaims struct {
	Sub      string   `json:"sub"`
	TenantID string   `json:"tid"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Issuer   string   `json:"iss"`
	jwt.RegisteredClaims
}

// internalJWTPublicKey is lazy-loaded and cached.
var internalJWTPublicKey *rsa.PublicKey

// loadInternalJWTPublicKey loads the RSA public key for verifying Internal JWTs.
// Key path is read from INTERNAL_JWT_PUBLIC_KEY_PATH env var,
// or falls back to ./keys/internal-jwt-public.pem
func loadInternalJWTPublicKey() (*rsa.PublicKey, error) {
	if internalJWTPublicKey != nil {
		return internalJWTPublicKey, nil
	}

	keyPath := os.Getenv("INTERNAL_JWT_PUBLIC_KEY_PATH")
	if keyPath == "" {
		keyPath = "/usr/local/apisix/conf/keys/internal-jwt-public.pem"
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read internal JWT public key from %s: %w", keyPath, err)
	}

	block, _ := pem.Decode(data)
	if block != nil {
		// PEM-encoded key
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse internal JWT public key PEM: %w", err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("internal JWT public key is not RSA")
		}
		internalJWTPublicKey = rsaPub
	} else {
		// Base64 DER-encoded (no PEM headers)
		cleaned := strings.TrimSpace(string(data))
		cleaned = strings.ReplaceAll(cleaned, "-----BEGIN PUBLIC KEY-----", "")
		cleaned = strings.ReplaceAll(cleaned, "-----END PUBLIC KEY-----", "")
		cleaned = strings.Join(strings.Fields(cleaned), "")

		derBytes, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("decode internal JWT public key base64: %w", err)
		}
		pub, err := x509.ParsePKIXPublicKey(derBytes)
		if err != nil {
			return nil, fmt.Errorf("parse internal JWT public key DER: %w", err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("internal JWT public key is not RSA")
		}
		internalJWTPublicKey = rsaPub
	}

	log.Info().Str("path", keyPath).Msg("Internal JWT public key loaded")
	return internalJWTPublicKey, nil
}

// InternalJWTAuth validates the X-Internal-Token header set by APISIX Gateway.
// This replaces the legacy JWTAuth (Keycloak direct) middleware.
// Flow: Client → APISIX (verifies Keycloak JWT) → signs X-Internal-Token (RS256) → Service
func InternalJWTAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr := c.Request().Header.Get("X-Internal-Token")
			if tokenStr == "" {
				log.Warn().
					Str("uri", c.Request().RequestURI).
					Msg("Missing X-Internal-Token header (expected from APISIX Gateway)")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing internal token")
			}

			pubKey, err := loadInternalJWTPublicKey()
			if err != nil {
				log.Error().Err(err).Msg("Cannot load Internal JWT public key")
				return echo.NewHTTPError(http.StatusInternalServerError, "authentication configuration error")
			}

			claims := &InternalJWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return pubKey, nil
			}, jwt.WithIssuedAt(), jwt.WithExpirationRequired())

			if err != nil || !token.Valid {
				log.Warn().
					Err(err).
					Str("uri", c.Request().RequestURI).
					Msg("Internal JWT verification failed")
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid internal token")
			}

			// Store validated info in context
			c.Set("userID", claims.Sub)
			c.Set("tenantID", claims.TenantID)
			c.Set("username", claims.Username)
			c.Set("email", claims.Email)
			c.Set("roles", claims.Roles)

			log.Trace().
				Str("userID", claims.Sub).
				Str("tenantID", claims.TenantID).
				Msg("Internal JWT verified")

			return next(c)
		}
	}
}

// TenantResolver resolves the tenantKey from the X-Tenant-ID header or JWT claims.
func TenantResolver() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenantKey := c.Request().Header.Get("X-Tenant-ID")
			if tenantKey == "" {
				// Fallback: use tenantID stored by InternalJWTAuth from JWT claims
				tenantKey, _ = c.Get("tenantID").(string)
			}
			if tenantKey == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "X-Tenant-ID header is required")
			}
			c.Set("tenantKey", tenantKey)
			return next(c)
		}
	}
}
