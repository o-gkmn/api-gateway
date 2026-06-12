package reqctx

import "context"

const payloadKey contextKey = "payload"

func WithPayload[T any](ctx context.Context, payload T) context.Context {
	return context.WithValue(ctx, payloadKey, payload)
}

func GetPayload[T any](ctx context.Context) (T, bool) {
	p, ok := ctx.Value(payloadKey).(T)
	return p, ok
}
