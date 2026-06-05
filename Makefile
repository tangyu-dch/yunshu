SHELL := /bin/bash

GO ?= go
NPM ?= npm
WEB_DIR ?= web
BIN_DIR ?= bin
PID_DIR ?= run
LOG_DIR ?= logs
ADDR ?= :8080
WEB_HOST ?= 0.0.0.0
WEB_PORT ?= 5173
CONFIG ?= configs/default.yaml

SERVICES := cc-edge cc-console cc-call cc-worker cc-all cc-firewall-guard

.PHONY: help fmt fmt-go test test-go vet build build-go build-web web-build clean deps \
	web-install web-dev web-preview update-contracts \
	run-edge run-console run-call run-worker run-all run-firewall-guard \
	start-edge start-console start-call start-worker start-firewall-guard \
	stop-edge stop-console stop-call stop-worker stop-firewall-guard \
	restart-edge restart-console restart-call restart-worker restart-firewall-guard \
	status-edge status-console status-call status-worker status-firewall-guard \
	start-all stop-all restart-all status-all

help:
	@printf "可用命令:\n"
	@printf "  make fmt             - 格式化 Go 代码\n"
	@printf "  make test            - 运行 Go 全量测试\n"
	@printf "  make vet             - 运行 Go 静态检查\n"
	@printf "  make build           - 构建 Go 后端和前端\n"
	@printf "  make web-build       - 构建前端（等同于 make build 的前端部分）\n"
	@printf "  make run-console     - 启动 cc-console\n"
	@printf "  make run-call        - 启动 cc-call\n"
	@printf "  make run-edge        - 启动 cc-edge\n"
	@printf "  make run-worker      - 启动 cc-worker\n"
	@printf "  make run-firewall-guard - 启动 cc-firewall-guard\n"
	@printf "  make run-all         - 一键启动 All-in-One 多合一单进程服务\n"
	@printf "  make start-console   - 后台启动 cc-console\n"
	@printf "  make stop-console    - 停止 cc-console\n"
	@printf "  make restart-console - 重启 cc-console\n"
	@printf "  make status-console  - 查看 cc-console 状态\n"
	@printf "  make start-all       - 后台启动全部服务\n"
	@printf "  make stop-all        - 停止全部服务\n"
	@printf "  make web-dev         - 启动前端开发服务器\n"
	@printf "  make update-contracts - 重新生成 contracts 摘要\n"

fmt: fmt-go

fmt-go:
	@gofmt -w .

test: test-go

test-go:
	@$(GO) test ./...

vet:
	@$(GO) vet ./...

build: build-go build-web

build-go:
	@mkdir -p $(BIN_DIR)
	@for service in $(SERVICES); do \
		$(GO) build -o $(BIN_DIR)/$$service ./cmd/$$service; \
	done

build-web:
	@cd $(WEB_DIR) && $(NPM) run build

web-build: build-web

clean:
	@rm -rf $(BIN_DIR)
	@rm -rf $(WEB_DIR)/dist
	@rm -rf $(PID_DIR)
	@rm -rf $(LOG_DIR)

deps:
	@$(GO) mod download
	@cd $(WEB_DIR) && $(NPM) install

web-install:
	@cd $(WEB_DIR) && $(NPM) install

web-dev:
	@cd $(WEB_DIR) && $(NPM) run dev -- --host $(WEB_HOST) --port $(WEB_PORT)

web-preview:
	@cd $(WEB_DIR) && $(NPM) run preview -- --host $(WEB_HOST) --port $(WEB_PORT)

update-contracts:
	@$(GO) run ./cmd/update-agents

define run_service
	@if [ -n "$(CONFIG)" ]; then \
		$(GO) run ./cmd/$(1) -addr $(2) -config $(CONFIG); \
	else \
		$(GO) run ./cmd/$(1) -addr $(2); \
	fi
endef

define start_service
	@mkdir -p $(PID_DIR) $(LOG_DIR)
	@pidfile="$(PID_DIR)/$(1).pid"; \
	logfile="$(LOG_DIR)/$(1).log"; \
	if [ -f "$$pidfile" ] && kill -0 "$$(cat $$pidfile)" 2>/dev/null; then \
		echo "$(1) 已在运行，pid=$$(cat $$pidfile)"; \
		exit 0; \
	fi; \
	if [ -n "$(CONFIG)" ]; then \
		nohup $(BIN_DIR)/$(1) -addr $(2) -config $(CONFIG) > "$$logfile" 2>&1 & \
	else \
		nohup $(BIN_DIR)/$(1) -addr $(2) > "$$logfile" 2>&1 & \
	fi; \
	echo $$! > "$$pidfile"; \
	echo "$(1) 已启动，pid=$$(cat $$pidfile) log=$$logfile"
endef

