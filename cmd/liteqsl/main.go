// Command liteqsl 是 LiteQSL-Web v2 的单体程序入口。
//
// 用法：
//
//	./liteqsl                      # 使用默认配置启动（config.yaml 可选）
//	./liteqsl -config path.yaml    # 指定配置文件
//	./liteqsl -version             # 打印版本号
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bsxiaocai/LiteQSL-Web/internal/api"
	"github.com/bsxiaocai/LiteQSL-Web/internal/config"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
	"github.com/bsxiaocai/LiteQSL-Web/internal/version"
)

func main() {
	// 子命令：liteqsl reset-password ...
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		os.Exit(runResetPassword(os.Args[2:]))
	}

	configPath := flag.String("config", "config.yaml", "配置文件路径（不存在时使用默认值）")
	showVersion := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.AppVersion)
		return
	}

	logger := newLogger(levelFromString(os.Getenv("LITEQSL_LOG_LEVEL")))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	logger = newLogger(levelFromString(cfg.LogLevel))

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		logger.Error("打开数据库失败", "path", cfg.DBPath, "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Init(db); err != nil {
		logger.Error("初始化数据库失败", "err", err)
		os.Exit(1)
	}

	srv := api.New(cfg, db, logger)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 优雅关闭：监听 SIGINT / SIGTERM。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		logger.Info("收到退出信号，正在关闭服务…")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	logger.Info("服务已启动",
		"addr", addr,
		"db", cfg.DBPath,
		"version", version.AppVersion,
		"log_level", cfg.LogLevel,
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("服务启动失败", "err", err)
		os.Exit(1)
	}
	logger.Info("服务已退出")
}

// newLogger 创建输出到 stdout 的文本 logger。
func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// levelFromString 解析日志级别字符串。
func levelFromString(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
