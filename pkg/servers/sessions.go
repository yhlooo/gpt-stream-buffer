package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/google/uuid"

	"github.com/yhlooo/gpt-stream-buffer/pkg/sessions"
)

// NewSessionServer 创建会话服务
func NewSessionServer(mgr sessions.Manager) *SessionServer {
	return &SessionServer{mgr: mgr}
}

// SessionServer 会话服务
type SessionServer struct {
	mgr sessions.Manager
}

// Register 注册路由
func (s *SessionServer) Register(ctx context.Context, mux *http.ServeMux) {
	logger := logr.FromContextOrDiscard(ctx)
	mux.HandleFunc("POST /v1/sessions", handlerWithLogger(logger, s.handleCreateSession))
	mux.HandleFunc("DELETE /v1/sessions/{name}", handlerWithLogger(logger, s.handleDeleteSession))
	mux.HandleFunc("POST /v1/sessions/{name}/messages", handlerWithLogger(logger, s.handleCreateSessionMessage))
	mux.HandleFunc(
		"GET /v1/sessions/{name}/bufferedmessages",
		handlerWithLogger(logger, s.handleGetSessionBufferedMessage),
	)
}

// CreateSession 创建会话
func (s *SessionServer) CreateSession(ctx context.Context, session *sessions.Session) (*sessions.Session, error) {
	ret, err := s.mgr.Create(ctx, session.Options)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// DeleteSession 删除会话
func (s *SessionServer) DeleteSession(ctx context.Context, name string) error {
	return s.mgr.Delete(ctx, name)
}

// CreateSessionMessage 推送消息到会话
func (s *SessionServer) CreateSessionMessage(ctx context.Context, name string, msg *sessions.Message) error {
	session, err := s.mgr.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := session.SendMessage(ctx, msg); err != nil {
		return err
	}
	return nil
}

// BufferedMessage 缓冲的消息
type BufferedMessage struct {
	// 增量内容
	Delta *sessions.Message `json:"delta,omitempty"`
	// 是否所有消息已接收完成
	Done bool `json:"done,omitempty"`
}

// GetSessionBufferedMessage 获取会话缓冲区的消息
func (s *SessionServer) GetSessionBufferedMessage(ctx context.Context, name string) (*BufferedMessage, error) {
	session, err := s.mgr.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	msg, err := session.ReceiveMessage(ctx)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return &BufferedMessage{Done: true}, nil
	}
	return &BufferedMessage{Delta: msg}, nil
}

// SimpleResponse 简单响应
type SimpleResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// handleCreateSession 处理创建会话请求
func (s *SessionServer) handleCreateSession(w http.ResponseWriter, req *http.Request) {
	session := &sessions.Session{}
	if !handleReadJSON(w, req, session) {
		return
	}

	session, err := s.CreateSession(req.Context(), session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// handleDeleteSession 处理删除会话请求
func (s *SessionServer) handleDeleteSession(w http.ResponseWriter, req *http.Request) {
	sessionName := req.PathValue("name")
	if sessionName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("session name is required"))
		return
	}

	if err := s.DeleteSession(req.Context(), sessionName); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, SimpleResponse{Code: 200, Message: "Ok"})
}

// handleCreateSessionMessage 处理创建会话消息请求
func (s *SessionServer) handleCreateSessionMessage(w http.ResponseWriter, req *http.Request) {
	sessionName := req.PathValue("name")
	if sessionName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("session name is required"))
		return
	}

	msg := &sessions.Message{}
	if !handleReadJSON(w, req, msg) {
		return
	}

	if err := s.CreateSessionMessage(req.Context(), sessionName, msg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, SimpleResponse{Code: 200, Message: "Ok"})
}

// handleGetSessionBufferedMessage 处理获取缓冲区消息请求
func (s *SessionServer) handleGetSessionBufferedMessage(w http.ResponseWriter, req *http.Request) {
	sessionName := req.PathValue("name")
	if sessionName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("session name is required"))
		return
	}

	msg, err := s.GetSessionBufferedMessage(req.Context(), sessionName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

// handlerWithLogger 注入 logr.Logger 的 http.HandlerFunc
func handlerWithLogger(logger logr.Logger, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req = req.WithContext(logr.NewContext(req.Context(), logger.WithValues("reqID", uuid.New().String())))
		handler(w, req)
	}
}

// handleReadJSON 处理读 JSON 请求体
func handleReadJSON(w http.ResponseWriter, req *http.Request, v interface{}) bool {
	logger := logr.FromContextOrDiscard(req.Context())

	bodyRaw, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		logger.Error(err, "read body error")
		writeError(w, http.StatusInternalServerError, err)
		return false
	}
	if err := json.Unmarshal(bodyRaw, v); err != nil {
		logger.Error(err, fmt.Sprintf("unmarshal body from json error, body: %s", string(bodyRaw)))
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// writeError
func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, SimpleResponse{
		Code:    code,
		Message: err.Error(),
	})
}

// writeJSON 写 JSON 响应
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.WriteHeader(code)
	bodyRaw, _ := json.Marshal(v)
	_, _ = w.Write(bodyRaw)
}
