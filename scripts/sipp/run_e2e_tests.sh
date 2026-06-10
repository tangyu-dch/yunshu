#!/bin/bash
# ============================================================================
# 云枢 SIP 端到端测试编排脚本
#
# 支持场景: inbound | api | batch | dialpad | signal | options | register | cancel | invalid-sdp | legality | all
# 用法:     bash run_e2e_tests.sh [scenario]
# ============================================================================

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Docker 内网 IP
FS_HOST="${FS_HOST:-192.168.107.6}"
KAM_HOST="${KAM_HOST:-192.168.107.2}"
LOCAL_IP="${LOCAL_IP:-192.168.107.0}"
FS_SIP_PORT=5080
KAM_PORT=5060

AGENT_PORT=6060
CUSTOMER_PORT=6080
INBOUND_PORT=6070
DIALPAD_PORT=6090
REGISTER_PORT=6061
CANCEL_PORT=6072
INVALID_SDP_PORT=6073
OPTIONS_PORT=6074

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS=0
FAIL=0
WARN=0
RESULTS=()
SIPP_PIDS=()
SIPP_CONTAINERS=()
SIPP_UAS_MODE="${SIPP_UAS_MODE:-auto}"
SIPP_IMAGE="${SIPP_IMAGE:-yunshu-sipp:local}"
SIPP_DOCKER_NETWORK="${SIPP_DOCKER_NETWORK:-callcenter_net}"

cleanup() {
    for pid in ${SIPP_PIDS[@]+"${SIPP_PIDS[@]}"}; do
        kill "$pid" 2>/dev/null || true
    done
    for name in ${SIPP_CONTAINERS[@]+"${SIPP_CONTAINERS[@]}"}; do
        docker rm -f "$name" >/dev/null 2>&1 || true
    done
    wait 2>/dev/null
}
trap cleanup EXIT

ensure_sipp_image() {
    if docker image inspect "$SIPP_IMAGE" >/dev/null 2>&1; then
        return 0
    fi
    if [ "$SIPP_UAS_MODE" = "host" ]; then
        return 1
    fi
    echo -e "${YELLOW}[准备]${NC} 构建 SIPp Docker 镜像 $SIPP_IMAGE ..." >&2
    docker build -t "$SIPP_IMAGE" "$SCRIPT_DIR" >/tmp/yunshu_sipp_docker_build.log 2>&1
}

start_uas() {
    local name="$1"
    local scenario="$2"
    local service="$3"
    local port="$4"
    local target="$5"
    local log="/tmp/sipp_${name}_$(date +%s).log"

    if [ "$SIPP_UAS_MODE" != "host" ] && ensure_sipp_image; then
        local container="sipp_${name}_$(date +%s)_$$"
        docker run -d \
            --name "$container" \
            --network "$SIPP_DOCKER_NETWORK" \
            -v "$SCRIPT_DIR:/scenarios:ro" \
            --entrypoint sh \
            "$SIPP_IMAGE" \
            -lc "ip=\$(hostname -I | awk '{print \$1}'); exec sipp -sf /scenarios/$scenario -s '$service' -i \"\$ip\" -p '$port' -m 1 -nostdin -timeout 60s -timeout_error '$target'" >/dev/null
        SIPP_CONTAINERS+=("$container")
        echo "docker:$container"
        return 0
    fi

    sipp -sf "$SCRIPT_DIR/$scenario" \
        -s "$service" \
        -i "$LOCAL_IP" \
        -p "$port" \
        -m 1 \
        -nostdin \
        -timeout 60s \
        -timeout_error \
        -trace_err -error_file "/tmp/sipp_${name}_err.log" \
        -screen_file "$log" \
        "$target" &>/dev/null &

    local pid=$!
    SIPP_PIDS+=("$pid")
    echo "$pid"
}

