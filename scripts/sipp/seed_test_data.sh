#!/bin/bash
# ============================================================================
# 云枢 E2E 测试数据初始化脚本 (幂等版, 自包含)
#
# 初始化: MySQL 测试数据 + Redis 状态
# 前置: 已通过 yunshu installer install 初始化基础数据 (商户/FS 节点等)
#       本脚本会自动创建所需的分机/坐席/号码池/技能组等测试数据
# 用法:   bash seed_test_data.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${YELLOW}[0/4]${NC} 验证前置依赖 (installer 基础数据)..."

# 验证 installer 创建的基础数据存在
MISSING=0

# 商户 1001 必须存在 (OutboundGuard.validateMerchant 依赖)
MCH_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e \
    "SELECT COUNT(*) FROM cc_mch_info WHERE id = 1001 AND del_flag = 0" 2>/dev/null || echo "0")
if [ "$MCH_COUNT" -lt 1 ]; then
    echo -e "  ${RED}✗${NC} 商户 1001 不存在 — 请先运行 yunshu installer install"
    MISSING=1
else
    echo -e "  ${GREEN}✓${NC} 商户 1001 存在"
fi

# 分机 1001/1002 由本脚本创建 (无需 installer 预置，仅提示状态)
EXT_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e \
    "SELECT COUNT(*) FROM cc_res_extension WHERE extension_number IN ('1001','1002') AND merchant_id = 1001 AND del_flag = 0" 2>/dev/null || echo "0")
if [ "$EXT_COUNT" -ge 2 ]; then
    echo -e "  ${GREEN}✓${NC} 分机 1001/1002 已存在 (将重建)"
else
    echo -e "  ${CYAN}ℹ${NC} 分机 1001/1002 不存在 (将由本脚本创建)"
fi

# 检查计费配置 (OutboundGuard.validateBilling 依赖)
# 无计费记录时 validateBilling 放行 (ErrRecordNotFound -> nil)，但预付费余额不足会拒绝
BILLING_MODE=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e \
    "SELECT payment_mode FROM cc_mch_billing_overview WHERE merchant_id = 1001 LIMIT 1" 2>/dev/null || echo "")
if [ -n "$BILLING_MODE" ] && [ "$BILLING_MODE" = "1" ]; then
    # 预付费模式，检查余额
    BALANCE=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e \
        "SELECT current_balance + COALESCE(credit_limit, 0) FROM cc_mch_billing_overview WHERE merchant_id = 1001 LIMIT 1" 2>/dev/null || echo "0")
    if [ "$BALANCE" -le 0 ] 2>/dev/null; then
        echo -e "  ${YELLOW}⚠${NC} 商户 1001 预付费余额为 0 — API 外呼将被 OutboundGuard 拒绝"
    else
        echo -e "  ${GREEN}✓${NC} 商户 1001 计费正常 (预付费, 余额充足)"
    fi
else
    echo -e "  ${GREEN}✓${NC} 商户 1001 计费正常 (后付费或无记录)"
fi

if [ "$MISSING" -eq 1 ]; then
    echo -e "  ${RED}前置依赖缺失，请先运行: yunshu installer install${NC}"
    exit 1
fi
echo ""

echo -e "${YELLOW}[1/4]${NC} 初始化 MySQL 测试数据 (幂等)..."

SEED_OUTPUT=$(docker exec -i cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 < "$SCRIPT_DIR/seed_test_data.sql" 2>&1)

if [ $? -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} MySQL 测试数据已初始化"
    # 提取最后一行汇总
    SUMMARY=$(echo "$SEED_OUTPUT" | grep "^seed_data" | tail -1)
    if [ -n "$SUMMARY" ]; then
        echo -e "    $SUMMARY" | awk -F'\t' '{printf "    网关: %s  号码池: %s  技能组: %s  号码: %s,%s,%s  坐席: %s,%s\n", $2,$3,$4,$5,$6,$7,$8,$9}'
    fi
