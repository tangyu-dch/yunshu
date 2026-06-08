-- ============================================================================
-- 云枢 SIP 端到端测试数据初始化 (幂等版, 自包含)
--
-- 用法: docker exec -i cc-mysql mysql -uroot -pdb123456 yunshu < seed_test_data.sql
--
-- 前置: 已通过 yunshu installer install 初始化基础数据 (商户/FS 节点等)
--       本脚本会自动创建所需的分机/坐席/号码池/技能组等测试数据
-- ============================================================================

SET NAMES utf8mb4;

-- ============================================================
-- 1. 清理旧测试数据 (幂等: 先删后插)
-- ============================================================

-- 外键依赖: 先删子表再删父表
DELETE FROM cc_res_pool_phone_skill_group WHERE pool_phone_id IN (
    SELECT p.id FROM cc_res_pool_phone p
    WHERE p.phone IN ('01088886666','01088887777','01088888888')
);
DELETE FROM cc_res_pool_phone WHERE phone IN ('01088886666','01088887777','01088888888');
DELETE FROM cc_tel_pool WHERE name = 'E2E测试号码池';

DELETE FROM cc_res_user_skill_group WHERE user_id IN (
    SELECT id FROM cc_res_mch_user WHERE seat_number IN ('1001','1002') AND merchant_id = 1001
    AND username IN ('坐席A','坐席B')
);
DELETE FROM cc_res_mch_user WHERE merchant_id = 1001 AND seat_number IN ('1001','1002')
    AND username IN ('坐席A','坐席B');

DELETE FROM cc_res_skill_group WHERE merchant_id = 1001 AND name = 'E2E测试技能组';

DELETE FROM cc_tel_gateway WHERE name = 'test-trunk';

DELETE FROM cc_res_location WHERE username IN ('1001','1002') AND user_agent = 'SIPp-e2e-test';

-- 清理测试分机 (仅清理本脚本创建的, 不影响 installer 创建的分机)
DELETE FROM cc_res_location WHERE username IN ('1001','1002') AND user_agent = 'SIPp-e2e-test';
DELETE FROM cc_res_extension WHERE extension_number IN ('1001','1002') AND merchant_id = 1001;

-- ============================================================
-- 2. 插入测试数据
-- ============================================================

-- ---- 2.1 SIP 中继网关 (Model=2 IP 直连, 指向 SIPp 客户 UAS) ----
INSERT INTO `cc_tel_gateway` (
    `name`, `description`, `model`, `realm`, `port`,
    `concurrency`, `priority`, `enable`, `del_flag`,
    `codec_prefs`, `created_time`, `updated_time`
) VALUES (
    'test-trunk', 'E2E测试外呼网关', 2, '192.168.107.0', '6080',
    100, 1, 1, 0,
    'PCMU,PCMA', NOW(3), NOW(3)
);

SET @gw_id = LAST_INSERT_ID();

