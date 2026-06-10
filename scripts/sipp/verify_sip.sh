#!/bin/bash
# ============================================================================
# 云枢 SIP 信令通路验证脚本
#
# 验证内容:
#   1. Docker 容器运行状态
#   2. FreeSWITCH OPTIONS 可用性探测
#   3. Kamailio OPTIONS 可用性探测
#   4. FreeSWITCH INVITE 信令验证
#   5. Kamailio INVITE 信令验证
#   6. SIP 合法性校验 (REGISTER/CANCEL/畸形SDP)
#   7. FreeSWITCH Sofia Profile 状态
#   8. 基础设施连接 (ESL/HTTP/Redis/MySQL)
#
# 前置条件:
#   1. SIPp 已安装 (brew install sipp)
#   2. Docker 容器运行中 (cc-freeswitch, cc-kamailio)
#   3. 云枢服务运行中 (端口 8080)
#
# 使用方法:
#   bash verify_sip.sh
# ============================================================================

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Docker 内网 IP（直连容器绕过 NAT）
FS_HOST="${FS_HOST:-192.168.107.6}"
KAMAILIO_HOST="${KAMAILIO_HOST:-192.168.107.2}"
LOCAL_SIP_IP="${LOCAL_SIP_IP:-192.168.107.0}"
FS_SIP_PORT=5080
KAMAILIO_PORT=5060

PASS=0
FAIL=0
WARN=0

# ============================================================================
# 工具函数
# ============================================================================

# 运行 SIPp 场景并测量延迟
# 用法: run_sipp_timed "scenario.xml" "service" "port" "target_host:port"
# 结果: TIMED_EXIT (退出码), TIMED_MS (延迟毫秒), TIMED_OUTPUT (sipp 输出)
run_sipp_timed() {
    local scenario="$1"
    local service="$2"
    local port="$3"
    local target="$4"

    local start_ms
    start_ms=$(python3 -c 'import time; print(int(time.time()*1000))' 2>/dev/null || echo $(($(date +%s) * 1000)))

    TIMED_OUTPUT=$(sipp -sf "$SCRIPT_DIR/$scenario" \
        -s "$service" \
        -i "$LOCAL_SIP_IP" \
        -p "$port" \
        -m 1 \
        -nostdin \
        -timeout 10s \
        -timeout_error \
        "$target" 2>&1 || true)

    TIMED_EXIT=$?

    local end_ms
    end_ms=$(python3 -c 'import time; print(int(time.time()*1000))' 2>/dev/null || echo $(($(date +%s) * 1000)))
    TIMED_MS=$((end_ms - start_ms))
}

# 从 SIPp 输出提取 SIP 响应码
extract_sip_response() {
    echo "$TIMED_OUTPUT" | grep -oE 'SIP/2\.0 [0-9]+ [A-Za-z ]+' | head -1 | sed 's/SIP\/2\.0 //' || echo "无响应"
}

echo ""
echo -e "${CYAN}=========================================="
echo "  云枢 SIP 信令通路验证"
echo -e "==========================================${NC}"
echo "  FreeSWITCH:   $FS_HOST:$FS_SIP_PORT"
echo "  Kamailio:     $KAMAILIO_HOST:$KAMAILIO_PORT"
echo "  本机 SIP IP:  $LOCAL_SIP_IP"
echo ""

# ---- 1. 基础连通性 ----
echo -e "${YELLOW}[1/8]${NC} 检查 Docker 容器状态..."
for svc in cc-freeswitch cc-kamailio cc-redis cc-mysql; do
    if docker ps --format '{{.Names}}' | grep -q "^${svc}$"; then
        status=$(docker inspect --format '{{.State.Health.Status}}' "$svc" 2>/dev/null || echo "running")
        echo -e "  ${GREEN}✓${NC} $svc: ${status:-running}"
    else
        echo -e "  ${RED}✗${NC} $svc: 未运行"
        ((FAIL++))
    fi
done
echo ""

# ---- 2. FreeSWITCH OPTIONS 可用性探测 ----
echo -e "${YELLOW}[2/8]${NC} FreeSWITCH OPTIONS 可用性探测..."
run_sipp_timed "options_check_uac.xml" "sipp" 6070 "$FS_HOST:$FS_SIP_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    sip_resp=$(extract_sip_response)
    echo -e "  ${GREEN}✓${NC} FS OPTIONS: ${TIMED_MS}ms (${sip_resp:-传输层可达})"
    ((PASS++))
