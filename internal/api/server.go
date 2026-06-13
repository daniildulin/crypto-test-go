package api

import (
	"log"
	"net/http"
	"time"
)

// NewRouter собирает роуты. Method-pattern роутинг из stdlib mux (Go 1.22+).
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/createaddress", h.CreateAddress)
	mux.HandleFunc("POST /api/v1/validateaddress", h.ValidateAddress)
	mux.HandleFunc("POST /api/v1/tx", h.Tx)

	return logging(mux)
}

// logging - крошечная access-log прослойка.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
