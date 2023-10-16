//go:generate vfsgendev -source="statik".Assets
package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq"
	"github.com/nyaruka/ezconf"
	clover "github.com/nyaruka/rp-clover"
	"github.com/rakyll/statik/fs"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry"

	_ "github.com/nyaruka/rp-clover/statik"
)

var version = "Dev"

func main() {
	config := clover.NewConfig()
	loader := ezconf.NewLoader(&config, "clover", "Clover takes care of routing RapidPro messages based on membership.", []string{"clover.toml"})
	loader.MustLoad()

	var level slog.Level
	err := level.UnmarshalText([]byte(config.LogLevel))
	if err != nil {
		log.Fatalf("invalid log level %s", level)
		os.Exit(1)
	}

	// configure our logger
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(logHandler))

	logger := slog.With("comp", "main")
	logger.Info("starting clover", "version", version)

	// if we have a DSN entry, try to initialize it
	if config.SentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:           config.SentryDSN,
			EnableTracing: false,
		})
		if err != nil {
			log.Fatalf("error initiating sentry client, error %s, dsn %s", err, config.SentryDSN)
			os.Exit(1)
		}

		defer sentry.Flush(2 * time.Second)

		logger = slog.New(
			slogmulti.Fanout(
				logHandler,
				slogsentry.Option{Level: slog.LevelError}.NewSentryHandler(),
			),
		)
		logger = logger.With("release", version)
		slog.SetDefault(logger)
	}

	// our settings shouldn't contain a timezone, nothing will work right with this not being a constant UTC
	if strings.Contains(config.DB, "TimeZone") {
		logger.Error("invalid db connection string, do not specify a timezone, archiver always uses UTC", "db", config.DB)
	}

	// force our DB connection to be in UTC
	if strings.Contains(config.DB, "?") {
		config.DB += "&TimeZone=UTC"
	} else {
		config.DB += "?TimeZone=UTC"
	}

	var templateFS http.FileSystem

	// if we have a custom version, use it
	if version != "Dev" {
		config.Version = version
	}

	if config.Version == "Dev" {
		templateFS = http.Dir("static")
	} else {
		templateFS, err = fs.New()
		if err != nil {
			logger.Error("error initializing statik filesystem", "error", err)
		}
	}

	clover := clover.NewServer(config, templateFS)
	logger.Info("starting clover")
	err = clover.Start()
	if err != nil {
		logger.Error("error starting clover", "error", err)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("stopping clover", "signal", <-ch)

	err = clover.Stop()
	if err != nil {
		logger.Error("error stopping clover", "error", err)
	}
}