-- ---- 2.2 号码池 ----
INSERT INTO `cc_tel_pool` (
    `merchant_id`, `name`, `gateway_id`, `selection_strategy`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES (
    1001, 'E2E测试号码池', @gw_id, 'CONCURRENCY', 1, 0, NOW(3), NOW(3)
);

SET @pool_id = LAST_INSERT_ID();

-- ---- 2.3 外呼号码 (3 个测试 DID) ----
INSERT INTO `cc_res_pool_phone` (
    `pool_id`, `phone`, `concurrency`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES
    (@pool_id, '01088886666', 10, 1, 0, NOW(3), NOW(3)),
    (@pool_id, '01088887777', 10, 1, 0, NOW(3), NOW(3)),
    (@pool_id, '01088888888', 10, 1, 0, NOW(3), NOW(3));

SET @phone_1 = LAST_INSERT_ID();
SET @phone_2 = @phone_1 + 1;
SET @phone_3 = @phone_1 + 2;

-- ---- 2.4 技能组 ----
INSERT INTO `cc_res_skill_group` (
    `name`, `merchant_id`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES (
    'E2E测试技能组', 1001, 1, 0, NOW(3), NOW(3)
);

SET @sg_id = LAST_INSERT_ID();

-- ---- 2.5 坐席用户记录 ----
INSERT INTO `cc_res_mch_user` (
    `merchant_id`, `username`, `seat_number`, `call_extension_enable`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES
    (1001, '坐席A', '1001', 1, 1, 0, NOW(3), NOW(3)),
    (1001, '坐席B', '1002', 1, 1, 0, NOW(3), NOW(3));

SET @user_2001 = LAST_INSERT_ID();
SET @user_2002 = @user_2001 + 1;

-- ---- 2.6 分机记录 (绑定坐席用户, Kamailio 鉴权所需) ----
-- sip_domain 必须匹配商户 cc_mch_info.sip_domain
-- password/ha1 用于 Kamailio SIP Digest Auth
INSERT INTO `cc_res_extension` (
    `extension_number`, `password`, `sip_domain`, `ha1`, `ha1b`,
    `merchant_id`, `user_id`, `enable`, `bind_type`, `del_flag`,
    `created_time`, `updated_time`
) VALUES
    ('1001', 'e2e-test-1001', 'sip.merchant.yunshu.com',
     MD5(CONCAT('1001', ':', 'sip.merchant.yunshu.com', ':', 'e2e-test-1001')),
     MD5(CONCAT('1001@sip.merchant.yunshu.com', ':', 'sip.merchant.yunshu.com', ':', 'e2e-test-1001')),
     1001, @user_2001, 1, 1, 0, NOW(3), NOW(3)),
    ('1002', 'e2e-test-1002', 'sip.merchant.yunshu.com',
     MD5(CONCAT('1002', ':', 'sip.merchant.yunshu.com', ':', 'e2e-test-1002')),
     MD5(CONCAT('1002@sip.merchant.yunshu.com', ':', 'sip.merchant.yunshu.com', ':', 'e2e-test-1002')),
     1001, @user_2002, 1, 1, 0, NOW(3), NOW(3));

-- ---- 2.7 坐席 → 技能组 ----
INSERT INTO `cc_res_user_skill_group` (`user_id`, `skill_group_id`, `created_time`, `updated_time`)
VALUES (@user_2001, @sg_id, NOW(3), NOW(3)), (@user_2002, @sg_id, NOW(3), NOW(3));

-- ---- 2.8 号码 → 技能组 ----
INSERT INTO `cc_res_pool_phone_skill_group` (`pool_phone_id`, `skill_group_id`)
VALUES (@phone_1, @sg_id), (@phone_2, @sg_id), (@phone_3, @sg_id);

-- ---- 2.9 Kamailio 位置预注册 (坐席 → SIPp UAS, 域名对齐 sip_domain) ----
INSERT INTO `cc_res_location` (
    `username`, `domain`, `contact`, `received`, `expires`,
    `callid`, `cseq`, `last_modified`, `ruid`, `q`, `user_agent`
) VALUES
    ('1001', 'sip.merchant.yunshu.com',
     'sip:1001@192.168.107.0:6060', 'sip:192.168.107.0:6060',
     '2030-01-01 00:00:00', 'test-reg-1001', 1, NOW(3), 'e2e-test-ruid-1001', 1.0, 'SIPp-e2e-test'),
    ('1002', 'sip.merchant.yunshu.com',
     'sip:1002@192.168.107.0:6060', 'sip:192.168.107.0:6060',
     '2030-01-01 00:00:00', 'test-reg-1002', 1, NOW(3), 'e2e-test-ruid-1002', 1.0, 'SIPp-e2e-test');

-- ---- 2.10 汇总输出 ----
SELECT 'seed_data' AS `item`, @gw_id AS `gateway_id`, @pool_id AS `pool_id`,
       @sg_id AS `skill_group_id`, @phone_1 AS `phone_1_id`,
       @phone_2 AS `phone_2_id`, @phone_3 AS `phone_3_id`,
       @user_2001 AS `user_2001_id`, @user_2002 AS `user_2002_id`;
