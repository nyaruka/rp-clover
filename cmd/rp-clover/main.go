//go:generate vfsgendev -source="statik".Assets
package main

import (
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/evalphobia/logrus_sentry"
	_ "github.com/lib/pq"
	"github.com/nyaruka/ezconf"
	clover "github.com/nyaruka/rp-clover"
	"github.com/rakyll/statik/fs"
	log "github.com/sirupsen/logrus"

	_ "github.com/nyaruka/rp-clover/statik"
)

func main() {
	config := clover.NewConfig()
	loader := ezconf.NewLoader(&config, "clover", "Clover takes care of routing RapidPro messages based on membership.", []string{"clover.toml"})
	loader.MustLoad()

	// configure our logger
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{})

	level, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		log.Fatalf("Invalid log level '%s'", level)
	}
	log.SetLevel(level)

	// if we have a DSN entry, try to initialize it
	if config.SentryDSN != "" {
		hook, err := logrus_sentry.NewSentryHook(config.SentryDSN, []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel})
		hook.Timeout = 0
		hook.StacktraceConfiguration.Enable = true
		hook.StacktraceConfiguration.Skip = 4
		hook.StacktraceConfiguration.Context = 5
		if err != nil {
			log.WithError(err).Fatalf("invalid sentry DSN: '%s'", config.SentryDSN)
		}
		log.StandardLogger().Hooks.Add(hook)
	}

	// our settings shouldn't contain a timezone, nothing will work right with this not being a constant UTC
	if strings.Contains(config.DB, "TimeZone") {
		log.WithField("db", config.DB).Fatalf("invalid db connection string, do not specify a timezone, archiver always uses UTC")
	}

	// force our DB connection to be in UTC
	if strings.Contains(config.DB, "?") {
		config.DB += "&TimeZone=UTC"
	} else {
		config.DB += "?TimeZone=UTC"
	}

	var templateFS http.FileSystem

	if config.Version == "Dev" {
		templateFS = http.Dir("static")
	} else {
		templateFS, err = fs.New()
		if err != nil {
			log.WithError(err).Fatal("error initializing statik filesystem")
		}
	}

	clover := clover.NewServer(config, templateFS)
	log.Info("starting clover")
	err = clover.Start()
	if err != nil {
		log.WithError(err).Fatal("error starting clover")
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.WithField("signal", <-ch).Info("stopping clover")

	err = clover.Stop()
	if err != nil {
		log.WithError(err).Fatal("error stopping clover")
	}
}
