package httpapi

import (
	"compress/gzip"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if s.log != nil {
			s.log.Infow("request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "size", recorder.size, "duration", time.Since(started))
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if s.log != nil {
					s.log.Errorw("panic", "value", recovered, "stack", string(debug.Stack()))
				}
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	status      int
	wroteHeader bool
	compress    bool
}

func (w *gzipWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
	}
}

func (w *gzipWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.flushHeader()
	}
	if w.writer != nil {
		return w.writer.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *gzipWriter) flushHeader() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.compress && compressible(w.Header().Get("Content-Type")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.writer = gzip.NewWriter(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(w.status)
	w.wroteHeader = true
}

func compressible(contentType string) bool {
	for _, prefix := range []string{"text/", "application/json", "application/yaml"} {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			reader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer reader.Close()
			r.Body = io.NopCloser(reader)
		}
		writer := &gzipWriter{ResponseWriter: w, compress: strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")}
		next.ServeHTTP(writer, r)
		if !writer.wroteHeader {
			writer.flushHeader()
		}
		if writer.writer != nil {
			_ = writer.writer.Close()
		}
	})
}
