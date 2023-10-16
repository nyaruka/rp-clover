package clover

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nyaruka/rp-clover/models"
)

func newAdminRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	router.Use(s.adminOnly)
	router.Method(http.MethodGet, "/", s.newHandlerFunc(viewConfig))
	router.Method(http.MethodPost, "/", s.newHandlerFunc(updateConfig))
	router.Mount("/{interchangeUUID:[0-9a-fA-F-]{36}}/map", s.newHandlerFunc(handleMap))

	return router
}

func renderInterchanges(s *Server, w http.ResponseWriter, r *http.Request, config []byte, message string, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	tpl, err := loadTemplate(s.fs, "/admin/index.html")
	if err != nil {
		return err
	}

	err = tpl.Execute(w, map[string]interface{}{
		"config":  string(config),
		"message": message,
		"error":   errMsg,
	})
	if err != nil {
		return err
	}

	return nil
}

func viewConfig(s *Server, w http.ResponseWriter, r *http.Request) error {
	interchanges, err := models.GetInterchangeConfig(r.Context(), s.db)
	if err != nil {
		slog.Error("error loading interchange config", "error", err)
		return err
	}

	config, err := json.MarshalIndent(interchanges, "", "    ")
	if err != nil {
		return err
	}
	return renderInterchanges(s, w, r, config, "", err)
}

func updateConfig(s *Server, w http.ResponseWriter, r *http.Request) error {
	interchanges, err := models.GetInterchangeConfig(r.Context(), s.db)
	if err != nil {
		return err
	}

	config, err := json.MarshalIndent(interchanges, "", "    ")
	if err != nil {
		return err
	}

	err = r.ParseForm()
	if err != nil {
		return renderInterchanges(s, w, r, config, "", err)
	}

	config = []byte(r.Form.Get("config"))
	slog.Info("received new config", "config", string(config))

	// try to create our config
	interchanges = make([]*models.Interchange, 0)
	err = json.Unmarshal(config, &interchanges)
	if err != nil {
		return renderInterchanges(s, w, r, config, "", err)
	}

	err = models.UpdateInterchangeConfig(r.Context(), s.db, interchanges)
	if err != nil {
		return renderInterchanges(s, w, r, config, "", err)
	}

	// reselect our current interchanges
	interchanges, err = models.GetInterchangeConfig(r.Context(), s.db)
	if err != nil {
		slog.Error("error loading interchange config", "error", err)
		return err
	}

	config, err = json.MarshalIndent(interchanges, "", "    ")
	if err != nil {
		return err
	}

	return renderInterchanges(s, w, r, config, "configuration saved", err)
}

func loadTemplate(fs http.FileSystem, name string) (*template.Template, error) {
	file, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	text, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return template.New(name).Parse(string(text))
}

// handles a mapping request
func handleMap(s *Server, w http.ResponseWriter, r *http.Request) error {
	interchangeUUID := chi.URLParam(r, "interchangeUUID")

	// look up our interchange
	interchange, err := models.GetInterchange(r.Context(), s.db, interchangeUUID)
	if err != nil {
		return err
	}

	if interchange == nil {
		return writeErrorResponse(r.Context(), w, http.StatusNotFound, "interchange not found", fmt.Errorf("interchange not found"))
	}

	r.ParseForm()
	urn := r.Form.Get("urn")
	if urn == "" {
		return writeErrorResponse(r.Context(), w, http.StatusBadRequest, "missing urn", fmt.Errorf("missing urn field"))
	}

	// if this creating a new association
	if r.Method == http.MethodPost {
		var channel *models.Channel
		channelUUID := r.Form.Get("channel")

		// check that that UUID is in our interchange
		for _, c := range interchange.Channels {
			if c.UUID == channelUUID {
				channel = &c
				break
			}
		}

		if channel == nil {
			return writeErrorResponse(r.Context(), w, http.StatusBadRequest, "channel not found", fmt.Errorf("channel with UUID: %s not found", channelUUID))
		}

		// associate our URN
		err := models.SetChannelForURN(r.Context(), s.db, interchange, channel, urn)
		if err != nil {
			return err
		}

		return writeDataResponse(r.Context(), w, http.StatusOK, "mapping created", nil)
	} else if r.Method == http.MethodDelete {
		err := models.ClearChannelForURN(r.Context(), s.db, interchange, urn)
		if err != nil {
			return err
		}
		return writeDataResponse(r.Context(), w, http.StatusOK, "mapping removed", nil)
	}

	return writeErrorResponse(r.Context(), w, http.StatusMethodNotAllowed, "invalid method", fmt.Errorf("must be POST or DELETE"))
}
