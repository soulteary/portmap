// Copyright 2026 soulteary
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command portmap 实现等价于
//
//	sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222
//
// 的端口转发。默认使用纯 Go 实现，也可通过 -mode socat 直接调用系统 socat。
// 支持 TCP/UDP、并发限流、空闲超时与连接级日志。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/forward"
	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/socat"
)

// 以下变量通过 -ldflags "-X main.version=... -X main.commit=... -X main.date=..." 注入。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type options struct {
	listenPort  int
	listenHost  string
	target      string
	mode        string
	proto       string
	reuseAddr   bool
	useSudo     bool
	dialTimeout time.Duration
	maxConns    int
	idleTimeout time.Duration
	logLevel    string
	quiet       bool
	showVersion bool
	configPath  string
	lang        string
}

// String 返回最终生效配置的可读摘要，用于启动时打印，便于确认合并后的实际参数。
func (o options) String() string {
	listen := net.JoinHostPort(o.listenHost, strconv.Itoa(o.listenPort))
	return fmt.Sprintf(
		"mode=%s proto=%s listen=%s target=%s reuseaddr=%t dial-timeout=%s max-conns=%d idle-timeout=%s log-level=%s quiet=%t sudo=%t",
		o.mode, o.proto, listen, o.target, o.reuseAddr, o.dialTimeout, o.maxConns, o.idleTimeout, o.logLevel, o.quiet, o.useSudo,
	)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(argv []string) error {
	var opt options

	// 语言在首次调用 T() 时按系统环境自动检测（见 i18n.Detect）。
	// 若命令行显式指定 -lang，则预扫描一次并覆盖，使 --help 与 flag
	// 描述也使用指定语言。
	if code := preScanLang(argv); code != "" {
		if l, ok := i18n.ParseLang(code); ok {
			i18n.SetLang(l)
		}
	}

	fs := flag.NewFlagSet("portmap", flag.ContinueOnError)
	fs.IntVar(&opt.listenPort, "listen-port", 22, i18n.T(i18n.KeyFlagListenPort))
	fs.StringVar(&opt.listenHost, "listen-host", "", i18n.T(i18n.KeyFlagListenHost))
	fs.StringVar(&opt.target, "target", "127.0.0.1:2222", i18n.T(i18n.KeyFlagTarget))
	fs.StringVar(&opt.mode, "mode", "go", i18n.T(i18n.KeyFlagMode))
	fs.StringVar(&opt.proto, "proto", "tcp", i18n.T(i18n.KeyFlagProto))
	fs.BoolVar(&opt.reuseAddr, "reuseaddr", true, i18n.T(i18n.KeyFlagReuseAddr))
	fs.BoolVar(&opt.useSudo, "sudo", false, i18n.T(i18n.KeyFlagSudo))
	fs.DurationVar(&opt.dialTimeout, "dial-timeout", 10*time.Second, i18n.T(i18n.KeyFlagDialTimeout))
	fs.IntVar(&opt.maxConns, "max-conns", 0, i18n.T(i18n.KeyFlagMaxConns))
	fs.DurationVar(&opt.idleTimeout, "idle-timeout", 0, i18n.T(i18n.KeyFlagIdleTimeout))
	fs.StringVar(&opt.logLevel, "log-level", "info", i18n.T(i18n.KeyFlagLogLevel))
	fs.BoolVar(&opt.quiet, "quiet", false, i18n.T(i18n.KeyFlagQuiet))
	fs.BoolVar(&opt.showVersion, "version", false, i18n.T(i18n.KeyFlagVersion))
	fs.StringVar(&opt.configPath, "config", "", i18n.T(i18n.KeyFlagConfig))
	fs.StringVar(&opt.lang, "lang", "", i18n.T(i18n.KeyFlagLang, strings.Join(i18n.Codes(), "/")))

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T(i18n.KeyUsageTitle))
		fmt.Fprintln(os.Stderr, i18n.T(i18n.KeyUsageLine, "portmap"))
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if len(argv) == 0 {
		fs.Usage()
		return nil
	}

	// -lang 已在 preScanLang 中生效；此处不再重复处理。

	if opt.showVersion {
		fmt.Println(i18n.T(i18n.KeyVersionLine, version, commit, date))
		return nil
	}

	// 记录用户显式设置过哪些 flag：既用于配置文件合并（flag 优先），
	// 也用于 socat 模式判断是否使用了仅 go 模式支持的选项。
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// 加载配置文件并合并：仅对配置文件中出现、且命令行未显式设置的字段生效。
	// 优先级：命令行显式 flag > 配置文件 > 内置默认值。
	if opt.configPath != "" {
		cfg, err := loadConfig(opt.configPath)
		if err != nil {
			return err
		}
		if err := mergeConfig(&opt, cfg, setFlags); err != nil {
			return err
		}
		// 若语言来自配置文件（命令行未显式 -lang，但配置文件提供了 lang），
		// 在此补一次 SetLang，使运行期消息（日志/错误）使用该语言。
		// 注意：--help 与 flag 描述在 flag 解析前已定稿，配置文件无法改变其语言。
		if !setFlags["lang"] && opt.lang != "" {
			if l, ok := i18n.ParseLang(opt.lang); ok {
				i18n.SetLang(l)
			}
		}
	}

	if opt.listenPort <= 0 || opt.listenPort > 65535 {
		return errors.New(i18n.T(i18n.KeyErrListenPort, opt.listenPort))
	}
	if strings.TrimSpace(opt.target) == "" {
		return errors.New(i18n.T(i18n.KeyErrTargetEmpty))
	}
	proto := strings.ToLower(strings.TrimSpace(opt.proto))
	if proto != "tcp" && proto != "udp" {
		return errors.New(i18n.T(i18n.KeyErrProto, opt.proto))
	}
	opt.proto = proto

	if opt.idleTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrIdleNeg, opt.idleTimeout))
	}
	if opt.maxConns < 0 {
		return errors.New(i18n.T(i18n.KeyErrMaxConnsNeg, opt.maxConns))
	}
	if opt.dialTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrDialNeg, opt.dialTimeout))
	}
	logLevel := strings.ToLower(strings.TrimSpace(opt.logLevel))
	if logLevel != "info" && logLevel != "debug" {
		return errors.New(i18n.T(i18n.KeyErrLogLevel, opt.logLevel))
	}
	opt.logLevel = logLevel

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch opt.mode {
	case "go":
		return runGo(ctx, opt)
	case "socat":
		return runSocat(ctx, opt, setFlags)
	default:
		return errors.New(i18n.T(i18n.KeyErrMode, opt.mode))
	}
}

