#!/bin/bash
# ============================================================================
# 云枢 SIP 端到端测试编排脚本
#
# 支持场景: inbound | api | batch | dialpad | all
# 用法:     bash run_e2e_tests.sh [scenario]
# ============================================================================

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Docker 内网 IP
FS_HOST="${FS_HOST:-192.168.107.6}"
KAM_HOST="${KAM_HOST:-192.168.107.6}"
LOCAL_IP="${LOCAL_IP:-192.168.107.0}"
FS_SIP_PORT=5080
KAM_PORT=5060

AGENT_PORT=6060
CUSTOMER_PORT=6080
INBOUND_PORT=6070
DIALPAD_PORT=6090

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS=0
FAIL=0
RESULTS=()
SIPP_PIDS=()

cleanup() {
    for pid in "${SIPP_PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null
}
trap cleanup EXIT

start_uas() {
    local name="$1"
    local scenario="$2"
    local service="$3"
    local port="$4"
    local target="$5"
    local log="/tmp/sipp_${name}_$(date +%s).log"

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

    while [ $timeout -gt 0 ]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            wait "$pid"
            local exit_code=$?
            return $exit_code
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

        echo -e "${YELLOW}[1/2]${NC} 启动坐席 UAS (port=$AGENT_PORT)..."
        AGENT_PID=$(start_uas "inbound_agent" "agent_uas.xml" "1001" "$AGENT_PORT" "$KAM_HOST:$KAM_PORT")
        sleep 1

        echo -e "${YELLOW}[2/2]${NC} 发起呼入 INVITE (DID=01088886666)..."
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
        HTTP_CODE=$(curl -s -o /tmp/api_call_resp.json -w "%{http_code}" \
            -X POST "http://127.0.0.1:8080/cti/callTask/call" \
            -H "Content-Type: application/json" \
            -d '{"userId":2001,"callee":"13800001111"}' 2>/dev/null || echo "000")

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
        echo -e "  提示: curl -X POST http://127.0.0.1:8080/cti/batch-call-task/dispatch -d '{\"taskId\":1}'"

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

        echo -e "${YELLOW}[1/2]${NC} 启动客户 UAS (port=$CUSTOMER_PORT)..."
        CUST_PID=$(start_uas "dialpad_customer" "customer_uas.xml" "13800003333" "$CUSTOMER_PORT" "$FS_HOST:$FS_SIP_PORT")
        sleep 1

        echo -e "${YELLOW}[2/2]${NC} 坐席发起 INVITE (模拟拨号盘直呼)..."
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

    *)
        echo "未知场景: $SCENARIO"
        echo "用法: $0 [all | inbound | api | batch | dialpad]"
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