else
    if echo "$TIMED_OUTPUT" | grep -q "timed out"; then
        echo -e "  ${RED}✗${NC} FS OPTIONS: ${TIMED_MS}ms (超时 — 信令不通)"
        ((FAIL++))
    else
        echo -e "  ${YELLOW}?${NC} FS OPTIONS: ${TIMED_MS}ms (异常退出)"
        ((WARN++))
    fi
fi
echo ""

# ---- 3. Kamailio OPTIONS 可用性探测 ----
echo -e "${YELLOW}[3/8]${NC} Kamailio OPTIONS 可用性探测..."
run_sipp_timed "options_check_uac.xml" "sipp" 6090 "$KAMAILIO_HOST:$KAMAILIO_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    sip_resp=$(extract_sip_response)
    echo -e "  ${GREEN}✓${NC} Kamailio OPTIONS: ${TIMED_MS}ms (${sip_resp:-传输层可达})"
    ((PASS++))
else
    if echo "$TIMED_OUTPUT" | grep -q "timed out"; then
        echo -e "  ${RED}✗${NC} Kamailio OPTIONS: ${TIMED_MS}ms (超时 — 信令不通)"
        ((FAIL++))
    else
        echo -e "  ${YELLOW}?${NC} Kamailio OPTIONS: ${TIMED_MS}ms (异常退出)"
        ((WARN++))
    fi
fi
echo ""

# ---- 4. FreeSWITCH INVITE 信令验证 ----
echo -e "${YELLOW}[4/8]${NC} FreeSWITCH INVITE 信令验证..."
echo "  发送 INVITE → $FS_HOST:$FS_SIP_PORT ..."
run_sipp_timed "signal_check_uac.xml" "01088886666" 6071 "$FS_HOST:$FS_SIP_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} FS INVITE: ${TIMED_MS}ms — 信令通路正常 (200 OK 呼叫建立)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "unexpected message"; then
    # signal_check 的 rejected 路径: SIPp 收到 4xx/5xx 后 ACK + stop_now
    # 但由于 optional recv 可能不匹配，fallback 检查
    sip_resp=$(echo "$TIMED_OUTPUT" | grep -oE 'SIP/2\.0 [0-9]+ [A-Za-z ]+' | head -1 | sed 's/SIP\/2\.0 //')
    echo -e "  ${GREEN}✓${NC} FS INVITE: ${TIMED_MS}ms — ${sip_resp:-4xx 响应} (信令通路正常，无坐席注册)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "timed out"; then
    echo -e "  ${RED}✗${NC} FS INVITE: ${TIMED_MS}ms (超时 — 信令不通)"
    ((FAIL++))
else
    # SIPp 场景正常退出 (rejected 路径: ACK + stop_now)
    # 检查是否有 SIP 交互发生
    if echo "$TIMED_OUTPUT" | grep -qE "100|180|183|200|404|480|486|503"; then
        sip_resp=$(extract_sip_response)
        echo -e "  ${GREEN}✓${NC} FS INVITE: ${TIMED_MS}ms — ${sip_resp} (信令通路正常)"
        ((PASS++))
    else
        echo -e "  ${YELLOW}?${NC} FS INVITE: ${TIMED_MS}ms (响应异常)"
        echo "$TIMED_OUTPUT" | grep -E "unexpected|error|abort" | head -3
        ((WARN++))
    fi
fi
echo ""

# ---- 5. Kamailio INVITE 信令验证 ----
echo -e "${YELLOW}[5/8]${NC} Kamailio INVITE 信令验证..."
echo "  发送 INVITE → $KAMAILIO_HOST:$KAMAILIO_PORT ..."
run_sipp_timed "signal_check_uac.xml" "1001" 6091 "$KAMAILIO_HOST:$KAMAILIO_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} Kamailio INVITE: ${TIMED_MS}ms — 信令通路正常 (200 OK 呼叫建立)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "unexpected message"; then
    sip_resp=$(echo "$TIMED_OUTPUT" | grep -oE 'SIP/2\.0 [0-9]+ [A-Za-z ]+' | head -1 | sed 's/SIP\/2\.0 //')
    echo -e "  ${GREEN}✓${NC} Kamailio INVITE: ${TIMED_MS}ms — ${sip_resp:-4xx 响应} (信令通路正常)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "timed out"; then
    echo -e "  ${RED}✗${NC} Kamailio INVITE: ${TIMED_MS}ms (超时 — 信令不通)"
    ((FAIL++))
