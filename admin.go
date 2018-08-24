package clover

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nyaruka/rp-clover/models"
	"github.com/sirupsen/logrus"
)

func newAdminRouter(s *Server) *chi.Mux {
	router := chi.NewRouter()

	router.Use(s.adminOnly)
	router.Method(http.MethodGet, "/", s.newHandlerFunc(viewConfig))
	router.Method(http.MethodPost, "/", s.newHandlerFunc(updateConfig))
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
		logrus.WithError(err).Error("error loading interchange config")
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
	config, err := json.MarshalIndent(interchanges, "", "    ")
	if err != nil {
		return err
	}

	err = r.ParseForm()
	if err != nil {
		return renderInterchanges(s, w, r, config, "", err)
	}

	config = []byte(r.Form.Get("config"))
	logrus.WithField("config", string(config)).Info("received new config")

	// try to create our config
	interchanges = make([]models.Interchange, 0)
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
		logrus.WithError(err).Error("error loading interchange config")
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
	text, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return template.New(name).Parse(string(text))
}
