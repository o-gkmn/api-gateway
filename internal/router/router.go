package router

import (
	"api-gateway/logger"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
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

type paramEntry struct {
	key   string
	value string
}

type Params struct {
	entries [maxParams]paramEntry
	count   int
}

type Router struct {
	root            *routeNode
	paramPool       sync.Pool
	notFoundHandler Handler
}

type Handler func(w http.ResponseWriter, r *http.Request)

func NewRouter() *Router {
	rootNode := &routeNode{
		prefix: "/",
		kind:   nodeStatic,
	}

	r := &Router{
		root:            rootNode,
		notFoundHandler: defaultNotFoundHandler,
	}

	r.paramPool.New = func() any {
		return &Params{}
	}

	return r
}

func (r *Router) NotFound(handler Handler) {
	r.notFoundHandler = handler
}

func (r *Router) addRoute(path string, method methodIndex, handler Handler) {
	p := parsePath(path)

	// We will traverse on parsed path. We decide what insert algorithm by whether static node or param/wildcard
	n := r.root
	for _, part := range p {
		if part.kind == nodeStatic {
			n = r.insertInto(n, part.path)
		} else {
			n = insertDynamicNode(n, part.path, part.kind)
		}
	}

	// When traverse process done we will assign the handler on relevant method.
	if n.handlers[method] != nil {
		logger.Error("route handler is already defined", slog.String("path", path))
		panic(fmt.Sprintf("handler for path '%s' is already registered", path))
	}

	n.handlers[method] = handler
}

func (r *Router) GET(path string, handler Handler) {
	r.addRoute(path, methodGET, handler)
}

func (r *Router) POST(path string, handler Handler) {
	r.addRoute(path, methodPOST, handler)
}

func (r *Router) PUT(path string, handler Handler) {
	r.addRoute(path, methodPUT, handler)
}

func (r *Router) PATCH(path string, handler Handler) {
	r.addRoute(path, methodPATCH, handler)
}

func (r *Router) DELETE(path string, handler Handler) {
	r.addRoute(path, methodDELETE, handler)
}

func (r *Router) OPTIONS(path string, handler Handler) {
	r.addRoute(path, methodOPTIONS, handler)
}

func (r *Router) HEAD(path string, handler Handler) {
	r.addRoute(path, methodHEAD, handler)
}

func (r *Router) CONNECT(path string, handler Handler) {
	r.addRoute(path, methodCONNECT, handler)
}

func (r *Router) TRACE(path string, handler Handler) {
	r.addRoute(path, methodTRACE, handler)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler, err := r.findHandler(req.Method, req.URL.Path)
	if err != nil {
		r.notFoundHandler(w, req)
		return
	}
	handler(w, req)
}

func defaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
