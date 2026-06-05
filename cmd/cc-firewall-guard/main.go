// Package main 是 cc-firewall-guard 守护进程的入口。
// 负责宿主机网络层的国别 IP 拦截规则动态更新，并异步分析内核日志上报拦截流水。
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/config"
	redisinfra "yunshu/internal/infra/redis"
)

const (
	ipdenyURLPattern = "http://www.ipdeny.com/ipblocks/data/countries/%s.zone"
	cacheDirName     = "configs/ipblocks"
	ipsetSetName     = "blocked_countries"
	ipsetTempName    = "blocked_countries_temp"
)

// IPBlockLogReq Webhook 提交实体
type IPBlockLogReq struct {
	IP          string `json:"ip"`
	CountryCode string `json:"countryCode"`
	CallID      string `json:"callId"`
	Method      string `json:"method"`
}

type IPBlockGuard struct {
	cfg            config.Config
	redisClient    *goredis.Client
	consoleBaseURL string
	logger         *slog.Logger
	isDryRun       bool

	mu            sync.RWMutex
	activeRanges  []IPRangeMap
	lastConfig    string
	onlyAllowCN   bool
	lastOnlyAllow string
}

type IPRangeMap struct {
	CountryCode string
	IPNet       *net.IPNet
}

func main() {
	configPath := flag.String("config", "configs/default.yaml", "配置参数 YAML 文件")
	_ = flag.String("addr", "", "No-op address flag for Makefile compatibility")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	logger.Info("【云枢防火墙卫士】正在启动守护进程...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("加载配置文件失败，正在使用默认值", "path", *configPath, "error", err.Error())
	}

	// 初始化 Redis 客户端
	var redisClient *goredis.Client
	if len(cfg.Redis.Addrs) > 0 {
		redisClient = redisinfra.NewClient(cfg.Redis)
	}

	consoleBase := "http://localhost:8080"
	if cfg.Console.CallBaseURL != "" {
		// 估算 Console 基础地址，一般是 callBaseURL 或者是 localhost:8080
		// 因为 daemon 是同主机部署，可以直接请求控制台地址
		consoleBase = cfg.Console.CallBaseURL
	}

	// 检查是否具备 Linux 的 ipset/iptables 环境和 root 权限
	isDryRun := true
	if _, err := exec.LookPath("ipset"); err == nil {
		if _, err := exec.LookPath("iptables"); err == nil {
			if os.Geteuid() == 0 {
				isDryRun = false
			}
		}
	}

	if isDryRun {
		logger.Warn("【工作模式警告】当前未检测到 Linux 环境、无 ipset/iptables 命令或无 root 权限。cc-firewall-guard 将运行在【模拟与开发测试(Dry-Run)】模式！")
	} else {
		logger.Info("【工作模式正常】检测到 Linux 生产网络环境，已启用内核 ipset + iptables 级物理拦截")
	}

	guard := &IPBlockGuard{
		cfg:            cfg,
		redisClient:    redisClient,
		consoleBaseURL: consoleBase,
		logger:         logger,
		isDryRun:       isDryRun,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动配置同步轮询器
	go guard.configSyncLoop(ctx)

	// 启动拦截日志尾随抓取器
	go guard.logScraperLoop(ctx)

	// 监听退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("【云枢防火墙卫士】收到终止信号，正在优雅关闭...", "signal", sig.String())
	cancel()

	// 退出时清理防火墙规则（如果在 Linux 正常模式）
	if !isDryRun {
		guard.cleanupFirewallRules()
	}
	logger.Info("【云枢防火墙卫士】已完全安全退出")
}

// configSyncLoop 周期性同步 Redis/DB 拦截国家黑名单，并热更新 ipset
func (g *IPBlockGuard) configSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	g.logger.Info("防火墙配置动态同步轮询器已启动")
	g.syncConfig(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.syncConfig(ctx)
		}
	}
}

