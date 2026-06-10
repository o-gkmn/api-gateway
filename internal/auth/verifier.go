package auth

import (
	"api-gateway/internal/reqctx"
	"context"
)

type Verifier interface {
	Verify(ctx context.Context, token string) (*reqctx.Claims, error)
}
