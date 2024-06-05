# 🍀 Clover 

[![Build Status](https://github.com/nyaruka/rp-clover/workflows/CI/badge.svg)](https://github.com/nyaruka/rp-clover/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/nyaruka/rp-clover/branch/main/graph/badge.svg)](https://codecov.io/gh/nyaruka/rp-clover)

Router for incoming messages to RapidPro/TextIt, takes care of changing contact affinity based on keywords and routing 
incoming messages based on that affinity.

```
Clover takes care of routing RapidPro/TextIt messages based on membership.

Usage of clover:
  -address string
    	the address clover will listen on (default "localhost")
  -db string
    	the connection string for our database (default "postgres://localhost/clover_test?sslmode=disable")
  -debug-conf
    	print where config values are coming from
  -help
    	print usage information
  -log-level string
    	the log level, one of error, warn, info, debug (default "info")
  -password string
    	the password for the admin user (default "sesame123")
  -port int
    	the port clover will listen on (default 8081)
  -sentry-dsn string
    	the sentry configuration to log errors to, if any
  -version string
    	the version being run (default "Dev")

Environment variables:
                              CLOVER_ADDRESS - string
                                   CLOVER_DB - string
                            CLOVER_LOG_LEVEL - string
                             CLOVER_PASSWORD - string
                                 CLOVER_PORT - int
                           CLOVER_SENTRY_DSN - string
                              CLOVER_VERSION - string
```
