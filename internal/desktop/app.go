package desktop

import (
	"context"
	"sync"
	"sync/atomic"

	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
)

type App struct {
	settingsMu        sync.Mutex
	clearing          atomic.Bool
	ctx               context.Context
	auth              *auth.Manager
	settings          *settings.Store
	configurationErr  error
	backgroundWarning string
	restoreOverview   bool
	latencyMu         sync.Mutex
	latencySequence   uint64
	latencyChecks     map[string]latencyCheck
	latencyTargets    []networkdiag.LatencyTarget
	latencyConfigured bool
	traceSession      diagnosticSession
	pingSession       diagnosticSession
}

func NewApp(restoreOverview bool) *App {
	store, err := settings.NewStore()
	return &App{auth: auth.NewManager(), settings: store, configurationErr: err, restoreOverview: restoreOverview}
}