else
    echo -e "  ${RED}✗${NC} MySQL 初始化失败:"
    echo "$SEED_OUTPUT"
    exit 1
fi

echo -e "${YELLOW}[2/4]${NC} 初始化 Redis 坐席状态..."

# 分机状态 -> IDLE (1=IDLE, 2=BUSY, 3=WRAP-up, 4=OFFLINE)
docker exec cc-redis redis-cli HSET extension:status 1001 1 > /dev/null 2>&1
docker exec cc-redis redis-cli HSET extension:status 1002 1 > /dev/null 2>&1

# Kamailio 认证缓存
docker exec cc-redis redis-cli HSET kamailio:auth:sip.merchant.yunshu.com 1001 1 > /dev/null 2>&1
docker exec cc-redis redis-cli HSET kamailio:auth:sip.merchant.yunshu.com 1002 1 > /dev/null 2>&1

# Alive key (TTL 1 小时 — 覆盖 E2E 测试完整执行周期)
docker exec cc-redis redis-cli SET extension:alive:1001 1 EX 3600 > /dev/null 2>&1
docker exec cc-redis redis-cli SET extension:alive:1002 1 EX 3600 > /dev/null 2>&1

echo -e "  ${GREEN}✓${NC} Redis 坐席状态已初始化 (1001/1002 = IDLE, TTL=3600s)"

echo -e "${YELLOW}[3/4]${NC} 验证数据..."

# 验证 MySQL
GW_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_tel_gateway WHERE name='test-trunk' AND del_flag=0" 2>/dev/null)
POOL_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_tel_pool WHERE name='E2E测试号码池' AND del_flag=0" 2>/dev/null)
PHONE_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_res_pool_phone WHERE phone IN ('01088886666','01088887777','01088888888') AND del_flag=0" 2>/dev/null)
SG_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_res_skill_group WHERE name='E2E测试技能组' AND del_flag=0" 2>/dev/null)
LOC_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_res_location WHERE user_agent='SIPp-e2e-test'" 2>/dev/null)
EXT_TEST_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu --default-character-set=utf8mb4 -N -e "SELECT COUNT(*) FROM cc_res_extension WHERE extension_number IN ('1001','1002') AND merchant_id = 1001 AND del_flag = 0" 2>/dev/null)

echo -e "  网关: ${GREEN}$GW_COUNT${NC}  号码池: ${GREEN}$POOL_COUNT${NC}  号码: ${GREEN}$PHONE_COUNT${NC}  技能组: ${GREEN}$SG_COUNT${NC}  位置: ${GREEN}$LOC_COUNT${NC}  分机: ${GREEN}$EXT_TEST_COUNT${NC}"

# 验证 Redis
STATUS_1001=$(docker exec cc-redis redis-cli HGET extension:status 1001)
STATUS_1002=$(docker exec cc-redis redis-cli HGET extension:status 1002)

echo -e "  分机 1001 状态: ${GREEN}$STATUS_1001${NC} (1=IDLE)  分机 1002 状态: ${GREEN}$STATUS_1002${NC} (1=IDLE)"
echo ""

echo -e "${YELLOW}[4/4]${NC} 验证数据完整性..."

# 数据完整性校验
if [ "$GW_COUNT" -eq 1 ] && [ "$POOL_COUNT" -eq 1 ] && [ "$PHONE_COUNT" -eq 3 ] && [ "$SG_COUNT" -eq 1 ] && [ "$LOC_COUNT" -eq 2 ] && [ "$EXT_TEST_COUNT" -eq 2 ]; then
    echo -e "${GREEN}✓ 测试数据初始化完成，所有数据校验通过！${NC}"
else
    echo -e "${YELLOW}⚠ 数据校验不一致，预期: 网关=1 号码池=1 号码=3 技能组=1 位置=2 分机=2${NC}"
    exit 1
fi