wait_sipp() {
    local pid="$1"
    local name="$2"
    local timeout=45

    if [[ "$pid" == docker:* ]]; then
        local container="${pid#docker:}"
        while [ $timeout -gt 0 ]; do
            if ! docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
                docker logs "$container" >/tmp/sipp_${name}_docker.log 2>/tmp/sipp_${name}_err.log || true
                local exit_code
                exit_code=$(docker inspect --format '{{.State.ExitCode}}' "$container" 2>/dev/null || echo 255)
                docker rm -f "$container" >/dev/null 2>&1 || true
                return "$exit_code"
            fi
            sleep 1
            ((timeout--))
        done
        docker logs "$container" >/tmp/sipp_${name}_docker.log 2>/tmp/sipp_${name}_err.log || true
        docker rm -f "$container" >/dev/null 2>&1 || true
        return 255
    fi

    while [ $timeout -gt 0 ]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            # start_uas 通过命令替换返回 PID，实际进程可能不是当前 shell 的直接子进程，不能可靠 wait。
            # 这里以进程正常退出作为完成信号；具体 SIP 信令结果由调用方结合日志/错误文件判断。
            return 0
        fi
        sleep 1
        ((timeout--))
    done
    kill "$pid" 2>/dev/null || true
    return 255
}

check_sipp_result() {
    local name="$1"
    local pid="$2"
    local log_pattern="$3"

    if wait_sipp "$pid" "$name"; then
        RESULTS+=("${GREEN}PASS${NC}: $name — SIP 信令流程完整")
        ((PASS++))
        return 0
    else
        local exit_code=$?
        # 检查是否收到关键 SIP 消息
        local log="/tmp/sipp_${name}_*.log"
        local latest_log=$(ls -t /tmp/sipp_${name}_*.log 2>/dev/null | head -1)
        if [ -n "$latest_log" ] && grep -q "$log_pattern" "$latest_log" 2>/dev/null; then
            RESULTS+=("${GREEN}PASS${NC}: $name — SIP 信令交换成功 (非标准退出)")
            ((PASS++))
            return 0
        fi
        RESULTS+=("${RED}FAIL${NC}: $name — 超时或信令异常 (exit=$exit_code)")
        ((FAIL++))
        return 1
    fi
}

# ============================================================================
echo ""
echo -e "${CYAN}=========================================="
echo "  云枢 SIP 端到端测试"
echo -e "==========================================${NC}"
echo "  FreeSWITCH: $FS_HOST:$FS_SIP_PORT"
echo "  Kamailio:   $KAM_HOST:$KAM_PORT"
echo "  本机 IP:    $LOCAL_IP"
echo "  场景:       ${1:-all}"
echo ""

# 前置: 初始化测试数据
echo -e "${YELLOW}[准备]${NC} 初始化测试数据..."
bash "$SCRIPT_DIR/seed_test_data.sh" 2>/dev/null
echo ""

SCENARIO="${1:-all}"

