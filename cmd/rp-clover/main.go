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
	clover "github.com/nyaruka/rp-clover"
	"github.com/nyaruka/rp-clover/runtime"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry/v2"
)

var version = "Dev"

func main() {
	cfg := runtime.LoadConfig()

	var level slog.Level
	err := level.UnmarshalText([]byte(cfg.LogLevel))
	if err != nil {
		log.Fatalf("invalid log level %s", level)
	}

	// configure our logger
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(logHandler))

	logger := slog.With("comp", "main")
	logger.Info("starting clover", "version", version)

	// if we have a DSN entry, try to initialize it
	if cfg.SentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:           cfg.SentryDSN,
			EnableTracing: false,
		})
		if err != nil {
			log.Fatalf("error initiating sentry client, error %s, dsn %s", err, cfg.SentryDSN)
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

	// require an admin password so the admin interface is never exposed with a default
	if cfg.Password == "" {
		logger.Error("no admin password set, set CLOVER_PASSWORD")
		os.Exit(1)
	}

	// our settings shouldn't contain a timezone, nothing will work right with this not being a constant UTC
	if strings.Contains(cfg.DB, "TimeZone") {
		logger.Error("invalid db connection string, do not specify a timezone, clover always uses UTC")
		os.Exit(1)
	}

	// force our DB connection to be in UTC
	if strings.Contains(cfg.DB, "?") {
		cfg.DB += "&TimeZone=UTC"
	} else {
		cfg.DB += "?TimeZone=UTC"
	}

	// if we have a custom version, use it
	if version != "Dev" {
		cfg.Version = version
	}

	var templateFS http.FileSystem
	if cfg.Version == "Dev" {
		templateFS = http.Dir("static")
	} else {
		templateFS = clover.Assets()
	}

	rt, err := runtime.NewRuntime(cfg)
	if err != nil {
		logger.Error("error creating runtime", "error", err)
		os.Exit(1)
	}

	srv := clover.NewServer(rt, templateFS)
	logger.Info("starting clover")
	err = srv.Start()
	if err != nil {
		logger.Error("error starting clover", "error", err)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("stopping clover", "signal", <-ch)

	err = srv.Stop()
	if err != nil {
		logger.Error("error stopping clover", "error", err)
	}
}
