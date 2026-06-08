-- ============================================================================
-- 云枢 SIP 端到端测试数据初始化
--
-- 用法: docker exec -i cc-mysql mysql -uroot -pdb123456 yunshu < seed_test_data.sql
--
-- 前置: 已通过 yunshu installer install 初始化基础数据 (商户/分机/FS 节点等)
-- ============================================================================

SET NAMES utf8mb4;

-- ---- 1. SIP 中继网关 (Model=2 IP 直连, 指向 SIPp 客户 UAS) ----
INSERT INTO `cc_tel_gateway` (
    `name`, `description`, `model`, `realm`, `port`,
    `concurrency`, `priority`, `enable`, `del_flag`,
    `codec_prefs`, `created_time`, `updated_time`
) VALUES (
    'test-trunk', 'E2E测试外呼网关', 2, '192.168.107.0', '6080',
    100, 1, 1, 0,
    'PCMU,PCMA', NOW(), NOW()
)
ON DUPLICATE KEY UPDATE
    `model` = VALUES(`model`), `realm` = VALUES(`realm`), `port` = VALUES(`port`),
    `concurrency` = VALUES(`concurrency`), `enable` = VALUES(`enable`);

-- 获取网关 ID
SET @gw_id = (SELECT `id` FROM `cc_tel_gateway` WHERE `name` = 'test-trunk' AND `del_flag` = 0 LIMIT 1);

-- ---- 2. 号码池 ----
INSERT INTO `cc_tel_pool` (
    `merchant_id`, `name`, `gateway_id`, `selection_strategy`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES (
    1001, 'E2E测试号码池', @gw_id, 'CONCURRENCY', 1, 0, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE `gateway_id` = VALUES(`gateway_id`), `enable` = VALUES(`enable`);

SET @pool_id = (SELECT `id` FROM `cc_tel_pool` WHERE `merchant_id` = 1001 AND `name` = 'E2E测试号码池' AND `del_flag` = 0 LIMIT 1);

-- ---- 3. 外呼号码 (3 个测试 DID) ----
INSERT INTO `cc_res_pool_phone` (
    `pool_id`, `phone`, `concurrency`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES
    (@pool_id, '01088886666', 10, 1, 0, NOW(), NOW()),
    (@pool_id, '01088887777', 10, 1, 0, NOW(), NOW()),
    (@pool_id, '01088888888', 10, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE `concurrency` = VALUES(`concurrency`), `enable` = VALUES(`enable`);

SET @phone_1 = (SELECT `id` FROM `cc_res_pool_phone` WHERE `pool_id` = @pool_id AND `phone` = '01088886666' LIMIT 1);
SET @phone_2 = (SELECT `id` FROM `cc_res_pool_phone` WHERE `pool_id` = @pool_id AND `phone` = '01088887777' LIMIT 1);
SET @phone_3 = (SELECT `id` FROM `cc_res_pool_phone` WHERE `pool_id` = @pool_id AND `phone` = '01088888888' LIMIT 1);

-- ---- 4. 技能组 ----
INSERT INTO `cc_res_skill_group` (
    `name`, `merchant_id`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES (
    'E2E测试技能组', 1001, 1, 0, NOW(), NOW()
)
ON DUPLICATE KEY UPDATE `enable` = VALUES(`enable`);

SET @sg_id = (SELECT `id` FROM `cc_res_skill_group` WHERE `merchant_id` = 1001 AND `name` = 'E2E测试技能组' AND `del_flag` = 0 LIMIT 1);

-- ---- 5. 坐席用户记录 (对齐 cc_res_extension 的 user_id=2001/2002) ----
INSERT INTO `cc_res_mch_user` (
    `merchant_id`, `username`, `seat_number`, `call_extension_enable`, `enable`, `del_flag`,
    `created_time`, `updated_time`
) VALUES
    (1001, '坐席A', '1001', 1, 1, 0, NOW(), NOW()),
    (1001, '坐席B', '1002', 1, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE `enable` = VALUES(`enable`), `call_extension_enable` = VALUES(`call_extension_enable`);

SET @user_2001 = (SELECT `id` FROM `cc_res_mch_user` WHERE `merchant_id` = 1001 AND `seat_number` = '1001' AND `del_flag` = 0 LIMIT 1);
SET @user_2002 = (SELECT `id` FROM `cc_res_mch_user` WHERE `merchant_id` = 1001 AND `seat_number` = '1002' AND `del_flag` = 0 LIMIT 1);

-- ---- 6. 坐席 → 技能组 ----
INSERT IGNORE INTO `cc_res_user_skill_group` (`user_id`, `skill_group_id`, `created_time`, `updated_time`)
VALUES (2001, @sg_id, NOW(), NOW()), (2002, @sg_id, NOW(), NOW());

-- ---- 7. 号码 → 技能组 ----
INSERT IGNORE INTO `cc_res_pool_phone_skill_group` (`pool_phone_id`, `skill_group_id`)
VALUES (@phone_1, @sg_id), (@phone_2, @sg_id), (@phone_3, @sg_id);

-- ---- 8. Kamailio 位置预注册 (坐席 1001 → SIPp UAS 192.168.107.0:6060) ----
INSERT INTO `cc_res_location` (
    `username`, `domain`, `contact`, `received`, `expires`,
    `callid`, `cseq`, `last_modified`, `ruid`, `q`, `user_agent`
) VALUES (
    '1001', 'sip.yunshu.local',
    'sip:1001@192.168.107.0:6060', 'sip:192.168.107.0:6060',
    '2030-01-01 00:00:00', 'test-reg-1001', 1, NOW(), 'e2e-test-ruid-1001', 1.0, 'SIPp-e2e-test'
) ON DUPLICATE KEY UPDATE
    `contact` = VALUES(`contact`), `received` = VALUES(`received`),
    `expires` = VALUES(`expires`), `cseq` = VALUES(`cseq`), `last_modified` = NOW();

-- ---- 9. 汇总输出 ----
SELECT 'seed_data' AS `item`, @gw_id AS `gateway_id`, @pool_id AS `pool_id`,
       @sg_id AS `skill_group_id`, @phone_1 AS `phone_1_id`,
       @phone_2 AS `phone_2_id`, @phone_3 AS `phone_3_id`;
