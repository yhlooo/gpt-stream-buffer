package sessions

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/go-logr/logr"
	"github.com/sashabaranov/go-openai"
)

// Metadata 元数据
type Metadata struct {
	Name string `json:"name"`
}

// Session 会话
type Session struct {
	Meta    Metadata       `json:"meta"`
	Options SessionOptions `json:"options"`

	lock       sync.RWMutex
	messages   []Message
	cancelRecv context.CancelFunc
	recvDone   chan struct{}
	buff       *Message
	buffCursor int
}

// SessionOptions 会话选项
type SessionOptions struct {
	// GPT API URL
	BaseURL string `json:"baseURL"`
	// 调用 GPT API 的 Token
	Token string `json:"token"`
	// 使用的模型
	Model string `json:"model"`
	// 初始消息
	InitialMessages []Message `json:"initialMessages"`

	// 频率惩罚
	FrequencyPenalty float32 `json:"frequencyPenalty,omitempty"`
	// 最大生成 token 数
	MaxTokens int `json:"maxTokens,omitempty"`
	// 重复惩罚
	PresencePenalty float32 `json:"presencePenalty,omitempty"`
	// 停止标记
	Stop []string `json:"stop,omitempty"`
	// 温度
	Temperature float32 `json:"temperature"`
	// 输出所取概率分位数
	TopP float32 `json:"topP,omitempty"`
	// 是否输出 token 的对数概率
	Logprobs bool `json:"logprobs,omitempty"`
	// 输出概率由高到低备选 token 数目
	TopLogprobs int `json:"topLogprobs,omitempty"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// SendMessage 发送消息
func (s *Session) SendMessage(ctx context.Context, message *Message) error {
	if message == nil {
		return errors.New("message must not be nil")
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// 停止当前正在进行的接收
	if s.cancelRecv != nil {
		s.cancelRecv()
	}

	// 准备要发送的消息
	if s.messages == nil && len(s.Options.InitialMessages) > 0 {
		s.messages = make([]Message, len(s.Options.InitialMessages))
		copy(s.messages, s.Options.InitialMessages)
	}
	if s.buff != nil && s.buff.Content != "" {
		s.messages = append(s.messages, Message{
			Role:    s.buff.Role,
			Content: s.buff.Content,
		})
	}
	s.messages = append(s.messages, *message)

	// 发送
	s.recvDone = make(chan struct{})
	s.buff = &Message{}
	s.buffCursor = 0
	ctx, s.cancelRecv = context.WithCancel(ctx)
	go s.handleSend(ctx, s.buff, s.recvDone)

	return nil
}

// ReceiveMessage 接收消息
func (s *Session) ReceiveMessage(_ context.Context) (*Message, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.recvDone == nil || s.buff == nil {
		return nil, nil
	}
	msgContent := s.buff.Content[s.buffCursor:]
	if len(msgContent) == 0 {
		// 没有内容
		select {
		case <-s.recvDone:
			// 读完了
			return nil, nil
		default:
		}
	}
	s.buffCursor += len(msgContent)
	return &Message{
		Role:    s.buff.Role,
		Content: msgContent,
	}, nil
}

// handleSend 处理发送消息
func (s *Session) handleSend(ctx context.Context, buff *Message, done chan<- struct{}) {
	defer close(done)

	logger := logr.FromContextOrDiscard(ctx)

	config := openai.DefaultConfig(s.Options.Token)
	config.BaseURL = s.Options.BaseURL
	client := openai.NewClientWithConfig(config)

	s.lock.RLock()
	messages := make([]openai.ChatCompletionMessage, len(s.messages))
	for i, msg := range s.messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
		}
	}
	s.lock.RUnlock()

	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:            s.Options.Model,
		Messages:         messages,
		MaxTokens:        s.Options.MaxTokens,
		Temperature:      s.Options.Temperature,
		TopP:             s.Options.TopP,
		Stream:           true,
		Stop:             s.Options.Stop,
		PresencePenalty:  s.Options.PresencePenalty,
		FrequencyPenalty: s.Options.FrequencyPenalty,
		LogProbs:         s.Options.Logprobs,
		TopLogProbs:      s.Options.TopLogprobs,
	})
	if err != nil {
		logger.Error(err, "creating chat completion stream error")
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.Info("receive done")
				return
			}
			logger.Error(err, "receive error")
			return
		}
		if len(resp.Choices) == 0 {
			continue
		}

		s.lock.Lock()
		if resp.Choices[0].Delta.Role != "" {
			buff.Role = resp.Choices[0].Delta.Role
		}
		buff.Content += resp.Choices[0].Delta.Content
		s.lock.Unlock()
	}
}
