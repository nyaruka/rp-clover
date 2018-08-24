//go:generate statik -src=./static

package clover

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/rp-clover/migrations"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	config    *Config
	router    *chi.Mux
	server    *http.Server
	db        *sqlx.DB
	waitGroup sync.WaitGroup
	fs        http.FileSystem
}

func NewServer(config *Config, fs http.FileSystem) *Server {
	server := &Server{
		config: config,
		fs:     fs,
	}

	router := chi.NewRouter()
	server.router = router

	// global middleware
	router.Use(middleware.DefaultCompress)
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
	router.Mount("/handle/{interchangeUUID:[0-9a-fA-F-]{36}}", server.newHandlerFunc(handleInterchange))

	return server
}

func (s *Server) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	db, err := sqlx.Open("postgres", s.config.DB)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(4)
	s.db = db

	err = s.db.PingContext(ctx)
	if err != nil {
		log.WithError(err).Error("unable to ping database")
		return err
	}

	// migrate our db forward
	err = migrations.Migrate(ctx, db)
	if err != nil {
		return err
	}

	// wire up our main pages
	s.router.NotFound(s.handle404)
	s.router.MethodNotAllowed(s.handle405)
	s.router.Get("/", s.handleIndex)

	// configure timeouts on our server
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Address, s.config.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// and start serving HTTP
	go func() {
		s.waitGroup.Add(1)
		defer s.waitGroup.Done()
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("http server error")
		}
	}()

	log.WithFields(log.Fields{
		"address": s.config.Address,
		"port":    s.config.Port,
		"version": s.config.Version,
	}).Info("clover started")

	return nil
}

func (s *Server) Stop() error {
	if err := s.server.Shutdown(context.Background()); err != nil {
		log.WithError(err).Error("error shutting down server")
	}

	// wait for everything to stop
	s.waitGroup.Wait()

	log.Info("clover stopped")
	return nil
}

type ServerHandlerFunc func(*Server, http.ResponseWriter, *http.Request) error

func (s *Server) newHandlerFunc(handler ServerHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(s, w, r)
		if err != nil {
			log.WithError(err).Error()
			err = writeErrorResponse(r.Context(), w, http.StatusInternalServerError, "server error", err)
			if err != nil {
				log.WithError(err).Error("error while writing")
			}
		}
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	buf.WriteString("<title>clover</title><body><pre>\n")
	buf.WriteString(splash)
	buf.WriteString(s.config.Version)
	buf.WriteString("</pre></body>")
	w.Write(buf.Bytes())
}

func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
	log.WithField("url", r.URL.String()).WithField("method", r.Method).WithField("resp_status", "404").Info("not found")
	err := writeErrorResponse(r.Context(), w, http.StatusNotFound, "not found", fmt.Errorf("not found: %s", r.URL.String()))
	if err != nil {
		log.WithError(err).Error()
	}
}

func (s *Server) handle405(w http.ResponseWriter, r *http.Request) {
	log.WithField("url", r.URL.String()).WithField("method", r.Method).WithField("resp_status", "405").Info("invalid method")
	err := writeErrorResponse(r.Context(), w, http.StatusNotFound, "method not allowed", fmt.Errorf("method not allowed: %s", r.Method))
	if err != nil {
		log.WithError(err).Error()
	}
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte("admin")) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(s.config.Password)) != 1 {
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
