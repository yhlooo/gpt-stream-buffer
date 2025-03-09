package commands

import (
	"encoding/json"
	"fmt"

	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yhlooo/gpt-stream-buffer/pkg/commands/options"
	"github.com/yhlooo/gpt-stream-buffer/pkg/servers"
)

// NewCommand 创建命令
func NewCommand() *cobra.Command {
	opts := options.NewDefaultOptions()
	return NewCommandWithOptions(&opts)
}

// NewCommandWithOptions 基于选项创建命令
func NewCommandWithOptions(opts *options.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gpt-stream-buffer",
		Short:        "Proxy and streaming response buffer for ChatGPT compatible APIs",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// 校验全局选项
			if err := opts.Global.Validate(); err != nil {
				return err
			}
			// 设置日志
			logger := setLogger(cmd, opts.Global.Verbosity)
			// 输出选项
			optsRaw, _ := json.Marshal(opts)
			logger.V(1).Info(fmt.Sprintf("command: %q, args: %q, options: %s", cmd.Name(), args, string(optsRaw)))
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger := logr.FromContextOrDiscard(ctx)

			s := servers.NewServer(servers.Options{
				ListenAddr: opts.ListenAddr,
			})

			if err := s.Start(ctx); err != nil {
				return err
			}
			logger.Info(fmt.Sprintf("serve http on %s", s.Address().String()))

			<-s.Done()
			logger.Info("server done")

			return nil
		},
	}

	// 将选项绑定到命令行
	opts.Global.AddFlags(cmd.PersistentFlags())
	opts.AddFlags(cmd.Flags())

	// 添加子命令
	cmd.AddCommand(
		NewVersionCommandWithOptions(&opts.Version),
	)

	return cmd
}

// setLogger 设置命令日志，并返回 logr.Logger
func setLogger(cmd *cobra.Command, verbosity uint32) logr.Logger {
	// 设置日志级别
	logrusLogger := logrus.New()
	switch verbosity {
	case 1:
		logrusLogger.SetLevel(logrus.DebugLevel)
	case 2:
		logrusLogger.SetLevel(logrus.TraceLevel)
	default:
		logrusLogger.SetLevel(logrus.InfoLevel)
	}
	// 将 logger 注入上下文
	logger := logrusr.New(logrusLogger)
	cmd.SetContext(logr.NewContext(cmd.Context(), logger))

	return logger
}
