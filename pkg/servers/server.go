package servers

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/yhlooo/gpt-stream-buffer/pkg/sessions"
)

// Options 服务选项
type Options struct {
	ListenAddr string
}

// NewServer 创建 Server
func NewServer(opts Options) *Server {
	return &Server{opts: opts}
}

// Server HTTP 服务
type Server struct {
	opts Options

	done <-chan struct{}

	listener net.Listener
	mux      *http.ServeMux
}

// Start 开始提供服务
func (s *Server) Start(ctx context.Context) error {
	// 注册路由
	mux := http.NewServeMux()
	sessionServer := NewSessionServer(sessions.NewManager())
	sessionServer.Register(ctx, mux)

	// 监听端口
	var err error
	s.listener, err = net.Listen("tcp", s.opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %q error: %w", s.opts.ListenAddr, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s.done = ctx.Done()

	// 开始 http 服务
	go func() {
		defer cancel()
		if err := http.Serve(s.listener, mux); err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "http serve error")
		}
	}()

	return nil
}

// Done 返回服务结束通知通道
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// Address 获取服务端监听地址
func (s *Server) Address() net.Addr {
	return s.listener.Addr()
}
