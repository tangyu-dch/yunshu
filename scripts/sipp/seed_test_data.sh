#!/bin/bash
# ============================================================================
# 云枢 E2E 测试数据初始化脚本
#
# 初始化: MySQL 测试数据 + Redis 状态
# 用法:   bash seed_test_data.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo -e "${YELLOW}[1/3]${NC} 初始化 MySQL 测试数据..."

docker exec -i cc-mysql mysql -uroot -pdb123456 yunshu < "$SCRIPT_DIR/seed_test_data.sql" 2>/dev/null

echo -e "  ${GREEN}✓${NC} MySQL 测试数据已初始化"

echo -e "${YELLOW}[2/3]${NC} 初始化 Redis 坐席状态..."

# 分机状态 → IDLE
docker exec cc-redis redis-cli HSET extension:status 1001 1 > /dev/null 2>&1
docker exec cc-redis redis-cli HSET extension:status 1002 1 > /dev/null 2>&1

# Kamailio 认证缓存
docker exec cc-redis redis-cli HSET kamailio:auth:sip.yunshu.local 1001 2001 > /dev/null 2>&1
docker exec cc-redis redis-cli HSET kamailio:auth:sip.yunshu.local 1002 2002 > /dev/null 2>&1

# Alive key (TTL 5 分钟)
docker exec cc-redis redis-cli SET extension:alive:1001 1 EX 300 > /dev/null 2>&1
docker exec cc-redis redis-cli SET extension:alive:1002 1 EX 300 > /dev/null 2>&1

echo -e "  ${GREEN}✓${NC} Redis 坐席状态已初始化 (1001/1002 = IDLE)"

echo -e "${YELLOW}[3/3]${NC} 验证数据..."

# 验证 MySQL
GW_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu -N -e "SELECT COUNT(*) FROM cc_tel_gateway WHERE name='test-trunk' AND del_flag=0" 2>/dev/null)
POOL_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu -N -e "SELECT COUNT(*) FROM cc_tel_pool WHERE name='E2E测试号码池' AND del_flag=0" 2>/dev/null)
PHONE_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu -N -e "SELECT COUNT(*) FROM cc_res_pool_phone WHERE pool_id=(SELECT id FROM cc_tel_pool WHERE name='E2E测试号码池') AND del_flag=0" 2>/dev/null)
SG_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu -N -e "SELECT COUNT(*) FROM cc_res_skill_group WHERE name='E2E测试技能组' AND del_flag=0" 2>/dev/null)
LOC_COUNT=$(docker exec cc-mysql mysql -uroot -pdb123456 yunshu -N -e "SELECT COUNT(*) FROM cc_res_location WHERE username='1001'" 2>/dev/null)

echo -e "  网关: ${GREEN}$GW_COUNT${NC}  号码池: ${GREEN}$POOL_COUNT${NC}  号码: ${GREEN}$PHONE_COUNT${NC}  技能组: ${GREEN}$SG_COUNT${NC}  位置: ${GREEN}$LOC_COUNT${NC}"

# 验证 Redis
STATUS_1001=$(docker exec cc-redis redis-cli HGET extension:status 1001)
STATUS_1002=$(docker exec cc-redis redis-cli HGET extension:status 1002)

echo -e "  分机 1001 状态: ${GREEN}$STATUS_1001${NC} (1=IDLE)  分机 1002 状态: ${GREEN}$STATUS_1002${NC} (1=IDLE)"
echo ""
echo -e "${GREEN}测试数据初始化完成！${NC}"
