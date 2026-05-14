package server

type Middleware func(Handler) Handler

func Chain(h Handler, middlewares ...Middleware) Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

func (s *Server) ApplyMiddleware(h Handler) Handler {
	if len(s.middlewares) == 0 {
		return h
	}

	return Chain(h, s.middlewares...)
}

func (s *Server) Use(middlewares ...Middleware) {
	s.middlewares = append(s.middlewares, middlewares...)
}
