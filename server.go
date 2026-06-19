package clover

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nyaruka/rp-clover/runtime"
)

// shutdownTimeout bounds how long we wait for in-flight requests to drain on
// shutdown before forcing connections closed; keep it below the platform's stop
// timeout so the process exits cleanly rather than being killed.
const shutdownTimeout = 15 * time.Second

// Server is a clover server, which handles incoming handle requests and configuration updates
type Server struct {
	rt        *runtime.Runtime
	router    *chi.Mux
	server    *http.Server
	waitGroup sync.WaitGroup
	fs        http.FileSystem
}

// NewServer creates a new clover server
func NewServer(rt *runtime.Runtime, fs http.FileSystem) *Server {
	server := &Server{
		rt: rt,
		fs: fs,
	}

	router := chi.NewRouter()
	server.router = router

	// global middleware
	router.Use(middleware.Compress(flate.DefaultCompression))
	router.Use(middleware.StripSlashes)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(30 * time.Second))

	// mount our static files
	workDir, _ := os.Getwd()
	staticDir := filepath.Join(workDir, "./static")
	server.addFileServer(router, "/", http.Dir(staticDir))

	// and our admin views
	router.Mount("/admin", newAdminRouter(server))

	// and our handler view
	router.Mount("/i/{interchangeUUID:[0-9a-fA-F-]{36}}/receive", server.newHandlerFunc(handleInterchange))

	return server
}

// Start starts our clover server, returning any errors encountered
func (s *Server) Start() error {
	// wire up our main pages
	s.router.NotFound(s.handle404)
	s.router.MethodNotAllowed(s.handle405)
	s.router.Get("/", s.handleIndex)

	// configure timeouts on our server
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.rt.Config.Address, s.rt.Config.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.waitGroup.Add(1)

	// and start serving HTTP
	go func() {
		defer s.waitGroup.Done()
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
		}
	}()

	slog.Info("clover started",
		"address", s.rt.Config.Address,
		"port", s.rt.Config.Port,
		"version", s.rt.Config.Version,
	)

	return nil
}

// Stop stops our clover server, returning any errors encountered
func (s *Server) Stop() error {
	// give in-flight requests a bounded window to drain before forcing closed
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error("error shutting down server", "error", err)
	}

	// wait for everything to stop
	s.waitGroup.Wait()

	// close the database pool
	if err := s.rt.DB.Close(); err != nil {
		slog.Error("error closing database", "error", err)
	}

	slog.Info("clover stopped")
	return nil
}

type serverHandlerFunc func(*Server, http.ResponseWriter, *http.Request) error

func (s *Server) newHandlerFunc(handler serverHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(s, w, r)
		if err != nil {
			slog.Error("error handling request", "error", err)
			err = writeErrorResponse(r.Context(), w, http.StatusInternalServerError, "server error", err)
			if err != nil {
				slog.Error("error while writing", "error", err)
			}
		}
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	buf.WriteString("<title>clover</title><body><pre>\n")
	buf.WriteString(splash)
	buf.WriteString(s.rt.Config.Version)
	buf.WriteString("</pre></body>")
	w.Write(buf.Bytes())
}

func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	slog.Info("not found", "url", r.URL.String(), "method", r.Method, "resp_status", "404")
	err := writeErrorResponse(r.Context(), w, http.StatusNotFound, "not found", fmt.Errorf("not found: %s", r.URL.String()))
	if err != nil {
		slog.Error("error writing error response", "error", err)
	}
}

func (s *Server) handle405(w http.ResponseWriter, r *http.Request) {
	slog.Info("invalid method", "url", r.URL.String(), "method", r.Method, "resp_status", "405")
	err := writeErrorResponse(r.Context(), w, http.StatusNotFound, "method not allowed", fmt.Errorf("method not allowed: %s", r.Method))
	if err != nil {
		slog.Error("error writing error response", "error", err)
	}
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte("admin")) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(s.rt.Config.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Clover"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) addFileServer(r chi.Router, path string, root http.FileSystem) {
	fs := http.StripPrefix(path, http.FileServer(root))
	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}

const splash = `
     ___           ___       ___                         ___           ___     
    /  /\         /  /\     /  /\          ___          /  /\         /  /\    
   /  /::\       /  /:/    /  /::\        /  /\        /  /::\       /  /::\   
  /  /:/\:\     /  /:/    /  /:/\:\      /  /:/       /  /:/\:\     /  /:/\:\  
 /  /:/  \:\   /  /:/    /  /:/  \:\    /  /:/       /  /::\ \:\   /  /::\ \:\ 
/__/:/ \  \:\ /__/:/    /__/:/ \__\:\  /__/:/  ___  /__/:/\:\ \:\ /__/:/\:\_\:\
\  \:\  \__\/ \  \:\    \  \:\ /  /:/  |  |:| /  /\ \  \:\ \:\_\/ \__\/~|::\/:/
 \  \:\        \  \:\    \  \:\  /:/   |  |:|/  /:/  \  \:\ \:\      |  |:|::/ 
  \  \:\        \  \:\    \  \:\/:/    |__|:|__/:/    \  \:\_\/      |  |:|\/  
   \  \:\        \  \:\    \  \::/      \__\::::/      \  \:\        |__|:|   
    \__\/         \__\/     \__\/           ----        \__\/         \__\|   `
