module github.com/example/coffeeshop

go 1.22.0

require (
	github.com/pflow-xyz/petri-pilot v0.1.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/holiman/uint256 v1.3.2
	github.com/mattn/go-sqlite3 v1.14.24
	github.com/prometheus/client_golang v1.20.5
	golang.org/x/oauth2 v0.24.0
)

// For local development, set PETRI_PILOT_LOCAL_PATH environment variable
// or add: replace github.com/pflow-xyz/petri-pilot => /path/to/petri-pilot