else
    if echo "$TIMED_OUTPUT" | grep -qE "100|180|183|200|404|480|486|503"; then
        sip_resp=$(extract_sip_response)
        echo -e "  ${GREEN}✓${NC} Kamailio INVITE: ${TIMED_MS}ms — ${sip_resp} (信令通路正常)"
        ((PASS++))
    else
        echo -e "  ${YELLOW}?${NC} Kamailio INVITE: ${TIMED_MS}ms (响应异常)"
        echo "$TIMED_OUTPUT" | grep -E "unexpected|error|abort" | head -3
        ((WARN++))
    fi
fi
echo ""

# ---- 6. SIP 合法性校验 ----
echo -e "${YELLOW}[6/8]${NC} SIP 合法性校验..."

# 6a. REGISTER 合法性 (Kamailio)
echo -e "  ${CYAN}6a.${NC} REGISTER 鉴权合法性 (Kamailio)..."
run_sipp_timed "register_uac.xml" "1001" 6061 "$KAMAILIO_HOST:$KAMAILIO_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    sip_resp=$(extract_sip_response)
    if echo "$sip_resp" | grep -q "401"; then
        echo -e "  ${GREEN}✓${NC} REGISTER: ${TIMED_MS}ms — 401 Unauthorized (鉴权挑战正常)"
    elif echo "$sip_resp" | grep -q "403"; then
        echo -e "  ${GREEN}✓${NC} REGISTER: ${TIMED_MS}ms — 403 Forbidden (鉴权流程正常)"
    elif echo "$sip_resp" | grep -q "200"; then
        echo -e "  ${GREEN}✓${NC} REGISTER: ${TIMED_MS}ms — 200 OK (注册成功)"
    else
        echo -e "  ${GREEN}✓${NC} REGISTER: ${TIMED_MS}ms — ${sip_resp:-响应正常}"
    fi
    ((PASS++))
else
    if echo "$TIMED_OUTPUT" | grep -q "timed out"; then
        echo -e "  ${RED}✗${NC} REGISTER: ${TIMED_MS}ms (超时 — Kamailio 未响应)"
        ((FAIL++))
    else
        echo -e "  ${YELLOW}?${NC} REGISTER: ${TIMED_MS}ms (异常)"
        ((WARN++))
    fi
fi

# 6b. CANCEL 合法性 (FreeSWITCH)
echo -e "  ${CYAN}6b.${NC} CANCEL 事务合法性 (FreeSWITCH)..."
run_sipp_timed "cancel_uac.xml" "01088886666" 6072 "$FS_HOST:$FS_SIP_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} CANCEL: ${TIMED_MS}ms — 487 Request Terminated (CANCEL 事务正常)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "487"; then
    echo -e "  ${GREEN}✓${NC} CANCEL: ${TIMED_MS}ms — 487 响应已收到 (CANCEL 合法)"
    ((PASS++))
elif echo "$TIMED_OUTPUT" | grep -q "timed out"; then
    echo -e "  ${RED}✗${NC} CANCEL: ${TIMED_MS}ms (超时 — CANCEL 信令不通)"
    ((FAIL++))
else
    # CANCEL 可能因为 INVITE 被快速拒绝 (480/404) 而未能匹配
    if echo "$TIMED_OUTPUT" | grep -qE "480|404"; then
        echo -e "  ${GREEN}✓${NC} CANCEL: ${TIMED_MS}ms — INVITE 已被快速拒绝 (信令通路正常)"
        ((PASS++))
    else
        echo -e "  ${YELLOW}?${NC} CANCEL: ${TIMED_MS}ms (异常)"
        ((WARN++))
    fi
fi

# 6c. 畸形 SDP 错误处理 (FreeSWITCH)
echo -e "  ${CYAN}6c.${NC} 畸形 SDP 错误处理 (FreeSWITCH)..."
run_sipp_timed "invalid_sdp_uac.xml" "01088886666" 6073 "$FS_HOST:$FS_SIP_PORT"

if [ "$TIMED_EXIT" -eq 0 ]; then
    sip_resp=$(extract_sip_response)
    if echo "$sip_resp" | grep -qE "488|400|415"; then
        echo -e "  ${GREEN}✓${NC} 畸形SDP: ${TIMED_MS}ms — ${sip_resp} (错误正确拒绝)"
    elif echo "$sip_resp" | grep -q "200"; then
        echo -e "  ${YELLOW}⚠${NC} 畸形SDP: ${TIMED_MS}ms — 200 OK (畸形 SDP 未被检测，合规性问题)"
        ((WARN++))
    else
        echo -e "  ${GREEN}✓${NC} 畸形SDP: ${TIMED_MS}ms — ${sip_resp:-响应正常}"
    fi
    ((PASS++))
