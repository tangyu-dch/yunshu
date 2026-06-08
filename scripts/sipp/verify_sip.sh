#!/bin/bash
# ============================================================================
# 云枢 SIP 信令通路验证脚本
#
# 验证内容:
#   1. Docker 容器运行状态
#   2. FreeSWITCH SIP 信令通路 (INVITE → 100/480)
#   3. Kamailio SIP 信令通路 (INVITE → 100)
#   4. FreeSWITCH Sofia Profile 状态
#   5. 基础设施连接 (ESL/HTTP/Redis/MySQL)
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
KAMAILIO_HOST="${KAMAILIO_HOST:-192.168.107.6}"
LOCAL_SIP_IP="${LOCAL_SIP_IP:-192.168.107.0}"
FS_SIP_PORT=5080
KAMAILIO_PORT=5060

PASS=0
FAIL=0
WARN=0

echo ""
echo -e "${CYAN}=========================================="
echo "  云枢 SIP 信令通路验证"
echo -e "==========================================${NC}"
echo "  FreeSWITCH:   $FS_HOST:$FS_SIP_PORT"
echo "  Kamailio:     $KAMAILIO_HOST:$KAMAILIO_PORT"
echo "  本机 SIP IP:  $LOCAL_SIP_IP"
echo ""

# ---- 1. 基础连通性 ----
echo -e "${YELLOW}[1/5]${NC} 检查 Docker 容器状态..."
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

# ---- 2. FreeSWITCH SIP 信令验证 ----
echo -e "${YELLOW}[2/5]${NC} FreeSWITCH INVITE 信令验证..."
echo "  发送 INVITE → $FS_HOST:$FS_SIP_PORT ..."
FS_RESULT=$(sipp -sf "$SCRIPT_DIR/inbound_uac.xml" \
    -s 01088886666 \
    -i "$LOCAL_SIP_IP" \
    -p 6070 \
    -m 1 \
    -nostdin \
    -timeout 10s \
    -timeout_error \
    "$FS_HOST:$FS_SIP_PORT" 2>&1 || true)

if echo "$FS_RESULT" | grep -q "Aborting call on unexpected message.*480"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 100 Trying + 480 Unavailable — 信令通路正常 (无坐席注册)"
    ((PASS++))
elif echo "$FS_RESULT" | grep -q "Aborting call on unexpected message.*404"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 100 Trying + 404 Not Found — 信令通路正常 (DID 未配置)"
    ((PASS++))
elif echo "$FS_RESULT" | grep -q "200.*<---"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 200 OK — 呼叫建立成功!"
    ((PASS++))
elif echo "$FS_RESULT" | grep -q "100.*<---"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 100 Trying — SIP 传输层正常"
    ((PASS++))
elif echo "$FS_RESULT" | grep -q "timed out"; then
    echo -e "  ${RED}✗${NC} INVITE 超时 — 信令不通"
    ((FAIL++))
else
    echo -e "  ${YELLOW}?${NC} FS 响应异常:"
    echo "$FS_RESULT" | grep -E "unexpected|error|abort" | head -3
    ((WARN++))
fi
echo ""

# ---- 3. Kamailio SIP 信令验证 ----
echo -e "${YELLOW}[3/5]${NC} Kamailio INVITE 信令验证..."
echo "  发送 INVITE → $KAMAILIO_HOST:$KAMAILIO_PORT ..."
KAM_RESULT=$(sipp -sf "$SCRIPT_DIR/dialpad_uac.xml" \
    -s 2001 \
    -i "$LOCAL_SIP_IP" \
    -p 6090 \
    -m 1 \
    -nostdin \
    -timeout 10s \
    -timeout_error \
    "$KAMAILIO_HOST:$KAMAILIO_PORT" 2>&1 || true)

if echo "$KAM_RESULT" | grep -q "Aborting call on unexpected message"; then
    resp=$(echo "$KAM_RESULT" | grep -oE 'SIP/2\.0 [0-9]+ [A-Za-z ]+' | head -1 | sed 's/SIP\/2\.0 //')
    echo -e "  ${GREEN}✓${NC} INVITE → ${resp:-响应} — 信令通路正常"
    ((PASS++))
elif echo "$KAM_RESULT" | grep -q "200.*<---"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 200 OK — 呼叫建立成功"
    ((PASS++))
elif echo "$KAM_RESULT" | grep -q "100.*<---"; then
    echo -e "  ${GREEN}✓${NC} INVITE → 100 Trying — 传输层正常"
    ((PASS++))
elif echo "$KAM_RESULT" | grep -q "timed out"; then
    echo -e "  ${RED}✗${NC} INVITE 超时 — 信令不通"
    ((FAIL++))
else
    echo -e "  ${YELLOW}?${NC} Kamailio 响应异常:"
    echo "$KAM_RESULT" | grep -E "unexpected|error|abort" | head -3
    ((WARN++))
fi
echo ""

# ---- 4. FS Sofia Profile 状态 ----
echo -e "${YELLOW}[4/5]${NC} FreeSWITCH Sofia Profile 状态..."
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

# ---- 5. 基础设施连接验证 ----
echo -e "${YELLOW}[5/5]${NC} 基础设施连接验证..."

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
