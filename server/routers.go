package server

import (
	"fmt"
	"net/http"
	"strings"
)

type Handler func(w http.ResponseWriter, r *http.Request)

type Router struct {
	routes          map[string]map[string]Handler
	server          *Server
	notFoundHandler Handler
}

func NewRouter(server *Server) *Router {
	return &Router{
		server:          server,
		routes:          make(map[string]map[string]Handler),
		notFoundHandler: defaultNotFoundHandler,
	}
}

func (r *Router) NotFound(handler Handler) {
	r.notFoundHandler = handler
}

func (r *Router) addRoute(method, path string, handler Handler) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]Handler)
	}

	if r.routes[method][path] != nil {
		panic(fmt.Sprintf("route already defined: %s %s", method, path))
	}

	r.routes[method][path] = handler
}

func (r *Router) GET(path string, handler Handler) {
	r.addRoute(http.MethodGet, path, handler)
}

func (r *Router) POST(path string, handler Handler) {
	r.addRoute(http.MethodPost, path, handler)
}

func (r *Router) PUT(path string, handler Handler) {
	r.addRoute(http.MethodPut, path, handler)
}

func (r *Router) PATCH(path string, handler Handler) {
	r.addRoute(http.MethodPatch, path, handler)
}

func (r *Router) DELETE(path string, handler Handler) {
	r.addRoute(http.MethodDelete, path, handler)
}

func (r *Router) OPTIONS(path string, handler Handler) {
	r.addRoute(http.MethodOptions, path, handler)
}

func (r *Router) HEAD(path string, handler Handler) {
	r.addRoute(http.MethodHead, path, handler)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler, err := r.findHandler(req.Method, req.URL.Path)
	if err != nil {
		r.notFoundHandler(w, req)
		return
	}

	if r.server != nil {
		handler = r.server.ApplyMiddleware(handler)
	}

	handler(w, req)
}

func (r *Router) findHandler(method, path string) (Handler, error) {
	if methodRoutes, ok := r.routes[method]; ok {
		if handler, ok := methodRoutes[path]; ok {
			return handler, nil
		}

		for routePath, handler := range methodRoutes {
			if isWildcardMatch(routePath, path) {
				return handler, nil
			}
		}
	}
	return nil, fmt.Errorf("route not found: %s %s", method, path)
}

func isWildcardMatch(routePath, requestPath string) bool {
	routeParts := strings.Split(routePath, "/")
	requestParts := strings.Split(requestPath, "/")

	if len(routeParts) != len(requestParts) {
		return false
	}

	for i, routePart := range routeParts {
		if routePart == "*" {
			continue
		}

		if routePart != requestParts[i] {
			return false
		}
	}

	return true
}

func defaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
