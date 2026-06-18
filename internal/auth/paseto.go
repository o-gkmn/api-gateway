package auth

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/utils"
	"api-gateway/pkg/paseto"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"time"
)

const headerStr = "v4.public."

type PasetoVerifier struct {
	key      ed25519.PublicKey
	issuer   string
	audience string
	now      func() time.Time
}

func NewPasetoVerifier(
	issuer, audience string,
	key ed25519.PublicKey,
	now func() time.Time,
) *PasetoVerifier {
	if now == nil {
		now = time.Now
	}
	return &PasetoVerifier{
		key:      key,
		issuer:   issuer,
		audience: audience,
		now:      now,
	}
}

func (v *PasetoVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	parsedToken := utils.SplitN4(token)
	if parsedToken[0] != "v4" || parsedToken[1] != "public" || parsedToken[2] == "" {
		return nil, errors.New("invalid token")
	}

	m := parsedToken[2]
	f := parsedToken[3]

	mBytes, err := paseto.VerifyV4Public(m, f, v.key)
	if err != nil {
		return nil, err
	}

	var claims reqctx.Claims
	err = json.Unmarshal(mBytes, &claims)
	if err != nil {
		return nil, err
	}

	if v.issuer != "" && claims.Iss != v.issuer {
		return nil, errors.New("invalid issuer")
	}

	if v.audience != "" && claims.Aud != v.audience {
		return nil, errors.New("invalid audience")
	}

	nowUnix := v.now().Unix()
	if nowUnix > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}
