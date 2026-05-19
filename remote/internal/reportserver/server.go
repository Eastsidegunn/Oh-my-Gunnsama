package reportserver

import (
	"context"
	"net/http"
	"time"
)

type Server struct {
	dir     string
	mux     *http.ServeMux
	refresh func() error
}

func New(dir string) *Server { return NewWithRefresh(dir, nil) }

func NewWithRefresh(dir string, refresh func() error) *Server {
	s := &Server{dir: dir, mux: http.NewServeMux(), refresh: refresh}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if s.refresh != nil {
			if err := s.refresh(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, s.dir+"/index.html")
			return
		}
		http.FileServer(http.Dir(s.dir)).ServeHTTP(w, r)
	})
}

func (s *Server) RefreshLoop(ctx context.Context, interval time.Duration) {
	if s.refresh == nil {
		<-ctx.Done()
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.refresh()
		case <-ctx.Done():
			return
		}
	}
}
