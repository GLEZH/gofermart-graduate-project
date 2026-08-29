package httpapi

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed openapi.yaml
var openAPISpec []byte

const (
	specPath    = "/api/openapi.yaml"
	swaggerPath = "/swagger"
)

func (s *Server) mountDocs(router chi.Router) {
	router.Get(specPath, s.openAPI)
	router.Get(swaggerPath, s.swaggerUI)
	router.Get(swaggerPath+"/", s.swaggerUI)
}

func (s *Server) openAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(openAPISpec); err != nil && s.log != nil {
		s.log.Errorw("write openapi spec", "error", err)
	}
}

func (s *Server) swaggerUI(w http.ResponseWriter, r *http.Request) {
	s.render(w, "swagger.html", map[string]string{"Spec": specPath})
}
