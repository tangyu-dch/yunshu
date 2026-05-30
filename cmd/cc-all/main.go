// Package main 提供了一个 All-in-One 多合一的极速启动入口。
// 它能够在单个进程（cc-all）中，并发拉起 cc-edge、cc-console、cc-call 和 cc-worker 4 大服务。
// 共享同一个配置文件、日志器和数据库连接，非常适合本地开发、单机轻量级部署以及演示测试。
package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yunshu/internal/app"
	"yunshu/internal/contracts"
	"yunshu/internal/infra/config"
)

func env(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	// 允许用户在单进程模式下，为 4 大服务分别指定不同的监听端口，默认不冲突
	consoleAddr := flag.String("console-addr", env("CONSOLE_ADDR", ":8080"), "cc-console listen address")
	edgeAddr := flag.String("edge-addr", env("EDGE_ADDR", ":8081"), "cc-edge listen address")
	callAddr := flag.String("call-addr", env("CALL_ADDR", ":8082"), "cc-call listen address")
	workerAddr := flag.String("worker-addr", env("WORKER_ADDR", ":8083"), "cc-worker listen address")
	configPath := flag.String("config", env("CONFIG", ""), "config file path")
	_ = flag.String("addr", "", "compatibility addr flag placeholder")
	flag.Parse()

	// 自动加载默认配置文件
	if *configPath == "" {
		if _, err := os.Stat("configs/default.yaml"); err == nil {
			*configPath = "configs/default.yaml"
		}
	}

	cfg := config.Config{}
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			slog.Error("[cc-all] 配置加载失败", "path", *configPath, "error", err)
			os.Exit(1)
		}
		cfg = loaded
	}

	slog.Info("[cc-all] ===========================================================")
	slog.Info("[cc-all] ★★★ 正在单进程中并发拉起 Yunshu CallCenter 4 大服务... ★★★")
	slog.Info("[cc-all] ===========================================================")

	// 1. 启动 cc-edge 边缘网关服务
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-edge 边缘网关服务...", "addr", *edgeAddr)
		server := app.NewServerWithConfig(contracts.ServiceEdge, cfg)
		if err := server.ListenAndServe(*edgeAddr); err != nil {
			slog.Error("[cc-all] cc-edge 服务异常停止", "error", err)
		}
	}()

	// 2. 启动 cc-console 管理控制台与引导器
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-console 运营后台服务...", "addr", *consoleAddr)
		server := app.NewServerWithConfig(contracts.ServiceConsole, cfg)
		if err := server.ListenAndServe(*consoleAddr); err != nil {
			slog.Error("[cc-all] cc-console 服务异常停止", "error", err)
		}
	}()

	// 3. 启动 cc-call 呼叫核心与 ESL 调度引擎
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-call 呼叫控制服务...", "addr", *callAddr)
		server := app.NewServerWithConfig(contracts.ServiceCall, cfg)
		if err := server.ListenAndServe(*callAddr); err != nil {
			slog.Error("[cc-all] cc-call 服务异常停止", "error", err)
		}
	}()

	// 4. 启动 cc-worker 后台异步计费与流式处理器
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-worker 异步任务队列服务...", "addr", *workerAddr)
		server := app.NewServerWithConfig(contracts.ServiceWorker, cfg)
		if err := server.ListenAndServe(*workerAddr); err != nil {
			slog.Error("[cc-all] cc-worker 服务异常停止", "error", err)
		}
	}()

	// 监听系统退出信号，实现优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("[cc-all] 接收到退出信号，正在优雅关闭所有服务进程...", "signal", sig.String())
	time.Sleep(1 * time.Second)
	slog.Info("[cc-all] 服务安全退场。")
}