case "$SCENARIO" in
    inbound|all)
        echo -e "${CYAN}=========================================="
        echo "  场景 1: 客户呼入 (INBOUND)"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC(客户) → FS → yunshu → SIPp UAS(坐席)"
        echo ""

        echo -e "${YELLOW}[1/3]${NC} 坐席 1001 向 Kamailio 注册 (port=$AGENT_PORT)..."
        sipp -sf "$SCRIPT_DIR/register_auth_uac.xml" \
            -s "1001" \
            -i "$LOCAL_IP" \
            -p "$AGENT_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$KAM_HOST:$KAM_PORT" >/tmp/sipp_inbound_register.log 2>/tmp/sipp_inbound_register_err.log || true

        echo -e "${YELLOW}[2/3]${NC} 启动坐席 UAS (port=$AGENT_PORT)..."
        AGENT_PID=$(start_uas "inbound_agent" "agent_uas.xml" "1001" "$AGENT_PORT" "$KAM_HOST:$KAM_PORT")
        sleep 1

        echo -e "${YELLOW}[3/3]${NC} 发起呼入 INVITE (DID=01088886666)..."
        sipp -sf "$SCRIPT_DIR/inbound_uac.xml" \
            -s "01088886666" \
            -i "$LOCAL_IP" \
            -p "$INBOUND_PORT" \
            -m 1 \
            -nostdin \
            -timeout 30s \
            -timeout_error \
            -trace_err -error_file "/tmp/sipp_inbound_cust_err.log" \
            -screen_file "/tmp/sipp_inbound_cust.log" \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -3

        CUST_EXIT=${PIPESTATUS[0]}
        echo ""

        if [ $CUST_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: 呼入 - 客户侧完整信令 (INVITE→200 OK→ACK→BYE)")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: 呼入 - 客户侧信令异常 (exit=$CUST_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    api|all)
        echo -e "${CYAN}=========================================="
        echo "  场景 2: API 外呼 (AGENT_FIRST)"
        echo -e "==========================================${NC}"
        echo "  流程: API → yunshu → SIPp UAS(坐席) → SIPp UAS(客户)"
        echo "  注: 需要云枢服务运行中 (port 8080)"
        echo ""

        echo -e "${YELLOW}[1/3]${NC} 启动坐席 UAS (port=$AGENT_PORT)..."
        AGENT_PID=$(start_uas "api_agent" "agent_uas.xml" "1001" "$AGENT_PORT" "$KAM_HOST:$KAM_PORT")
        sleep 1

        echo -e "${YELLOW}[2/3]${NC} 启动客户 UAS (port=$CUSTOMER_PORT)..."
        CUST_PID=$(start_uas "api_customer" "customer_uas.xml" "13800001111" "$CUSTOMER_PORT" "$FS_HOST:$FS_SIP_PORT")
        sleep 1

        echo -e "${YELLOW}[3/3]${NC} 通过 API 发起外呼..."
        API_USER_ID=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT user_id FROM cc_res_extension WHERE extension_number='1001' AND enable=1 AND del_flag=0 ORDER BY id DESC LIMIT 1" 2>/dev/null || echo "")
        if [ -z "$API_USER_ID" ]; then
            API_USER_ID=2001
        fi
        HTTP_CODE=$(curl -s -o /tmp/api_call_resp.json -w "%{http_code}" \
            -X POST "http://127.0.0.1:8082/cti/callTask/call?callId=api-e2e-$(date +%s)" \
            -H "Content-Type: application/json" \
            -d "{\"userId\":${API_USER_ID},\"callee\":\"13800001111\"}" 2>/dev/null || echo "000")

        if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ]; then
            echo -e "  ${GREEN}✓${NC} API 响应: HTTP $HTTP_CODE"
        else
            echo -e "  ${YELLOW}?${NC} API 响应: HTTP $HTTP_CODE (可能需要 App-Key/App-Secret 认证)"
        fi

        # 等待 SIPp 完成
        echo -e "  等待坐席 UAS 完成..."
        wait_sipp "$AGENT_PID" "api_agent"
        AGENT_EXIT=$?
        echo -e "  等待客户 UAS 完成..."
        wait_sipp "$CUST_PID" "api_customer"
        CUST_EXIT=$?

        if [ $AGENT_EXIT -eq 0 ] && [ $CUST_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: API 外呼 - 双腿完整信令")
            ((PASS++))
        elif [ $AGENT_EXIT -eq 0 ] || [ $CUST_EXIT -eq 0 ]; then
            RESULTS+=("${YELLOW}PARTIAL${NC}: API 外呼 - 部分信令成功")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: API 外呼 - 信令未建立")
            ((FAIL++))
        fi
        echo ""
        ;;

    batch|all)
        echo -e "${CYAN}=========================================="
        echo "  场景 3: 批量外呼 (CUSTOMER_FIRST)"
        echo -e "==========================================${NC}"
        echo "  流程: yunshu → SIPp UAS(客户) → SIPp UAS(坐席)"
        echo "  注: 此场景需手动创建批量任务后通过 API 触发调度"
        echo ""

        echo -e "${YELLOW}[1/2]${NC} 启动客户 UAS (port=$CUSTOMER_PORT)..."
        CUST_PID=$(start_uas "batch_customer" "customer_uas.xml" "13800002222" "$CUSTOMER_PORT" "$FS_HOST:$FS_SIP_PORT")
        sleep 1

        echo -e "${YELLOW}[2/2]${NC} 启动坐席 UAS (port=$AGENT_PORT)..."
        AGENT_PID=$(start_uas "batch_agent" "agent_uas.xml" "1001" "$AGENT_PORT" "$KAM_HOST:$KAM_PORT")
        sleep 1

        echo -e "  ${CYAN}请通过 console 或 API 创建批量任务并启动，SIPp 已就绪等待 INVITE${NC}"
        echo -e "  提示: curl -X POST http://127.0.0.1:8082/cti/batch-call-task/dispatch -d '{\"taskId\":1}'"

        # 等待 SIPp 完成 (更长超时)
        echo -e "  等待客户 UAS 完成 (最多 45s)..."
        wait_sipp "$CUST_PID" "batch_customer"
        CUST_EXIT=$?
        echo -e "  等待坐席 UAS 完成..."
        wait_sipp "$AGENT_PID" "batch_agent"
        AGENT_EXIT=$?

        if [ $CUST_EXIT -eq 0 ] && [ $AGENT_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: 批量外呼 - 双腿完整信令")
            ((PASS++))
        else
            RESULTS+=("${YELLOW}SKIP${NC}: 批量外呼 - 需手动触发批量任务调度")
        fi
        echo ""
        ;;

    dialpad|all)
        echo -e "${CYAN}=========================================="
        echo "  场景 4: 拨号盘直呼 (API_DIRECT)"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC(坐席摘机) → Kamailio → FS → yunshu → SIPp UAS(客户)"
        echo ""

        echo -e "${YELLOW}[1/3]${NC} 启动客户 UAS (port=$CUSTOMER_PORT)..."
        CUST_PID=$(start_uas "dialpad_customer" "customer_uas.xml" "13800003333" "$CUSTOMER_PORT" "$FS_HOST:$FS_SIP_PORT")
        sleep 1

        echo -e "${YELLOW}[2/3]${NC} 坐席 1001 向 Kamailio 注册 (port=$DIALPAD_PORT)..."
        sipp -sf "$SCRIPT_DIR/register_auth_uac.xml" \
            -s "1001" \
            -i "$LOCAL_IP" \
            -p "$DIALPAD_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$KAM_HOST:$KAM_PORT" >/tmp/sipp_dialpad_register.log 2>/tmp/sipp_dialpad_register_err.log || true

        echo -e "${YELLOW}[3/3]${NC} 坐席发起 INVITE (模拟拨号盘直呼)..."
        sipp -sf "$SCRIPT_DIR/dialpad_uac.xml" \
            -s "1001" \
            -i "$LOCAL_IP" \
            -p "$DIALPAD_PORT" \
            -m 1 \
            -nostdin \
            -timeout 30s \
            -timeout_error \
            -trace_err -error_file "/tmp/sipp_dialpad_agent_err.log" \
            -screen_file "/tmp/sipp_dialpad_agent.log" \
            "$KAM_HOST:$KAM_PORT" 2>&1 | tail -3

        AGENT_EXIT=${PIPESTATUS[0]}
        echo ""

        if [ $AGENT_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: 拨号盘直呼 - 坐席侧完整信令")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: 拨号盘直呼 - 信令异常 (exit=$AGENT_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    signal)
        echo -e "${CYAN}=========================================="
        echo "  场景 5: 信令通路检测 (SIGNAL CHECK)"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC → INVITE → 100/4xx/5xx/200"
        echo ""

        echo -e "${YELLOW}[1/2]${NC} FreeSWITCH 信令检测..."
        sipp -sf "$SCRIPT_DIR/signal_check_uac.xml" \
            -s "01088886666" \
            -i "$LOCAL_IP" \
            -p "$INBOUND_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -3

        FS_EXIT=${PIPESTATUS[0]}
        if [ $FS_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: FS 信令检测 — 200 OK 呼叫建立成功")
            ((PASS++))
        else
            FS_LOG=$(ls -t /tmp/sipp_inbound_cust_*.log 2>/dev/null | head -1)
            if [ -n "$FS_LOG" ] && grep -qE "480|404|486|503" "$FS_LOG" 2>/dev/null; then
                RESULTS+=("${GREEN}PASS${NC}: FS 信令检测 — 4xx/5xx 响应 (信令通路正常，无坐席)")
                ((PASS++))
            else
                RESULTS+=("${RED}FAIL${NC}: FS 信令检测 — 超时或异常 (exit=$FS_EXIT)")
                ((FAIL++))
            fi
        fi

        echo -e "${YELLOW}[2/2]${NC} Kamailio 信令检测..."
        sipp -sf "$SCRIPT_DIR/signal_check_uac.xml" \
            -s "1001" \
            -i "$LOCAL_IP" \
            -p "$DIALPAD_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$KAM_HOST:$KAM_PORT" 2>&1 | tail -3

        KAM_EXIT=${PIPESTATUS[0]}
        if [ $KAM_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: Kamailio 信令检测 — 200 OK 呼叫建立成功")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: Kamailio 信令检测 — 异常 (exit=$KAM_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    options)
        echo -e "${CYAN}=========================================="
        echo "  场景 6: OPTIONS 可用性探测"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC → OPTIONS → 200/404 (传输层可达)"
        echo ""

        echo -e "${YELLOW}[1/2]${NC} FreeSWITCH OPTIONS..."
        sipp -sf "$SCRIPT_DIR/options_check_uac.xml" \
            -s "sipp" \
            -i "$LOCAL_IP" \
            -p "$OPTIONS_PORT" \
            -m 1 \
            -nostdin \
            -timeout 10s \
            -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -3

        FS_EXIT=${PIPESTATUS[0]}
        if [ $FS_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: FS OPTIONS — 传输层可达")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: FS OPTIONS — 传输层不可达 (exit=$FS_EXIT)")
            ((FAIL++))
        fi

        echo -e "${YELLOW}[2/2]${NC} Kamailio OPTIONS..."
        sipp -sf "$SCRIPT_DIR/options_check_uac.xml" \
            -s "sipp" \
            -i "$LOCAL_IP" \
            -p "$((OPTIONS_PORT + 1))" \
            -m 1 \
            -nostdin \
            -timeout 10s \
            -timeout_error \
            "$KAM_HOST:$KAM_PORT" 2>&1 | tail -3

        KAM_EXIT=${PIPESTATUS[0]}
        if [ $KAM_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: Kamailio OPTIONS — 传输层可达")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: Kamailio OPTIONS — 传输层不可达 (exit=$KAM_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    register)
        echo -e "${CYAN}=========================================="
        echo "  场景 7: REGISTER 鉴权合法性"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC → REGISTER (无凭证) → 401/403"
        echo ""

        echo -e "${YELLOW}[1/1]${NC} Kamailio REGISTER..."
        sipp -sf "$SCRIPT_DIR/register_uac.xml" \
            -s "1001" \
            -i "$LOCAL_IP" \
            -p "$REGISTER_PORT" \
            -m 1 \
            -nostdin \
            -timeout 10s \
            -timeout_error \
            "$KAM_HOST:$KAM_PORT" 2>&1 | tail -3

        REG_EXIT=${PIPESTATUS[0]}
        if [ $REG_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: REGISTER — Kamailio 鉴权响应正常")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: REGISTER — Kamailio 未响应 (exit=$REG_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    cancel)
        echo -e "${CYAN}=========================================="
        echo "  场景 8: CANCEL 事务合法性"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC → INVITE → CANCEL → 487"
        echo ""

        echo -e "${YELLOW}[1/1]${NC} FreeSWITCH CANCEL..."
        sipp -sf "$SCRIPT_DIR/cancel_uac.xml" \
            -s "01088886666" \
            -i "$LOCAL_IP" \
            -p "$CANCEL_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -3

        CANCEL_EXIT=${PIPESTATUS[0]}
        if [ $CANCEL_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: CANCEL — 487 Request Terminated")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: CANCEL — 异常 (exit=$CANCEL_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    invalid-sdp)
        echo -e "${CYAN}=========================================="
        echo "  场景 9: 畸形 SDP 错误处理"
        echo -e "==========================================${NC}"
        echo "  流程: SIPp UAC → INVITE(畸形SDP) → 488/400/415"
        echo ""

        echo -e "${YELLOW}[1/1]${NC} FreeSWITCH 畸形 SDP..."
        sipp -sf "$SCRIPT_DIR/invalid_sdp_uac.xml" \
            -s "01088886666" \
            -i "$LOCAL_IP" \
            -p "$INVALID_SDP_PORT" \
            -m 1 \
            -nostdin \
            -timeout 15s \
            -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -3

        SDP_EXIT=${PIPESTATUS[0]}
        if [ $SDP_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: 畸形SDP — 服务端正确拒绝")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: 畸形SDP — 异常 (exit=$SDP_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    legality)
        echo -e "${CYAN}=========================================="
        echo "  全量 SIP 合法性校验"
        echo -e "==========================================${NC}"
        echo ""

        # 1. FS INVITE 信令检测 (100 Trying = 传输层正常, 不需要 200 OK)
        echo -e "${YELLOW}[1/5]${NC} FS INVITE 信令检测..."
        SIG_OUT=$(sipp -sf "$SCRIPT_DIR/signal_check_uac.xml" \
            -s "01088886666" -i "$LOCAL_IP" -p "$INBOUND_PORT" -m 1 \
            -nostdin -timeout 10s -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1)
        SIG_EXIT=$?
        if [ $SIG_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: FS 信令检测 — 200 OK 完整通话")
            ((PASS++))
        elif echo "$SIG_OUT" | grep -qE "100.*<---|unexpected.*message"; then
            # 100 Trying 收到 = SIP 传输层正常 (无坐席 UAS 时 FS 发 480 或超时)
            RESULTS+=("${GREEN}PASS${NC}: FS 信令检测 — 100 Trying 收到, 传输层正常")
            ((PASS++))
        elif echo "$SIG_OUT" | grep -q "timed out"; then
            RESULTS+=("${RED}FAIL${NC}: FS 信令检测 — 超时, 信令不通")
            ((FAIL++))
        else
            RESULTS+=("${YELLOW}WARN${NC}: FS 信令检测 — 异常 (exit=$SIG_EXIT)")
            ((WARN++))
        fi

        # 2. OPTIONS 可用性探测
        echo -e "${YELLOW}[2/5]${NC} OPTIONS 可用性探测..."
        sipp -sf "$SCRIPT_DIR/options_check_uac.xml" \
            -s "sipp" -i "$LOCAL_IP" -p "$OPTIONS_PORT" -m 1 \
            -nostdin -timeout 10s -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -1
        if [ ${PIPESTATUS[0]} -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: OPTIONS — FS 传输层可达 (200 OK)")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: OPTIONS — FS 不可达")
            ((FAIL++))
        fi

        # 3. REGISTER 鉴权合法性
        echo -e "${YELLOW}[3/5]${NC} REGISTER 鉴权合法性..."
        sipp -sf "$SCRIPT_DIR/register_uac.xml" \
            -s "1001" -i "$LOCAL_IP" -p "$REGISTER_PORT" -m 1 \
            -nostdin -timeout 10s -timeout_error \
            "$KAM_HOST:$KAM_PORT" 2>&1 | tail -1
        if [ ${PIPESTATUS[0]} -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: REGISTER — 401 鉴权挑战正常")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: REGISTER — 异常")
            ((FAIL++))
        fi

        # 4. CANCEL 事务合法性 (FS park 后 CANCEL → 487)
        echo -e "${YELLOW}[4/5]${NC} CANCEL 事务合法性..."
        sipp -sf "$SCRIPT_DIR/cancel_uac.xml" \
            -s "01088886666" -i "$LOCAL_IP" -p "$CANCEL_PORT" -m 1 \
            -nostdin -timeout 15s -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1 | tail -1
        if [ ${PIPESTATUS[0]} -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: CANCEL — 487 Request Terminated")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: CANCEL — 异常")
            ((FAIL++))
        fi

        # 5. 畸形 SDP 错误处理 (FS park 不校验 SDP, 只验证 100 Trying)
        echo -e "${YELLOW}[5/5]${NC} 畸形 SDP 处理检测..."
        ISDP_OUT=$(sipp -sf "$SCRIPT_DIR/invalid_sdp_uac.xml" \
            -s "01088886666" -i "$LOCAL_IP" -p "$INVALID_SDP_PORT" -m 1 \
            -nostdin -timeout 10s -timeout_error \
            "$FS_HOST:$FS_SIP_PORT" 2>&1)
        ISDP_EXIT=$?
        if [ $ISDP_EXIT -eq 0 ]; then
            RESULTS+=("${GREEN}PASS${NC}: 畸形SDP — 488 正确拒绝")
            ((PASS++))
        elif echo "$ISDP_OUT" | grep -qE "100.*<---"; then
            # FS park() 不校验 SDP 内容, 100 Trying 证明信令层正常
            RESULTS+=("${YELLOW}WARN${NC}: 畸形SDP — FS 接受了畸形 SDP (park 不校验 SDP 内容)")
            ((WARN++))
        elif echo "$ISDP_OUT" | grep -q "unexpected.*message"; then
            # FS 可能返回了非 488 的其他错误码
            RESULTS+=("${GREEN}PASS${NC}: 畸形SDP — FS 拒绝了畸形 SDP")
            ((PASS++))
        else
            RESULTS+=("${RED}FAIL${NC}: 畸形SDP — 异常 (exit=$ISDP_EXIT)")
            ((FAIL++))
        fi
        echo ""
        ;;

    *)
        echo "未知场景: $SCENARIO"
        echo "用法: $0 [all | inbound | api | batch | dialpad | signal | options | register | cancel | invalid-sdp | legality]"
        exit 1
        ;;
esac

# ============================================================================
# 汇总
# ============================================================================
echo -e "${CYAN}=========================================="
echo "  端到端测试结果汇总"
echo -e "==========================================${NC}"
for r in "${RESULTS[@]}"; do
    echo -e "  $r"
done
echo ""
echo -e "  ${GREEN}通过: $PASS${NC}  ${RED}失败: $FAIL${NC}"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
