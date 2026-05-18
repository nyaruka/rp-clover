package runtime

import (
	"github.com/nyaruka/ezconf"
)

// Config is our top level configuration object
type Config struct {
	DB        string `help:"the connection string for our database"`
	LogLevel  string `help:"the log level, one of error, warn, info, debug"`
	SentryDSN string `help:"the sentry configuration to log errors to, if any"`
	Version   string `help:"the version being run"`
	Password  string `help:"the password for the admin user"`
	Address   string `help:"the address clover will listen on"`
	Port      int    `help:"the port clover will listen on"`
}

// NewDefaultConfig returns a new default configuration object
func NewDefaultConfig() *Config {
	return &Config{
		DB:       "postgres://clover_test:temba@localhost/clover_test?sslmode=disable",
		LogLevel: "info",
		Address:  "localhost",
		Port:     8081,
		Version:  "Dev",
		Password: "sesame123",
	}
}

// LoadConfig loads the config from clover.toml + environment + flags via ezconf
func LoadConfig() *Config {
	cfg := NewDefaultConfig()
	loader := ezconf.NewLoader(cfg, "clover", "Clover takes care of routing RapidPro messages based on membership.", []string{"clover.toml"})
	loader.MustLoad()
	return cfg
}
