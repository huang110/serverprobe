package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/huang110/serverprobe/agent/internal/config"
	"github.com/huang110/serverprobe/agent/internal/service"
	"github.com/huang110/serverprobe/agent/pkg/logger"
)

var (
	configFile = flag.String("config", "", "配置文件路径")
	version    = flag.Bool("version", false, "显示版本信息")
	liteMode   = flag.Bool("lite", false, "轻量级模式")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("ServerProbe Agent v%s\n", config.Version)
		os.Exit(0)
	}

	// 加载配置
	loader := config.NewLoader(*configFile)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 应用命令行参数
	if *liteMode {
		cfg.Agent.LiteMode = true
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Invalid config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.InitLogger(cfg.Log.Level, cfg.Log.File, cfg.Log.MaxSize); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Infof("ServerProbe Agent v%s starting...", config.Version)
	logger.Infof("Lite mode: %v", cfg.Agent.LiteMode)

	// 创建 Agent 服务
	agent, err := service.NewAgent(cfg, loader)
	if err != nil {
		logger.Fatalf("Failed to create agent: %v", err)
	}

	// 启动 Agent
	if err := agent.Start(); err != nil {
		logger.Fatalf("Failed to start agent: %v", err)
	}

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	agent.Stop()
}
