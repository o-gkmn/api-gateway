package httpx

import (
	"bufio"
	"net"
	"net/http"
)

type ResponseRecorder interface {
	http.ResponseWriter
	Status() int
	Bytes() int
	Wrote() bool
}

type recorder struct {
	http.ResponseWriter
	wrote  bool
	status int
	bytes  int
}

type recorderHijacker struct {
	*recorder
}

func (r *recorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *recorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorder) Wrote() bool { return r.wrote }

func (r *recorder) Status() int { return r.status }

func (r *recorder) Bytes() int { return r.bytes }

func (r *recorderHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.ResponseWriter.(http.Hijacker).Hijack()
}

func WrapWriter(w http.ResponseWriter) ResponseRecorder {
	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	if _, ok := w.(http.Hijacker); ok {
		return &recorderHijacker{recorder: rec}
	}
	return rec
}