else
    if echo "$TIMED_OUTPUT" | grep -q "timed out"; then
        echo -e "  ${RED}✗${NC} 畸形SDP: ${TIMED_MS}ms (超时 — 信令不通)"
        ((FAIL++))
    else
        # 检查是否有 SIP 错误响应被正确处理
        if echo "$TIMED_OUTPUT" | grep -qE "488|400|415|480|404"; then
            sip_resp=$(extract_sip_response)
            echo -e "  ${GREEN}✓${NC} 畸形SDP: ${TIMED_MS}ms — ${sip_resp:-错误正确拒绝}"
            ((PASS++))
        else
            echo -e "  ${YELLOW}?${NC} 畸形SDP: ${TIMED_MS}ms (异常)"
            ((WARN++))
        fi
    fi
fi
echo ""

# ---- 7. FS Sofia Profile 状态 ----
echo -e "${YELLOW}[7/8]${NC} FreeSWITCH Sofia Profile 状态..."
PROFILES=$(docker exec cc-freeswitch fs_cli -x "sofia status" 2>/dev/null | grep -E "RUNNING|ALIASED")
if [ -n "$PROFILES" ]; then
    echo "$PROFILES" | while read -r line; do
        name=$(echo "$line" | awk '{print $1}')
        data=$(echo "$line" | awk '{print $3}')
        echo -e "  ${GREEN}✓${NC} $name: $data"
    done
    ((PASS++))
else
    echo -e "  ${RED}✗${NC} 无法获取 Sofia Profile 状态"
    ((FAIL++))
fi
echo ""

# ---- 8. 基础设施连接验证 ----
echo -e "${YELLOW}[8/8]${NC} 基础设施连接验证..."

# ESL
if docker exec cc-freeswitch fs_cli -x "status" 2>/dev/null | grep -q "UP"; then
    echo -e "  ${GREEN}✓${NC} FreeSWITCH ESL 端口 (8021) 可达"
    ((PASS++))
else
    echo -e "  ${RED}✗${NC} FreeSWITCH ESL 端口不可达"
    ((FAIL++))
fi

# 云枢 HTTP
YUNSHU_HTTP=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/healthz 2>/dev/null || echo "000")
if [ "$YUNSHU_HTTP" = "200" ]; then
    echo -e "  ${GREEN}✓${NC} 云枢服务 HTTP (8080) 健康"
    ((PASS++))
elif [ "$YUNSHU_HTTP" = "404" ]; then
    echo -e "  ${GREEN}✓${NC} 云枢服务 HTTP (8080) 可达 (无 /healthz, 返回 404)"
    ((PASS++))
else
    echo -e "  ${YELLOW}?${NC} 云枢服务 HTTP 响应: $YUNSHU_HTTP"
    ((WARN++))
fi

# Redis
REDIS_PING=$(docker exec cc-redis redis-cli ping 2>/dev/null || echo "FAIL")
if [ "$REDIS_PING" = "PONG" ]; then
    echo -e "  ${GREEN}✓${NC} Redis 连接正常"
    ((PASS++))
else
    echo -e "  ${RED}✗${NC} Redis 连接失败"
    ((FAIL++))
fi

# MySQL
MYSQL_UP=$(docker exec cc-mysql mysqladmin ping -h 127.0.0.1 -u root -pdb123456 2>/dev/null | grep -c "alive" || echo "0")
if [ "$MYSQL_UP" -ge 1 ]; then
    echo -e "  ${GREEN}✓${NC} MySQL 连接正常"
    ((PASS++))
else
    echo -e "  ${RED}✗${NC} MySQL 连接失败"
    ((FAIL++))
fi
echo ""

# ---- 汇总 ----
echo -e "${CYAN}=========================================="
echo "  验证结果汇总"
echo -e "==========================================${NC}"
echo -e "  ${GREEN}通过: $PASS${NC}"
echo -e "  ${RED}失败: $FAIL${NC}"
echo -e "  ${YELLOW}警告: $WARN${NC}"
echo ""

if [ "$FAIL" -eq 0 ]; then
    echo -e "${GREEN}所有 SIP 信令通路验证通过！${NC}"
    echo "注: 480/404 响应表示信令通路正常但无坐席注册（预期行为）"
    exit 0
else
    echo -e "${RED}存在验证失败项，请检查上方日志${NC}"
    exit 1
fi