define start_console_service
	@mkdir -p $(PID_DIR) $(LOG_DIR)
	@pidfile="$(PID_DIR)/$(1).pid"; \
	logfile="$(LOG_DIR)/$(1).log"; \
	if [ -f "$$pidfile" ] && kill -0 "$$(cat $$pidfile)" 2>/dev/null; then \
		echo "$(1) 已在运行，pid=$$(cat $$pidfile)"; \
		exit 0; \
	fi; \
	if [ -n "$(CONFIG)" ]; then \
		nohup env CC_CALL_BASE_URL="http://localhost:8082" $(BIN_DIR)/$(1) -addr $(2) -config $(CONFIG) > "$$logfile" 2>&1 & \
	else \
		nohup env CC_CALL_BASE_URL="http://localhost:8082" $(BIN_DIR)/$(1) -addr $(2) > "$$logfile" 2>&1 & \
	fi; \
	echo $$! > "$$pidfile"; \
	echo "$(1) 已启动，pid=$$(cat $$pidfile) log=$$logfile"
endef

define run_console_service
	@if [ -n "$(CONFIG)" ]; then \
		env CC_CALL_BASE_URL="http://localhost:8082" $(GO) run ./cmd/$(1) -addr $(2) -config $(CONFIG); \
	else \
		env CC_CALL_BASE_URL="http://localhost:8082" $(GO) run ./cmd/$(1) -addr $(2); \
	fi
endef

define stop_service
	@pidfile="$(PID_DIR)/$(1).pid"; \
	if [ ! -f "$$pidfile" ]; then \
		echo "$(1) 未运行"; \
		exit 0; \
	fi; \
	pid="$$(cat $$pidfile)"; \
	if kill -0 "$$pid" 2>/dev/null; then \
		kill "$$pid"; \
		echo "$(1) 已停止，pid=$$pid"; \
	else \
		echo "$(1) 进程不存在，清理残留 pidfile"; \
	fi; \
	rm -f "$$pidfile"
endef

define status_service
	@pidfile="$(PID_DIR)/$(1).pid"; \
	if [ -f "$$pidfile" ] && kill -0 "$$(cat $$pidfile)" 2>/dev/null; then \
		echo "$(1) 运行中，pid=$$(cat $$pidfile)"; \
	else \
		echo "$(1) 未运行"; \
	fi
endef

run-edge:
	$(call run_service,cc-edge,:8081)

run-console:
	$(call run_console_service,cc-console,:8080)

run-call:
	$(call run_service,cc-call,:8082)

run-worker:
	$(call run_service,cc-worker,:8083)

run-firewall-guard:
	$(call run_service,cc-firewall-guard,)

run-all:
	@if [ -n "$(CONFIG)" ]; then \
		$(GO) run ./cmd/cc-all -config $(CONFIG); \
	else \
		$(GO) run ./cmd/cc-all; \
	fi

start-edge: build-go
	$(call start_service,cc-edge,:8081)

start-console: build-go
	$(call start_console_service,cc-console,:8080)

start-call: build-go
	$(call start_service,cc-call,:8082)

start-worker: build-go
	$(call start_service,cc-worker,:8083)

start-firewall-guard: build-go
	$(call start_service,cc-firewall-guard,)

start-cc-all: build-go
	$(call start_service,cc-all,)

stop-edge:
	$(call stop_service,cc-edge)

stop-console:
	$(call stop_service,cc-console)

stop-call:
	$(call stop_service,cc-call)

stop-worker:
	$(call stop_service,cc-worker)

stop-firewall-guard:
	$(call stop_service,cc-firewall-guard)

stop-cc-all:
	$(call stop_service,cc-all)

restart-edge: stop-edge start-edge

restart-console: stop-console start-console

restart-call: stop-call start-call

restart-worker: stop-worker start-worker

restart-firewall-guard: stop-firewall-guard start-firewall-guard

restart-cc-all: stop-cc-all start-cc-all

status-edge:
	$(call status_service,cc-edge)

status-console:
	$(call status_service,cc-console)

status-call:
	$(call status_service,cc-call)

status-worker:
	$(call status_service,cc-worker)

status-firewall-guard:
	$(call status_service,cc-firewall-guard)

status-cc-all:
	$(call status_service,cc-all)

start-all: build-go
	$(call start_service,cc-edge,:8081)
	$(call start_console_service,cc-console,:8080)
	$(call start_service,cc-call,:8082)
	$(call start_service,cc-worker,:8083)
	$(call start_service,cc-firewall-guard,)

stop-all:
	$(call stop_service,cc-firewall-guard)
	$(call stop_service,cc-worker)
	$(call stop_service,cc-call)
	$(call stop_service,cc-console)
	$(call stop_service,cc-edge)

restart-all: stop-all start-all

status-all:
	$(call status_service,cc-edge)
	$(call status_service,cc-console)
	$(call status_service,cc-call)
	$(call status_service,cc-worker)
	$(call status_service,cc-firewall-guard)
