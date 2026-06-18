package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const attrsGroup = "attrs"

type ctxKey struct{}

type ctxHandler struct{ slog.Handler }

func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(ctxKey{}).([]slog.Attr); ok {
		for _, a := range attrs {
			r.AddAttrs(a)
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{h.Handler.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{h.Handler.WithGroup(name)}
}

func Init() {
	h := ctxHandler{slog.NewJSONHandler(os.Stdout, getHandlerOptions())}
	slog.SetDefault(slog.New(h))
}

func getHandlerOptions() *slog.HandlerOptions {
	return &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if a.Value.Kind() == slog.KindTime {
					a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
				}
			}

			return a
		},
	}
}

func With(ctx context.Context, attrs ...slog.Attr) context.Context {
	existing, _ := ctx.Value(ctxKey{}).([]slog.Attr)
	merged := make([]slog.Attr, 0, len(existing)+len(attrs))
	merged = append(merged, existing...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, ctxKey{}, merged)
}

func Debug(msg string, attrs ...slog.Attr) {
	logAttrs(context.Background(), slog.LevelDebug, msg, attrs)
}
func Info(msg string, attrs ...slog.Attr) {
	logAttrs(context.Background(), slog.LevelInfo, msg, attrs)
}

func Warn(msg string, attrs ...slog.Attr) {
	logAttrs(context.Background(), slog.LevelWarn, msg, attrs)
}

func Error(msg string, attrs ...slog.Attr) {
	logAttrs(context.Background(), slog.LevelError, msg, attrs)
}

func DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelDebug, msg, attrs)
}
func InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelInfo, msg, attrs)
}
func WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelWarn, msg, attrs)
}
func ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelError, msg, attrs)
}

func groupAttr(attrs []slog.Attr) slog.Attr {
	return slog.Attr{Key: attrsGroup, Value: slog.GroupValue(attrs...)}
}

func logAttrs(ctx context.Context, lvl slog.Level, msg string, attrs []slog.Attr) {
	if len(attrs) == 0 {
		slog.LogAttrs(ctx, lvl, msg)
		return
	}
	slog.LogAttrs(ctx, lvl, msg, groupAttr(attrs))
}
