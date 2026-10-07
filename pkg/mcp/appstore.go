package mcp

// Shared appstore.Store accessor for the MCP tools. Lazily opened at
// appstore.DefaultPath() (overridable via PETRI_APPSTORE_DB) on first use,
// exactly once per process — matching pkg/mcp/service.go's ~/.petri-pilot
// convention for the sibling services state.
//
// Tests must never touch that path: setAppStoreForTest swaps in a
// temp-file/in-memory store for the duration of a test and restores the
// previous value afterward, which is what lets pkg/mcp's tests exercise
// persistence without risking the real database.

import (
	"sync"

	"github.com/pflow-xyz/petri-pilot/pkg/appstore"
)

var (
	appStoreOnce sync.Once
	appStore     *appstore.Store
	appStoreErr  error
	appStoreMu   sync.Mutex
)

// getAppStore returns the process-wide appstore.Store, opening it on first
// use.
func getAppStore() (*appstore.Store, error) {
	appStoreMu.Lock()
	override := appStore
	appStoreMu.Unlock()
	if override != nil {
		return override, nil
	}

	appStoreOnce.Do(func() {
		s, err := appstore.Open(appstore.DefaultPath())
		appStoreMu.Lock()
		appStore, appStoreErr = s, err
		appStoreMu.Unlock()
	})

	appStoreMu.Lock()
	defer appStoreMu.Unlock()
	return appStore, appStoreErr
}

// setAppStoreForTest overrides the process-wide store for the duration of a
// test and returns a restore function. Using this (rather than the
// PETRI_APPSTORE_DB env var, which only takes effect before first use) is
// what keeps every test isolated even though getAppStore's sync.Once has
// already fired for the test binary.
func setAppStoreForTest(s *appstore.Store) (restore func()) {
	appStoreMu.Lock()
	prev := appStore
	appStore = s
	appStoreMu.Unlock()
	return func() {
		appStoreMu.Lock()
		appStore = prev
		appStoreMu.Unlock()
	}
}
