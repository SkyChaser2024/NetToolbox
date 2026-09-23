// Package automonitor owns the lifetime of a single cancellable authentication
// worker. Saved preferences and a user's temporary pause are separate state.
package automonitor

import (
	"context"
	"sync"
	"sync/atomic"
)

type State uint32

const (
	Stopped State = iota
	Disabled
	Monitoring
	Paused
	WaitingCredentials
	WaitingLink
	Online
	Confirming
	Authenticating
	Exhausted
	ConfigurationError
	AdapterError
	PasswordError
	ForegroundBusy
	WaitingNetwork
)

func (s State) String() string {
	switch s {
	case Disabled:
		return "自动认证已关闭"
	case Monitoring:
		return "正在监测有线网络"
	case Paused:
		return "手动注销后已暂停自动认证"
	case WaitingCredentials:
		return "等待有效账号配置"
	case WaitingLink:
		return "等待有线网卡连接"
	case Online:
		return "有线网络已连接"
	case Confirming:
		return "正在确认断线状态"
	case Authenticating:
		return "正在自动认证（最多 3 次）"
	case Exhausted:
		return "本次断线已停止重试，可重新启动后台后重试"
	case ConfigurationError:
		return "配置读取失败"
	case AdapterError:
		return "无法读取或找到可认证的有线网卡"
	case PasswordError:
		return "无法读取保存的密码"
	case ForegroundBusy:
		return "主界面正在认证"
	case WaitingNetwork:
		return "认证成功，等待网络就绪"
	default:
		return "自动认证后台未运行"
	}
}

type Config struct {
	Enabled bool
	Key     string
}

type Controller struct {
	mu      sync.Mutex
	config  Config
	paused  bool
	cancel  context.CancelFunc
	done    chan struct{}
	state   atomic.Uint32
	worker  func(context.Context, func(State))
	onState func(State)
}

func New(worker func(context.Context, func(State)), onState func(State)) *Controller {
	return &Controller{worker: worker, onState: onState}
}

func (c *Controller) State() State { return State(c.state.Load()) }

func (c *Controller) publish(state State) {
	c.state.Store(uint32(state))
	if c.onState != nil {
		c.onState(state)
	}
}

func (c *Controller) drain() {
	if c.cancel != nil {
		c.cancel()
		<-c.done
		c.cancel = nil
	}
}

func (c *Controller) start() {
	if !c.config.Enabled {
		c.publish(Disabled)
		return
	}
	if c.paused {
		c.publish(Paused)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel, c.done = cancel, make(chan struct{})
	done := c.done
	c.publish(Monitoring)
	go func() {
		defer close(done)
		c.worker(ctx, c.publish)
	}()
}

func (c *Controller) Reload(config Config, force bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.config == config && c.State() != Stopped {
		return
	}
	c.drain()
	c.config = config
	c.start()
}

// Pause waits until the worker releases its authentication resources. Callers
// can safely send a manual logoff only after this method returns.
func (c *Controller) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drain()
	c.paused = true
	if c.config.Enabled {
		c.publish(Paused)
	} else {
		c.publish(Disabled)
	}
}

func (c *Controller) Resume(config Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drain()
	c.paused = false
	c.config = config
	c.start()
}

func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drain()
	c.publish(Stopped)
}
