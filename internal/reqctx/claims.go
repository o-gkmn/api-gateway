package reqctx

import "context"

type Claims struct {
	Subject string
	Role    []string
}

const claimsKey = contextKey("claims")

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func GetClaims(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok
}
