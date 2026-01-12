package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	EmailKey  contextKey = "email"
	NameKey   contextKey = "name"
)

type AuthMiddleware struct {
	jwtSecret    string
	supabaseURL  string
	publicKeyMu  sync.RWMutex
	publicKeys   map[string]*ecdsa.PublicKey
	lastFetch    time.Time
	fetchTimeout time.Duration
}

func NewAuthMiddleware(jwtSecret, supabaseURL string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:    jwtSecret,
		supabaseURL:  supabaseURL,
		fetchTimeout: 1 * time.Hour,
	}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		tokenString := parts[1]
		if len(tokenString) == 0 {
			respondError(w, http.StatusUnauthorized, "empty token")
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if m.jwtSecret == "" {
				log.Printf("[AUTH] JWT secret not configured")
				return nil, fmt.Errorf("jwt secret not configured")
			}

			if strings.HasPrefix(m.jwtSecret, "eyJ") {
				log.Printf("[AUTH] JWT secret appears to be a token, not a secret")
				return nil, fmt.Errorf("SUPABASE_JWT_SECRET is set to a JWT token instead of the JWT secret string")
			}

			alg := token.Method.Alg()

			switch alg {
			case "HS256":
				if decodedSecret, err := base64.StdEncoding.DecodeString(m.jwtSecret); err == nil {
					return decodedSecret, nil
				}
				return []byte(m.jwtSecret), nil
			case "ES256":
				kid, _ := token.Header["kid"].(string)
				publicKey, err := m.getSupabasePublicKey(kid)
				if err != nil {
					log.Printf("[AUTH] Failed to get Supabase public key: %v", err)
					return nil, fmt.Errorf("ES256 verification failed: %w. Please ensure frontend sends access_token, not id_token", err)
				}
				return publicKey, nil
			default:
				log.Printf("[AUTH] Unexpected signing method: %v", alg)
				return nil, fmt.Errorf("unexpected signing method: %v", alg)
			}
		})

		if err != nil {
			log.Printf("[AUTH] Token parsing failed for %s %s: %v", r.Method, r.URL.Path, err)
			respondError(w, http.StatusUnauthorized, fmt.Sprintf("invalid token: %v", err))
			return
		}

		if !token.Valid {
			log.Printf("[AUTH] Token is invalid for %s %s", r.Method, r.URL.Path)
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("[AUTH] Invalid token claims type for %s %s", r.Method, r.URL.Path)
			respondError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}
		userID, ok := claims["sub"].(string)
		if !ok {
			log.Printf("[AUTH] User ID not found in token claims for %s %s (claims: %v)", r.Method, r.URL.Path, claims)
			respondError(w, http.StatusUnauthorized, "user id not found in token")
			return
		}

		email, _ := claims["email"].(string)
		name := ""
		if metadata, ok := claims["user_metadata"].(map[string]interface{}); ok {
			if n, ok := metadata["full_name"].(string); ok {
				name = n
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		if email != "" {
			ctx = context.WithValue(ctx, EmailKey, email)
		}
		if name != "" {
			ctx = context.WithValue(ctx, NameKey, name)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailKey).(string)
	return email, ok
}

func GetUserName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(NameKey).(string)
	return name, ok
}

func (m *AuthMiddleware) getSupabasePublicKey(kid string) (*ecdsa.PublicKey, error) {
	m.publicKeyMu.RLock()
	if m.publicKeys != nil && time.Since(m.lastFetch) < m.fetchTimeout {
		if key, ok := m.publicKeys[kid]; ok {
			defer m.publicKeyMu.RUnlock()
			return key, nil
		}
		if kid == "" && len(m.publicKeys) > 0 {
			for _, key := range m.publicKeys {
				defer m.publicKeyMu.RUnlock()
				return key, nil
			}
		}
	}
	m.publicKeyMu.RUnlock()

	m.publicKeyMu.Lock()
	defer m.publicKeyMu.Unlock()

	if m.publicKeys != nil && time.Since(m.lastFetch) < m.fetchTimeout {
		if key, ok := m.publicKeys[kid]; ok {
			return key, nil
		}
	}

	if m.supabaseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL not configured")
	}

	baseURL := strings.TrimSuffix(m.supabaseURL, "/")
	jwksURLs := []string{
		baseURL + "/auth/v1/.well-known/jwks.json",
		baseURL + "/.well-known/jwks.json",
		baseURL + "/auth/.well-known/jwks.json",
	}

	var resp *http.Response
	for _, url := range jwksURLs {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		client := &http.Client{Timeout: 5 * time.Second}
		tryResp, err := client.Do(req)
		cancel()

		if err == nil && tryResp.StatusCode == http.StatusOK {
			resp = tryResp
			break
		}
		if tryResp != nil {
			tryResp.Body.Close()
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to fetch JWKS from Supabase")
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			X   string `json:"x"`
			Y   string `json:"y"`
			Crv string `json:"crv"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	newPublicKeys := make(map[string]*ecdsa.PublicKey)
	for _, key := range jwks.Keys {
		xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil {
			continue
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
		if err != nil {
			continue
		}

		pubKey := &ecdsa.PublicKey{
			Curve: getCurve(key.Crv),
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}
		newPublicKeys[key.Kid] = pubKey
	}

	m.publicKeys = newPublicKeys
	m.lastFetch = time.Now()

	if key, ok := m.publicKeys[kid]; ok {
		return key, nil
	}
	if kid == "" && len(m.publicKeys) > 0 {
		for _, key := range m.publicKeys {
			return key, nil
		}
	}

	return nil, fmt.Errorf("key with kid %s not found in JWKS (available keys: %d)", kid, len(m.publicKeys))
}

func getCurve(crv string) *elliptic.CurveParams {
	switch crv {
	case "P-256":
		return elliptic.P256().Params()
	case "P-384":
		return elliptic.P384().Params()
	case "P-521":
		return elliptic.P521().Params()
	default:
		return elliptic.P256().Params()
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