func (g *IPBlockGuard) syncConfig(ctx context.Context) {
	var configVal string
	var onlyAllowVal string
	var err error

	if g.redisClient != nil {
		configVal, err = g.redisClient.Get(ctx, operate.RedisKeyBlockedCountries).Result()
		if err != nil && err != goredis.Nil {
			g.logger.Error("从 Redis 读取拦截国家列表失败", "error", err.Error())
			return
		}
		onlyAllowVal, err = g.redisClient.Get(ctx, operate.RedisKeyOnlyAllowCN).Result()
		if err != nil && err != goredis.Nil {
			g.logger.Error("从 Redis 读取仅放行中国配置失败", "error", err.Error())
			return
		}
	}

	g.mu.RLock()
	last := g.lastConfig
	lastOnlyAllow := g.lastOnlyAllow
	g.mu.RUnlock()

	if configVal == last && onlyAllowVal == lastOnlyAllow && last != "" && lastOnlyAllow != "" {
		return
	}

	g.logger.Info("监测到拦截配置发生变更或初次加载", "oldBlocked", last, "newBlocked", configVal, "oldOnlyAllow", lastOnlyAllow, "newOnlyAllow", onlyAllowVal)

	onlyAllowCN := (onlyAllowVal == "true")
	var countries []string
	if onlyAllowCN {
		countries = []string{"CN"}
	} else {
		countries = parseCountriesList(configVal)
	}

	ranges, err := g.loadCountryRanges(countries)
	if err != nil {
		g.logger.Error("加载国家网段数据失败", "error", err.Error())
		return
	}

	if onlyAllowCN {
		privateCIDRs := []string{
			"127.0.0.0/8",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"169.254.0.0/16",
			"224.0.0.0/4",
		}
		for _, cidr := range privateCIDRs {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				ranges = append(ranges, IPRangeMap{
					CountryCode: "PRIVATE",
					IPNet:       ipNet,
				})
			}
		}
	}

	g.mu.Lock()
	g.activeRanges = ranges
	g.lastConfig = configVal
	g.onlyAllowCN = onlyAllowCN
	g.lastOnlyAllow = onlyAllowVal
	g.mu.Unlock()

	// 动态更新 Linux ipset
	if !g.isDryRun {
		if err := g.applyIPSetSwap(countries, ranges); err != nil {
			g.logger.Error("应用内核 ipset 规则失败", "error", err.Error())
		} else {
			g.logger.Info("已成功同步最新拦截网段至 Linux ipset 内核集合", "countriesCount", len(countries), "rangesCount", len(ranges))
		}
	} else {
		g.logger.Info("【模拟模式】跳过 ipset/iptables 物理规则应用", "rangesCount", len(ranges))
	}
}

