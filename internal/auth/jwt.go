package auth

import (
	"api-gateway/internal/reqctx"
	"context"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTVerifier struct {
	key      *rsa.PublicKey
	issuer   string
	audience string
}

func NewJWTVerifier(key *rsa.PublicKey, issuer, audience string) *JWTVerifier {
	return &JWTVerifier{key: key, issuer: issuer, audience: audience}
}

func (v *JWTVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	parsed, err := jwt.Parse(token,
		func(token *jwt.Token) (any, error) { return v.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, err
	}

	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
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
	expTime := time.Unix(int64(exp), 0)
	return &reqctx.Claims{Subject: sub, Roles: roles, Exp: expTime}, nil
}
