package clover

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

// NewConfig returns a new default configuration object
func NewConfig() *Config {
	config := Config{
		DB:       "postgres://localhost/clover_test?sslmode=disable",
		LogLevel: "info",
		Address:  "localhost",
		Port:     8081,
		Version:  "Dev",
		Password: "sesame123",
	}

	return &config
}