func parseCountriesList(val string) []string {
	parts := strings.Split(val, ",")
	var res []string
	for _, p := range parts {
		trimmed := strings.ToUpper(strings.TrimSpace(p))
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

// loadCountryRanges 下载或加载本地缓存的国别网段，并编译为 in-memory CIDRs
func (g *IPBlockGuard) loadCountryRanges(countries []string) ([]IPRangeMap, error) {
	if err := os.MkdirAll(cacheDirName, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	var allRanges []IPRangeMap

	for _, country := range countries {
		code := strings.ToLower(country)
		cachePath := filepath.Join(cacheDirName, fmt.Sprintf("%s.zone", code))

		// 检查本地是否存在，或者尝试下载
		needDownload := true
		if info, err := os.Stat(cachePath); err == nil {
			// 如果缓存文件生成于 24 小时以内，直接读取，不频繁下载
			if time.Since(info.ModTime()) < 24*time.Hour {
				needDownload = false
			}
		}

		if needDownload {
			g.logger.Info("正在从网络获取最新的国家 IP 聚合网段", "country", country)
			url := fmt.Sprintf(ipdenyURLPattern, code)
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				data, errRead := io.ReadAll(resp.Body)
				if errRead == nil {
					_ = os.WriteFile(cachePath, data, 0644)
					g.logger.Info("已成功写入国家 IP 聚合网段本地缓存", "country", country)
				}
			} else {
				g.logger.Warn("从网络获取 IP 网段失败，尝试读取本地缓存", "country", country, "error", err)
			}
		}

		// 读取文件并解析
		data, err := os.ReadFile(cachePath)
		if err != nil {
			g.logger.Error("无法读取国家网段数据", "country", country, "path", cachePath)
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		count := 0
		for scanner.Scan() {
			cidr := strings.TrimSpace(scanner.Text())
			if cidr == "" || strings.HasPrefix(cidr, "#") {
				continue
			}
			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			allRanges = append(allRanges, IPRangeMap{
				CountryCode: country,
				IPNet:       ipNet,
			})
			count++
		}
		g.logger.Info("已成功解析国家 IP 聚合网段", "country", country, "cidrsCount", count)
	}

	return allRanges, nil
}

// applyIPSetSwap 运用原子级的 ipset 交换机制热更新内核规则
func (g *IPBlockGuard) applyIPSetSwap(countries []string, ranges []IPRangeMap) error {
	// 1. 创建临时集
	cmd := exec.Command("ipset", "create", ipsetTempName, "hash:net", "hashsize", "4096", "maxelem", "1000000")
	if err := cmd.Run(); err != nil {
		// 可能上次残留了，先销毁一下
		_ = exec.Command("ipset", "destroy", ipsetTempName).Run()
		if err := exec.Command("ipset", "create", ipsetTempName, "hash:net", "hashsize", "4096", "maxelem", "1000000").Run(); err != nil {
			return fmt.Errorf("创建临时 ipset 失败: %w", err)
		}
	}

	// 2. 向临时集批量添加网段
	// 为了加速执行，我们可以通过 ipset restore 来批量载入网段而非单条 fork
	var restoreData bytes.Buffer
	restoreData.WriteString(fmt.Sprintf("create %s hash:net hashsize 4096 maxelem 1000000\n", ipsetTempName))
	for _, r := range ranges {
		restoreData.WriteString(fmt.Sprintf("add %s %s\n", ipsetTempName, r.IPNet.String()))
	}
	restoreCmd := exec.Command("ipset", "restore")
	restoreCmd.Stdin = &restoreData
	if err := restoreCmd.Run(); err != nil {
		_ = exec.Command("ipset", "destroy", ipsetTempName).Run()
		return fmt.Errorf("批量写入临时 ipset 失败: %w", err)
	}

	// 3. 确保目标 ipset 存在，如果不存在则创建之
	_ = exec.Command("ipset", "create", ipsetSetName, "hash:net", "hashsize", "4096", "maxelem", "1000000").Run()

	// 4. 原子级 swap
	if err := exec.Command("ipset", "swap", ipsetTempName, ipsetSetName).Run(); err != nil {
		_ = exec.Command("ipset", "destroy", ipsetTempName).Run()
		return fmt.Errorf("交换 ipset 失败: %w", err)
	}

	// 5. 销毁临时集
	_ = exec.Command("ipset", "destroy", ipsetTempName)

	// 6. 安装 iptables 规则确保 LOG 和 DROP
	g.ensureIptablesRules()

	return nil
}

func (g *IPBlockGuard) ensureIptablesRules() {
	g.mu.RLock()
	onlyAllowCN := g.onlyAllowCN
	g.mu.RUnlock()

	// 对 5060 (SIP) 和 5066 (WebSocket) 的 UDP/TCP 流量设置拦截并丢弃规则
	ports := []struct {
		proto string
		port  string
	}{
		{"udp", "5060"},
		{"tcp", "5060"},
		{"tcp", "5066"},
	}

	for _, item := range ports {
		// 1. 黑名单模式的参数：
		blacklistLogArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
		blacklistDropArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "DROP"}

		// 2. 白名单模式的参数：
		whitelistLogArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
		whitelistDropArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "DROP"}

		// 先删除残留以确保纯净
		_ = exec.Command("iptables", blacklistLogArgs...).Run()
		_ = exec.Command("iptables", blacklistDropArgs...).Run()
		_ = exec.Command("iptables", whitelistLogArgs...).Run()
		_ = exec.Command("iptables", whitelistDropArgs...).Run()

		// 3. 安装符合当前模式的规则
		var logArgs, dropArgs []string
		if onlyAllowCN {
			// 白名单模式：若 src IP 不是 allowed 集合，则记录 LOG 并 DROP
			logArgs = []string{"-I", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
			dropArgs = []string{"-I", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "DROP"}
		} else {
			// 黑名单模式：若 src IP 是 blocked 集合，则记录 LOG 并 DROP
			logArgs = []string{"-I", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
			dropArgs = []string{"-I", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "DROP"}
		}

		_ = exec.Command("iptables", logArgs...).Run()
		_ = exec.Command("iptables", dropArgs...).Run()
	}
}

func (g *IPBlockGuard) cleanupFirewallRules() {
	g.logger.Info("清理物理网络层 IP 拦截规则和 ipset 集合...")
	ports := []struct {
		proto string
		port  string
	}{
		{"udp", "5060"},
		{"tcp", "5060"},
		{"tcp", "5066"},
	}

	for _, item := range ports {
		// 删除黑名单拦截规则
		blacklistLogArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
		_ = exec.Command("iptables", blacklistLogArgs...).Run()
		blacklistDropArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "--match-set", ipsetSetName, "src", "-j", "DROP"}
		_ = exec.Command("iptables", blacklistDropArgs...).Run()

		// 删除白名单拦截规则
		whitelistLogArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "LOG", "--log-prefix", "SIP_BLOCK: "}
		_ = exec.Command("iptables", whitelistLogArgs...).Run()
		whitelistDropArgs := []string{"-D", "INPUT", "-p", item.proto, "--dport", item.port, "-m", "set", "!", "--match-set", ipsetSetName, "src", "-j", "DROP"}
		_ = exec.Command("iptables", whitelistDropArgs...).Run()
	}

	// 销毁 ipset 集合
	_ = exec.Command("ipset", "destroy", ipsetSetName)
	g.logger.Info("物理网络层规则清理已完成")
}

// logScraperLoop 抓取内核拦截日志并上报
func (g *IPBlockGuard) logScraperLoop(ctx context.Context) {
	g.logger.Info("内核拦截日志抓取器已启动")

	if g.isDryRun {
		g.logger.Info("【无操作模式】当前环境非支持的 Linux 内核级网络层（如 macOS 开发环境），已跳过拦截日志抓取模拟器。")
		return
	}

	// Linux 生产环境：抓取 journalctl 日志
	cmd := exec.CommandContext(ctx, "journalctl", "-f", "-k", "--grep=SIP_BLOCK:")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		g.logger.Error("无法开启 journalctl 日志流监听", "error", err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		g.logger.Error("无法启动 journalctl 日志监听进程", "error", err.Error())
		return
	}

	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				g.logger.Error("读取内核日志行发生异常", "error", err.Error())
			}
			break
		}

		// 解析形如：SIP_BLOCK: ... SRC=192.0.2.1 ... PROTO=UDP ...
		if !strings.Contains(line, "SIP_BLOCK:") {
			continue
		}

		srcIP := parseField(line, "SRC=")
		if srcIP == "" {
			continue
		}

		// 检查本地 IP 网段匹配出国家代码
		countryCode := g.lookupCountryByIP(srcIP)
		if countryCode == "" {
			countryCode = "UNKNOWN"
		}

		proto := parseField(line, "PROTO=")
		method := "INVITE" // 默认占位
		if strings.Contains(line, "DPT=5066") {
			method = "WS-SIP"
		} else if proto == "UDP" {
			method = "SIP-UDP"
		} else {
			method = "SIP-TCP"
		}

		g.logger.Info("【拦截事件】宿主机防火墙阻断境外攻击 IP", "ip", srcIP, "country", countryCode, "proto", proto)
		g.reportBlockEvent(srcIP, countryCode, method)
	}

	_ = cmd.Wait()
}

func parseField(line, key string) string {
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	tail := line[idx+len(key):]
	end := strings.Index(tail, " ")
	if end < 0 {
		return strings.TrimSpace(tail)
	}
	return tail[:end]
}

func (g *IPBlockGuard) lookupCountryByIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, r := range g.activeRanges {
		if r.IPNet.Contains(ip) {
			return r.CountryCode
		}
	}
	return ""
}

func (g *IPBlockGuard) reportBlockEvent(ip, countryCode, method string) {
	reqBody := IPBlockLogReq{
		IP:          ip,
		CountryCode: countryCode,
		CallID:      fmt.Sprintf("FW-%d", time.Now().UnixNano()),
		Method:      method,
	}

	raw, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/cti/kamailio/ipblock/log", g.consoleBaseURL)

	g.logger.Info("正在上报 IP 拦截审计流水至控制台 Webhook", "url", url, "ip", ip, "country", countryCode)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		g.logger.Error("上报拦截流水失败", "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		g.logger.Error("上报拦截流水返回非 200 响应", "status", resp.Status)
		return
	}

	g.logger.Info("拦截流水上报成功", "ip", ip)
}