// preScanLang 在正式解析 flag 前，从 argv 里粗略读取 -lang/--lang 的值，
// 以便 --help 与 flag 描述也能使用用户指定的语言。支持 "-lang zh" 与
// "-lang=zh" 两种写法。未找到时返回空串。
func preScanLang(argv []string) string {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			break
		}
		name := strings.TrimLeft(a, "-")
		if name == "lang" {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		if v, ok := strings.CutPrefix(name, "lang="); ok {
			return v
		}
	}
	return ""
}

func runGo(ctx context.Context, opt options) error {
	log.Print(i18n.T(i18n.KeyLogEffectiveConfig, opt))
	listen := net.JoinHostPort(opt.listenHost, strconv.Itoa(opt.listenPort))
	srv := forward.New(forward.Config{
		Listen:      listen,
		Target:      opt.target,
		Network:     opt.proto,
		ReuseAddr:   opt.reuseAddr,
		DialTimeout: opt.dialTimeout,
		MaxConns:    opt.maxConns,
		IdleTimeout: opt.idleTimeout,
		Debug:       strings.EqualFold(opt.logLevel, "debug"),
		Quiet:       opt.quiet,
	})
	watchStatusSignal(ctx, srv)
	if err := srv.ListenAndServe(ctx); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrServeExit), err)
	}
	return nil
}

func runSocat(ctx context.Context, opt options, setFlags map[string]bool) error {
	log.Print(i18n.T(i18n.KeyLogEffectiveConfig, opt))
	// socat 模式仅复用系统 socat，以下仅 go 模式支持的 flag 若被显式设置则提示忽略。
	goOnly := []string{"idle-timeout", "max-conns", "log-level", "quiet"}
	var ignored []string
	for _, name := range goOnly {
		if setFlags[name] {
			ignored = append(ignored, "-"+name)
		}
	}
	if len(ignored) > 0 {
		log.Print(i18n.T(i18n.KeyLogSocatIgnore, strings.Join(ignored, " ")))
	}

	socatOpt := socat.Options{
		ListenPort: opt.listenPort,
		Target:     opt.target,
		Proto:      opt.proto,
		Fork:       true,
		ReuseAddr:  opt.reuseAddr,
		ListenHost: opt.listenHost,
		Sudo:       opt.useSudo,
	}
	log.Print(i18n.T(i18n.KeyLogSocatExec, socatOpt.String()))
	if err := socatOpt.Run(ctx); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrSocatFailed), err)
	}
	return nil
}
