package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Manager struct {
	mu         sync.Mutex
	session    *Session
	cancel     context.CancelFunc
	state      State
	lastConfig Config
}

func NewManager() *Manager {
	return &Manager{state: StateIdle}
}

func (m *Manager) Start(cfg Config, sink EventSink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return errors.New("认证任务已在运行")
	}
	release, err := acquireOperationLock()
	if err != nil {
		return err
	}
	var session *Session
	wrappedSink := func(event Event) { m.forwardEvent(session, sink, event) }
	session, err = NewSession(cfg, wrappedSink)
	if err != nil {
		release()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	logoutConfig := logoffConfig(cfg)
	m.session = session
	m.cancel = cancel
	m.state = StateStarting
	go func(current *Session, logout Config) {
		defer release()
		err := current.Run(ctx)
		current.clearCredentials()
		current.Close()
		reportError := false
		m.mu.Lock()
		if m.session == current {
			m.session = nil
			m.cancel = nil
			if errors.Is(err, ErrAttemptsExhausted) {
				m.state = StateFailed
			} else if err != nil {
				m.state = StateError
				reportError = true
			} else if m.state == StateAuthenticated {
				m.lastConfig = logout
			} else {
				m.state = StateIdle
			}
		}
		m.mu.Unlock()
		if reportError && sink != nil {
			sink(Event{State: StateError, Level: "error", Message: err.Error(), Timestamp: time.Now().Format("15:04:05")})
		}
	}(session, logoutConfig)
	return nil
}

func (m *Manager) forwardEvent(session *Session, sink EventSink, event Event) {
	m.mu.Lock()
	if m.session != session {
		m.mu.Unlock()
		return
	}
	m.state = event.State
	m.mu.Unlock()
	if sink != nil {
		sink(event)
	}
}

func (m *Manager) Stop(sendLogoff bool) error {
	m.mu.Lock()
	session, cancel := m.session, m.cancel
	if session == nil {
		state, cfg := m.state, m.lastConfig
		if sendLogoff {
			m.state = StateIdle
			m.lastConfig = Config{}
		}
		m.mu.Unlock()
		if sendLogoff && state == StateAuthenticated {
			return SendLogoff(cfg.DeviceName, cfg.LocalMAC)
		}
		return nil
	}
	m.state = StateStopping
	m.session = nil
	m.cancel = nil
	m.mu.Unlock()

	var err error
	if sendLogoff {
		err = session.Logoff()
	}
	cancel()
	session.Close()
	m.mu.Lock()
	m.state = StateIdle
	if sendLogoff {
		m.lastConfig = Config{}
	}
	m.mu.Unlock()
	return err
}

func (m *Manager) MarkAuthenticated(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		m.state = StateAuthenticated
		m.lastConfig = logoffConfig(cfg)
	}
}

func logoffConfig(cfg Config) Config {
	return Config{DeviceName: cfg.DeviceName, LocalMAC: cfg.LocalMAC}
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session != nil
}
