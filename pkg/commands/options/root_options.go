package options

import "github.com/spf13/pflag"

// NewDefaultOptions 创建一个默认运行选项
func NewDefaultOptions() Options {
	return Options{
		Global:     NewDefaultGlobalOptions(),
		ListenAddr: ":80",
	}
}

// Options 命令运行选项
type Options struct {
	// 全局选项
	Global GlobalOptions `json:"global,omitempty" yaml:"global,omitempty"`

	// 监听地址
	ListenAddr string `json:"listenAddr,omitempty" yaml:"listenAddr,omitempty"`
}

// AddFlags 将选项绑定到命令行参数
func (opts *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&opts.ListenAddr, "listen", "l", opts.ListenAddr, "Listen address")
}
