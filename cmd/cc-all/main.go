// Package main 提供了一个 All-in-One 多合一的极速启动入口。
// 它能够在单个进程（cc-all）中，并发拉起 cc-edge、cc-console、cc-call 和 cc-worker 4 大服务。
// 共享同一个配置文件、日志器和数据库连接，非常适合本地开发、单机轻量级部署以及演示测试。
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("[cc-all] ===========================================================")
	slog.Info("[cc-all] ★★★ 正在单进程中并发拉起 Yunshu CallCenter 4 大服务... ★★★")
	slog.Info("[cc-all] ===========================================================")

	var servers []*app.Server
	var serversMu sync.Mutex

	// 1. 启动 cc-edge 边缘网关服务
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-edge 边缘网关服务...", "addr", *edgeAddr)
		server, err := app.NewServerWithConfig(contracts.ServiceEdge, cfg)
		if err != nil {
			slog.Error("[cc-all] cc-edge 服务初始化失败", "error", err)
			return
		}
		serversMu.Lock()
		servers = append(servers, server)
		serversMu.Unlock()
		if err := server.ListenAndServe(ctx, *edgeAddr); err != nil {
			slog.Error("[cc-all] cc-edge 服务异常停止", "error", err)
		}
	}()

	// 2. 启动 cc-console 运营后台服务
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-console 运营后台服务...", "addr", *consoleAddr)
		server, err := app.NewServerWithConfig(contracts.ServiceConsole, cfg)
		if err != nil {
			slog.Error("[cc-all] cc-console 服务初始化失败", "error", err)
			return
		}
		serversMu.Lock()
		servers = append(servers, server)
		serversMu.Unlock()
		if err := server.ListenAndServe(ctx, *consoleAddr); err != nil {
			slog.Error("[cc-all] cc-console 服务异常停止", "error", err)
		}
	}()

	// 3. 启动 cc-call 呼叫核心与 ESL 调度引擎
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-call 呼叫控制服务...", "addr", *callAddr)
		server, err := app.NewServerWithConfig(contracts.ServiceCall, cfg)
		if err != nil {
			slog.Error("[cc-all] cc-call 服务初始化失败", "error", err)
			return
		}
		serversMu.Lock()
		servers = append(servers, server)
		serversMu.Unlock()
		if err := server.ListenAndServe(ctx, *callAddr); err != nil {
			slog.Error("[cc-all] cc-call 服务异常停止", "error", err)
		}
	}()

	// 4. 启动 cc-worker 后台异步计费与流式处理器
	go func() {
		slog.Info("[cc-all] 正在拉起 cc-worker 异步任务队列服务...", "addr", *workerAddr)
		server, err := app.NewServerWithConfig(contracts.ServiceWorker, cfg)
		if err != nil {
			slog.Error("[cc-all] cc-worker 服务初始化失败", "error", err)
			return
		}
		serversMu.Lock()
		servers = append(servers, server)
		serversMu.Unlock()
		if err := server.ListenAndServe(ctx, *workerAddr); err != nil {
			slog.Error("[cc-all] cc-worker 服务异常停止", "error", err)
		}
	}()

	// 监听系统退出信号，等候上下文终止
	<-ctx.Done()

	slog.Info("[cc-all] 接收到退出信号，正在优雅关闭所有服务进程...")

	// 限制优雅关闭最大等待时间为 15 秒
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	serversMu.Lock()
	for _, server := range servers {
		server.Shutdown(shutdownCtx)
	}
	serversMu.Unlock()

	slog.Info("[cc-all] 所有子服务资源已安全释放。")
	time.Sleep(1 * time.Second)
	slog.Info("[cc-all] 服务安全退场。")
}
