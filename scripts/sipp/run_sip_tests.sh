#!/bin/bash
# ============================================================================
# 云枢 SIP 流程验证脚本
#
# 前置条件:
#   1. sip: SIPp 已安装 (brew install sipper / apt install sipp)
#   2. FreeSWITCH + Kamailio + MySQL + Redis 已启动 (docker compose up -d)
#   3. 已通过 installer 初始化数据库 (yunshu installer install)
#   4. 云枢服务已启动 (yunshu serve)
#
# 使用方法:
#   ./run_sip_tests.sh [scenario]
#   scenario: all | batch | api | inbound | dialpad
#   默认运行 all
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SIPP_DIR="$SCRIPT_DIR"

# 配置参数（根据实际环境修改）
# Docker 内网 IP（绕过 Docker NAT，直连容器）
FS_HOST="${FS_HOST:-192.168.107.6}"
FS_SIP_PORT="${FS_SIP_PORT:-5080}"
KAMAILIO_HOST="${KAMAILIO_HOST:-192.168.107.6}"
KAMAILIO_PORT="${KAMAILIO_PORT:-5060}"
CALLER_NUMBER="${CALLER_NUMBER:-01012345678}"
CALLEE_NUMBER="${CALLEE_NUMBER:-13800001111}"
AGENT_EXT="${AGENT_EXT:-2001}"
DID_NUMBER="${DID_NUMBER:-01088886666}"
# 本机在 Docker 网络上的 IP（FS/Kamailio 回包地址）
LOCAL_SIP_IP="${LOCAL_SIP_IP:-192.168.107.0}"

# 测试结果统计
PASS=0
FAIL=0
RESULTS=()

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

check_prerequisites() {
    echo -e "${YELLOW}[检查]${NC} 验证前置条件..."

    if ! command -v sipp &> /dev/null; then
        echo -e "${RED}[错误]${NC} SIPp 未安装，请先安装: brew install sipper 或 apt install sipp"
        exit 1
    fi

    # 检查 FreeSWITCH ESL 端口是否可达
    if ! nc -z "$FS_HOST" 8021 2>/dev/null; then
        echo -e "${RED}[错误]${NC} FreeSWITCH ESL 端口 8021 不可达，请确认 FS 已启动"
        exit 1
    fi

    echo -e "${GREEN}[OK]${NC} SIPp 版本: $(sipp -v 2>&1 | head -1)"
    echo -e "${GREEN}[OK]${NC} FreeSWITCH ESL 端口可达"
    echo ""
}

run_test() {
    local name="$1"
    local scenario_file="$2"
    local sipp_args="$3"

    echo -e "${YELLOW}[测试]${NC} $name"
    echo "       场景文件: $scenario_file"
    echo "       参数: $sipp_args"

    local log_file="/tmp/sipp_${name}_$(date +%s).log"

    if sipp -sf "$scenario_file" $sipp_args \
        -i "$LOCAL_SIP_IP" \
        -trace_err -error_file "/tmp/sipp_${name}_err.log" \
        -screen_file "$log_file" \
        -timeout 30s -timeout_error \
        2>&1; then
        echo -e "${GREEN}[通过]${NC} $name"
        RESULTS+=("${GREEN}PASS${NC}: $name")
        ((PASS++))
    else
        local exit_code=$?
        echo -e "${RED}[失败]${NC} $name (退出码: $exit_code)"
        if [ -f "/tmp/sipp_${name}_err.log" ]; then
            echo "       错误日志: $(tail -5 /tmp/sipp_${name}_err.log 2>/dev/null)"
        fi
        RESULTS+=("${RED}FAIL${NC}: $name")
        ((FAIL++))
    fi
    echo ""
}

test_batch_outbound() {
    echo "=========================================="
    echo "  场景 1: 批量外呼 (CUSTOMER_FIRST)"
    echo "=========================================="
    # 需要通过云枢 API 创建批量任务并触发，SIPp 模拟客户接听
    # 此场景需要先通过 API 创建任务，然后 SIPp 作为 UAS 等待来电
    run_test "batch_outbound" \
        "$SIPP_DIR/batch_outbound_uac.xml" \
        "-s $CALLEE_NUMBER -p 6080 -r 1 -m 1 -nostdin $FS_HOST:$FS_SIP_PORT"
}

test_api_outbound() {
    echo "=========================================="
    echo "  场景 2: API 外呼 (AGENT_FIRST)"
    echo "=========================================="
    # SIPp 模拟坐席 SIP 电话，等待系统 INVITE
    # 需要先通过 API 发起外呼请求
    run_test "api_outbound_agent" \
        "$SIPP_DIR/api_outbound_uas.xml" \
        "-s $AGENT_EXT -p 6060 -m 1 -nostdin $KAMAILIO_HOST:$KAMAILIO_PORT"
}

test_inbound() {
    echo "=========================================="
    echo "  场景 3: 客户呼入 (INBOUND)"
    echo "=========================================="
    # SIPp 模拟客户通过 DID 呼入
    # 前提：系统中有配置的 DID 号码和在线坐席
    run_test "inbound" \
        "$SIPP_DIR/inbound_uac.xml" \
        "-s $DID_NUMBER -p 6070 -m 1 -nostdin $FS_HOST:$FS_SIP_PORT"
}

test_dialpad_direct() {
    echo "=========================================="
    echo "  场景 4: 拨号盘直呼 (API_DIRECT)"
    echo "=========================================="
    # SIPp 模拟坐席 SIP 电话发起外呼
    run_test "dialpad_direct" \
        "$SIPP_DIR/dialpad_uac.xml" \
        "-s $AGENT_EXT -p 6090 -m 1 -nostdin $KAMAILIO_HOST:$KAMAILIO_PORT"
}

print_summary() {
    echo ""
    echo "=========================================="
    echo "  测试结果汇总"
    echo "=========================================="
    for r in "${RESULTS[@]}"; do
        echo -e "  $r"
    done
    echo ""
    echo -e "  通过: ${GREEN}$PASS${NC}  失败: ${RED}$FAIL${NC}"
    echo "=========================================="

    if [ "$FAIL" -gt 0 ]; then
        return 1
    fi
    return 0
}

# ============================================================================
# 主流程
# ============================================================================

SCENARIO="${1:-all}"

echo ""
echo "=========================================="
echo "  云枢 SIP 流程验证"
echo "=========================================="
echo "  FreeSWITCH: $FS_HOST"
echo "  Kamailio:   $KAMAILIO_HOST:$KAMAILIO_PORT"
echo "  场景:       $SCENARIO"
echo "=========================================="
echo ""

check_prerequisites

case "$SCENARIO" in
    batch)
        test_batch_outbound
        ;;
    api)
        test_api_outbound
        ;;
    inbound)
        test_inbound
        ;;
    dialpad)
        test_dialpad_direct
        ;;
    all)
        test_batch_outbound
        test_api_outbound
        test_inbound
        test_dialpad_direct
        ;;
    *)
        echo "未知场景: $SCENARIO"
        echo "用法: $0 [all | batch | api | inbound | dialpad]"
        exit 1
        ;;
esac

print_summary
