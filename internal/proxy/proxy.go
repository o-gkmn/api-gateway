package proxy

import (
	"api-gateway/internal/logger"
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
)

type instanceKey struct{}

type Proxy struct {
	pool      *Pool
	transport http.RoundTripper
	rp        *httputil.ReverseProxy
}

func NewProxy(pool *Pool, transport http.RoundTripper) *Proxy {
	p := &Proxy{
		pool:      pool,
		transport: transport,
	}

	p.rp = &httputil.ReverseProxy{
		Rewrite:        p.rewrite,
		Transport:      transport,
		ErrorHandler:   p.errorHandler,
		ModifyResponse: p.modifyResponse,
	}

	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, params *router.Params) {
	inst := p.pool.Pick()
	if inst == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	ctx := context.WithValue(r.Context(), instanceKey{}, inst)
	p.rp.ServeHTTP(w, r.WithContext(ctx))
}

func (p *Proxy) rewrite(pr *httputil.ProxyRequest) {
	inst, ok := pr.In.Context().Value(instanceKey{}).(*Instance)
	if !ok {
		logger.Error("rewrite: instance missing from context")
		return
	}

	pr.SetURL(inst.URL)
	pr.SetXForwarded()

	if claims, ok := reqctx.GetClaims(pr.In.Context()); ok {
		pr.Out.Header.Set("X-Gateway-Sub", claims.Sub)
		pr.Out.Header.Set("X-Gateway-Roles", strings.Join(claims.Roles, ","))
	}
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	logger.Error("upstream error", slog.Any("error", err))
	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
}

func (p *Proxy) modifyResponse(resp *http.Response) error {
	resp.Header.Set("X-Gateway", "true")
	resp.Header.Del("Server")
	resp.Header.Del("X-Powered-By")
	resp.Header.Del("X-AspNet-Version")
	resp.Header.Del("X-AspNetMvc-Version")
	resp.Header.Del("X-Generator")
	return nil
}
