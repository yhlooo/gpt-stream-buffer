package sessions

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Manager 会话管理器
type Manager interface {
	// Create 创建会话
	Create(ctx context.Context, opts SessionOptions) (*Session, error)
	// Delete 删除会话
	Delete(ctx context.Context, name string) error
	// Get 获取会话
	Get(ctx context.Context, name string) (*Session, error)
}

// NewManager 创建会话管理器
func NewManager() Manager {
	return &defaultManager{
		sessions: make(map[string]*Session),
	}
}

var (
	ErrSessionNotFound = errors.New("SessionNotFound")
)

// defaultManager 是 Manager 的默认实现
type defaultManager struct {
	lock     sync.RWMutex
	sessions map[string]*Session
}

var _ Manager = (*defaultManager)(nil)

// Create 创建会话
func (mgr *defaultManager) Create(_ context.Context, opts SessionOptions) (*Session, error) {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	name := uuid.New().String()
	session := &Session{
		Meta:    Metadata{Name: name},
		Options: opts,
	}
	mgr.sessions[name] = session

	return session, nil
}

// Delete 删除会话
func (mgr *defaultManager) Delete(_ context.Context, name string) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	session, ok := mgr.sessions[name]
	if !ok {
		return fmt.Errorf("%w: session %q not found", ErrSessionNotFound, name)
	}
	delete(mgr.sessions, name)

	// 停止接收
	session.lock.RLock()
	defer session.lock.RUnlock()
	if session.cancelRecv != nil {
		session.cancelRecv()
	}

	return nil
}

// Get 获取会话
func (mgr *defaultManager) Get(_ context.Context, name string) (*Session, error) {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()

	session, ok := mgr.sessions[name]
	if !ok {
		return nil, fmt.Errorf("%w: session %q not found", ErrSessionNotFound, name)
	}
	return session, nil
}
