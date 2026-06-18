package reqctx

import (
	"context"
)

type Claims struct {
	Sub   string   `json:"sub"`
	Iss   string   `json:"iss"`
	Aud   string   `json:"aud"`
	Roles []string `json:"roles"`
	Exp   int64    `json:"exp"`
}

const claimsKey = contextKey("claims")

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func GetClaims(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok
}
