package gateway

import (
	"io/fs"
	"net/http"
)

func (s *Server) mountWeb() {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r)
	})
	s.mux.Handle("GET /app.js", fileServer)
	s.mux.Handle("GET /app.css", fileServer)
}
