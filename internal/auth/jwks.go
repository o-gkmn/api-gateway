package auth

import (
	"api-gateway/internal/logger"
	"api-gateway/internal/reqctx"
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JSONWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	E   string `json:"e"`
	N   string `json:"n"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

type JWKSVerifier struct {
	mu          sync.RWMutex
	issuer      string
	audience    string
	jwks        map[string]crypto.PublicKey
	jwksUri     string
	client      *http.Client
	lastRefresh time.Time
	cooldown    time.Duration
	now         func() time.Time
	done        chan struct{}
	stopOnce    sync.Once
}

func NewJWKSVerifier(
	issuer, audience, jwksUri string,
	cooldown, interval time.Duration,
	now func() time.Time,
) *JWKSVerifier {
	v := &JWKSVerifier{
		issuer:   issuer,
		audience: audience,
		jwksUri:  jwksUri,
		cooldown: cooldown,
		now:      now,
		done:     make(chan struct{}),
	}

	if now == nil {
		v.now = time.Now
	}

	v.client = &http.Client{
		Timeout: time.Second * 10,
	}

	go v.refresh(interval)

	return v
}

func (v *JWKSVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	parsed, err := jwt.Parse(
		token,
		func(token *jwt.Token) (any, error) {
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, errors.New("missing kid in token header")
			}

			key, err := v.getKey(kid)
			if err != nil {
				if err := v.fetchKeys(ctx); err != nil {
					return nil, err
				}
				key, err = v.getKey(kid)
				if err != nil {
					return nil, err
				}
			}

			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, err
	}

	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}

	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	sub, _ := mc["sub"].(string)

	var roles []string
	if raw, ok := mc["roles"].([]any); ok {
		for _, r := range raw {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
	}

	exp, _ := mc["exp"].(float64)
	return &reqctx.Claims{Sub: sub, Roles: roles, Exp: int64(exp)}, nil
}

func PublicKeyToJWK(pub *rsa.PublicKey, kid string) JSONWebKey {
	nBytes := pub.N.Bytes()
	nStr := base64.RawURLEncoding.EncodeToString(nBytes)

	eBytes := big.NewInt(int64(pub.E)).Bytes()
	eStr := base64.RawURLEncoding.EncodeToString(eBytes)

	return JSONWebKey{
		N:   nStr,
		E:   eStr,
		Kid: kid,
		Alg: "RS256",
		Kty: "RSA",
		Use: "sig",
	}
}

func (v *JWKSVerifier) Stop() {
	v.stopOnce.Do(func() {
		close(v.done)
	})
}

func (v *JWKSVerifier) getKey(kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.jwks == nil {
		return nil, errors.New("jwks not set")
	}

	if key, ok := v.jwks[kid]; ok {
		return key, nil
	}

	return nil, errors.New("jwk not found")
}

func (v *JWKSVerifier) putKeys(jwks []JSONWebKey) error {
	newKeys := make(map[string]crypto.PublicKey)
	for _, jwk := range jwks {
		if jwk.Kty != "RSA" {
			continue
		}
		if jwk.Use != "" && jwk.Use != "sig" {
			continue
		}
		if jwk.Alg != "" && jwk.Alg != "RS256" {
			continue
		}

		pubKey, err := convertToPublicKey(jwk)
		if err != nil {
			continue
		}

		newKeys[jwk.Kid] = pubKey
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.jwks = newKeys

	return nil
}

func (v *JWKSVerifier) fetchKeys(ctx context.Context) error {
	v.mu.Lock()
	if v.now().Before(v.lastRefresh.Add(v.cooldown)) {
		v.mu.Unlock()
		return nil
	}

	v.lastRefresh = v.now()
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", v.jwksUri, nil)
	if err != nil {
		return err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to fetch JWKS: " + resp.Status)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var jwks JSONWebKeySet
	err = json.Unmarshal(respBody, &jwks)
	if err != nil {
		return err
	}

	err = v.putKeys(jwks.Keys)
	if err != nil {
		return err
	}

	return nil
}

func (v *JWKSVerifier) refresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := v.fetchKeys(context.Background()); err != nil {
				logger.Error("failed to refresh JWKS", slog.Any("error", err))
				continue
			}
		case <-v.done:
			return
		}
	}
}

func convertToPublicKey(jwk JSONWebKey) (*rsa.PublicKey, error) {
	if jwk.E == "" || jwk.N == "" {
		return nil, errors.New("jwk does not contain n or e")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}

	intE := int(big.NewInt(0).SetBytes(eBytes).Uint64())

	bigN := big.NewInt(0).SetBytes(nBytes)

	return &rsa.PublicKey{N: bigN, E: intE}, nil
}
