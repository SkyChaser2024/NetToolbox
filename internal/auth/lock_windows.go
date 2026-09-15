package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"
)

var ErrOperationBusy = errors.New("另一个网络工具箱进程正在进行认证，请稍后重试")

func acquireOperationLock() (func(), error) {
	path := filepath.Join(os.TempDir(), "network-toolbox-8021x.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	overlapped := &windows.Overlapped{}
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		file.Close()
		return nil, ErrOperationBusy
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
			_ = file.Close()
		})
	}, nil
}

func Run(ctx context.Context, cfg Config, sink EventSink) error {
	release, err := acquireOperationLock()
	if err != nil {
		return err
	}
	defer release()
	session, err := NewSession(cfg, sink)
	if err != nil {
		return err
	}
	defer session.clearCredentials()
	defer session.Close()
	return session.Run(ctx)
}
