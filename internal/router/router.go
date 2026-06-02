package router

import (
	"net/http"
	"strings"
)

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

type Handler func(w http.ResponseWriter, r *http.Request, params *Params)

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

type Router struct {
	tree                    *Tree
	notFoundHandler         Handler
	methodNotAllowedHandler Handler
	caseInsensitive         bool
}

func NewRouter() *Router {
	return &Router{
		tree:                    NewTree(),
		notFoundHandler:         defaultNotFoundHandler,
		methodNotAllowedHandler: defaultMethodNotAllowedHandler,
		caseInsensitive:         false,
	}
}

// NotFound sets the handler invoked when no route matches the request path.
// The params argument is always nil for not-found responses; do not access it.
func (r *Router) NotFound(handler Handler) {
	r.notFoundHandler = handler
}

func (r *Router) MethodNotAllowed(handler Handler) {
	r.methodNotAllowedHandler = handler
}

func (r *Router) CaseInsensitive() {
	r.caseInsensitive = true
}

func (r *Router) GET(path string, h Handler)     { r.tree.Insert(methodGET, r.normalize(path), h) }
func (r *Router) POST(path string, h Handler)    { r.tree.Insert(methodPOST, r.normalize(path), h) }
func (r *Router) PUT(path string, h Handler)     { r.tree.Insert(methodPUT, r.normalize(path), h) }
func (r *Router) PATCH(path string, h Handler)   { r.tree.Insert(methodPATCH, r.normalize(path), h) }
func (r *Router) DELETE(path string, h Handler)  { r.tree.Insert(methodDELETE, r.normalize(path), h) }
func (r *Router) OPTIONS(path string, h Handler) { r.tree.Insert(methodOPTIONS, r.normalize(path), h) }
func (r *Router) HEAD(path string, h Handler)    { r.tree.Insert(methodHEAD, r.normalize(path), h) }
func (r *Router) CONNECT(path string, h Handler) { r.tree.Insert(methodCONNECT, r.normalize(path), h) }
func (r *Router) TRACE(path string, h Handler)   { r.tree.Insert(methodTRACE, r.normalize(path), h) }

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
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}

	path := r.normalize(req.URL.Path)

	// /users/ and /user are same route
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	result := r.tree.Match(idx, path)
	switch result.status {
	case http.StatusOK:
		defer r.tree.ReleaseParams(result.params)
		result.handler(w, req, result.params)
	case http.StatusMethodNotAllowed:
		w.Header().Set("Allow", strings.Join(result.allowedMethods, ", "))
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	default:
		r.notFoundHandler(w, req, nil)
	}
}

func defaultNotFoundHandler(w http.ResponseWriter, r *http.Request, params *Params) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func defaultMethodNotAllowedHandler(w http.ResponseWriter, r *http.Request, params *Params) {
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (r *Router) normalize(path string) string {
	if r.caseInsensitive {
		return strings.ToLower(path)
	}

	return path
}
