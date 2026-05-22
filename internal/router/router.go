package router

import (
	"context"
	"net/http"
	"strings"
)

type contextKey struct{}

type methodIndex int

const maxParams = 8 // API DESIGN LIMITATIONS

const (
	methodGET methodIndex = iota
	methodPOST
	methodPUT
	methodDELETE
	methodPATCH
	methodHEAD
	methodOPTIONS
	methodCONNECT
	methodTRACE
	methodCount
)

var methodNames = [methodCount]string{
	methodGET:     "GET",
	methodPOST:    "POST",
	methodPUT:     "PUT",
	methodDELETE:  "DELETE",
	methodPATCH:   "PATCH",
	methodHEAD:    "HEAD",
	methodOPTIONS: "OPTIONS",
	methodCONNECT: "CONNECT",
	methodTRACE:   "TRACE",
}

type Handler func(w http.ResponseWriter, r *http.Request)

type Registrar interface {
	GET(path string, h Handler)
	POST(path string, h Handler)
	PUT(path string, h Handler)
	PATCH(path string, h Handler)
	DELETE(path string, h Handler)
	OPTIONS(path string, h Handler)
	HEAD(path string, h Handler)
	CONNECT(path string, h Handler)
	TRACE(path string, h Handler)
	NotFound(handler Handler)
}

var _ Registrar = (*Router)(nil)
var _ http.Handler = (*Router)(nil)

type Router struct {
	tree            *Tree
	notFoundHandler Handler
}

func NewRouter() *Router {
	return &Router{
		tree:            NewTree(),
		notFoundHandler: defaultNotFoundHandler,
	}
}

func (r *Router) NotFound(handler Handler) {
	r.notFoundHandler = handler
}

func Param(ctx context.Context, key string) string {
	p, ok := ctx.Value(contextKey{}).(*Params)
	if !ok || p == nil {
		return ""
	}
	return p.Get(key)
}

func (r *Router) GET(path string, h Handler)     { r.tree.Insert(methodGET, path, h) }
func (r *Router) POST(path string, h Handler)    { r.tree.Insert(methodPOST, path, h) }
func (r *Router) PUT(path string, h Handler)     { r.tree.Insert(methodPUT, path, h) }
func (r *Router) PATCH(path string, h Handler)   { r.tree.Insert(methodPATCH, path, h) }
func (r *Router) DELETE(path string, h Handler)  { r.tree.Insert(methodDELETE, path, h) }
func (r *Router) OPTIONS(path string, h Handler) { r.tree.Insert(methodOPTIONS, path, h) }
func (r *Router) HEAD(path string, h Handler)    { r.tree.Insert(methodHEAD, path, h) }
func (r *Router) CONNECT(path string, h Handler) { r.tree.Insert(methodCONNECT, path, h) }
func (r *Router) TRACE(path string, h Handler)   { r.tree.Insert(methodTRACE, path, h) }

var methodStringIndex = func() map[string]methodIndex {
	m := make(map[string]methodIndex, methodCount)
	for i, name := range methodNames {
		m[name] = methodIndex(i)
	}
	return m
}()

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	idx, ok := methodStringIndex[req.Method]
	if !ok {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	result := r.tree.Match(idx, req.URL.Path)
	switch result.status {
	case http.StatusOK:
		ctx := context.WithValue(req.Context(), contextKey{}, result.params)
		result.handler(w, req.WithContext(ctx))
		r.tree.ReleaseParams(result.params)
	case http.StatusMethodNotAllowed:
		w.Header().Set("Allow", strings.Join(result.allowedMethods, ", "))
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	default:
		r.notFoundHandler(w, req)
	}
}

func defaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
