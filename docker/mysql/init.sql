-- 创建统一呼叫中心数据库
CREATE DATABASE IF NOT EXISTS `yunshu` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE `yunshu`;

-- =========================================================================
-- 1. Kamailio 核心版本及内部表 (version & location)
-- =========================================================================

-- Kamailio 系统表：版本控制表
CREATE TABLE IF NOT EXISTS `version` (
  `table_name` VARCHAR(32) NOT NULL,
  `table_version` INT UNSIGNED NOT NULL,
  PRIMARY KEY (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 播种 Kamailio 的各模块表版本号 (Kamailio v6.1.2 标准)
INSERT INTO `version` (`table_name`, `table_version`) 
VALUES ('kamailio_dispatcher', 4),
       ('location', 9),
       ('kamailio_rtpengine', 1),
       ('cc_res_extension', 7)
ON DUPLICATE KEY UPDATE `table_version` = VALUES(`table_version`);

-- Kamailio Usrloc 动态分机位置注册映射表 (必须包含 ruid 唯一记录字段)
CREATE TABLE IF NOT EXISTS `location` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `username` VARCHAR(64) NOT NULL DEFAULT '',
  `domain` VARCHAR(64) DEFAULT NULL,
  `contact` VARCHAR(512) NOT NULL DEFAULT '',
  `received` VARCHAR(512) DEFAULT NULL,
  `path` VARCHAR(512) DEFAULT NULL,
  `expires` DATETIME NOT NULL DEFAULT '2030-01-01 00:00:00',
  `q` FLOAT NOT NULL DEFAULT 1.0,
  `callid` VARCHAR(255) NOT NULL DEFAULT '',
  `cseq` INT NOT NULL DEFAULT 0,
  `last_modified` DATETIME NOT NULL DEFAULT '1900-01-01 00:00:00',
  `flags` INT NOT NULL DEFAULT 0,
  `cflags` INT NOT NULL DEFAULT 0,
  `user_agent` VARCHAR(255) NOT NULL DEFAULT '',
  `socket` VARCHAR(64) NOT NULL DEFAULT '',
  `methods` INT NOT NULL DEFAULT 0,
  `instance` VARCHAR(255) DEFAULT NULL,
  `reg_id` INT NOT NULL DEFAULT 0,
  `server_id` INT NOT NULL DEFAULT 0,
  `connection_id` INT NOT NULL DEFAULT 0,
  `keepalive` INT NOT NULL DEFAULT 0,
  `partition` INT NOT NULL DEFAULT 0,
  `ruid` VARCHAR(64) NOT NULL DEFAULT '',
  UNIQUE KEY `ruid_idx` (`ruid`),
  INDEX `account_contact_idx` (`username`,`domain`,`contact`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =========================================================================
-- 2. Kamailio 业务表 (subscriber & dispatcher)
-- =========================================================================

-- Note: kamailio_subscriber table removed; authentication utilizes the unified cc_res_extension table directly.

-- Dispatcher 负载均衡与心跳探测网关表 (直接对接 Go 后端管理的 GORM 表 kamailio_dispatcher)
CREATE TABLE IF NOT EXISTS `kamailio_dispatcher` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `set_id` INT NOT NULL DEFAULT 1,
  `destination` VARCHAR(192) NOT NULL DEFAULT '',
  `flags` INT NOT NULL DEFAULT 0,
  `priority` INT NOT NULL DEFAULT 0,
  `attrs` VARCHAR(128) NOT NULL DEFAULT '',
  `description` VARCHAR(64) NOT NULL DEFAULT '',
  `enable` TINYINT(1) NOT NULL DEFAULT 1,
  `del_flag` TINYINT(1) NOT NULL DEFAULT 0,
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- RTPEngine 媒体代理节点配置表 (直接对接 Go 后端管理的 GORM 表 kamailio_rtpengine)
CREATE TABLE IF NOT EXISTS `kamailio_rtpengine` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `set_id` INT NOT NULL DEFAULT 1,
  `rtpengine_sock` VARCHAR(192) NOT NULL DEFAULT '',
  `disabled` TINYINT(1) NOT NULL DEFAULT 0,
  `weight` INT NOT NULL DEFAULT 1,
  `description` VARCHAR(64) NOT NULL DEFAULT '',
  `del_flag` TINYINT(1) NOT NULL DEFAULT 0,
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =========================================================================
-- 3. Go 后端 GORM 核心表 (freeswitch & extension)
-- =========================================================================

-- FreeSWITCH 媒体节点配置表
CREATE TABLE IF NOT EXISTS `freeswitch` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `address` VARCHAR(128) NOT NULL DEFAULT '',
  `local_address` VARCHAR(128) NOT NULL DEFAULT '',
  `esl_port` INT NOT NULL DEFAULT 8021,
  `sip_port` INT NOT NULL DEFAULT 5060,
  `password` VARCHAR(64) NOT NULL DEFAULT 'ClueCon',
  `setid` INT NOT NULL DEFAULT 1,
  `weight` INT NOT NULL DEFAULT 100,
  `rweight` INT NOT NULL DEFAULT 100,
  `cc` INT NOT NULL DEFAULT 100,
  `cmd_port` INT NOT NULL DEFAULT 8085,
  `canary` TINYINT(1) NOT NULL DEFAULT 0,
  `enable` TINYINT(1) NOT NULL DEFAULT 1,
  `del_flag` TINYINT(1) NOT NULL DEFAULT 0,
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- FreeSWITCH 事件租约表 (多实例高可用分配)
CREATE TABLE IF NOT EXISTS `freeswitch_event_lease` (
  `fs_addr` VARCHAR(128) NOT NULL PRIMARY KEY,
  `owner` VARCHAR(128) NOT NULL,
  `lease_expiry` DATETIME NOT NULL,
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_freeswitch_event_lease_owner` (`owner`),
  INDEX `idx_freeswitch_event_lease_expiry` (`lease_expiry`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 坐席分机基础配置表 (直接对接 Kamailio 鉴权做多租户 HA1b 方案)
CREATE TABLE IF NOT EXISTS `cc_res_extension` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `extension_number` VARCHAR(64) NOT NULL DEFAULT '',
  `password` VARCHAR(64) NOT NULL DEFAULT '',
  `sip_domain` VARCHAR(64) NOT NULL DEFAULT '',
  `ha1` VARCHAR(64) NOT NULL DEFAULT '',
  `ha1b` VARCHAR(64) NOT NULL DEFAULT '',
  `merchant_id` INT NOT NULL DEFAULT 1,
  `user_id` INT NOT NULL DEFAULT 1,
  `enable` TINYINT(1) NOT NULL DEFAULT 1,
  `bind_type` INT NOT NULL DEFAULT 1,
  `del_flag` TINYINT(1) NOT NULL DEFAULT 0,
  `offline_at` DATETIME DEFAULT NULL,
  `created_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `idx_extension_merchant` (`extension_number`, `merchant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- =========================================================================
-- 4. 初始数据播种 (Seed Data)
-- =========================================================================

-- 播种 Kamailio Dispatcher：让信令网关把呼叫路由给名为 freeswitch 的容器
INSERT INTO `kamailio_dispatcher` (`set_id`, `destination`, `flags`, `priority`, `attrs`, `description`, `enable`, `del_flag`) 
VALUES (1, 'sip:freeswitch:5060', 0, 1, 'max-concurrency=100', 'Docker Internal FreeSWITCH Media Server', 1, 0)
ON DUPLICATE KEY UPDATE `destination` = VALUES(`destination`);

-- 播种 FreeSWITCH 节点：让 Go 后端 CTI 连接名为 freeswitch:8021 的容器 ESL 控制面
INSERT INTO `freeswitch` (`address`, `local_address`, `esl_port`, `sip_port`, `password`, `setid`, `enable`)
VALUES ('freeswitch', 'freeswitch', 8021, 5060, 'ClueCon', 1, 1);

-- 播种分机及账号：供测试终端注册 (1001 & 1002，使用 HA1/HA1b 哈希鉴权)
INSERT INTO `cc_res_extension` (`extension_number`, `password`, `sip_domain`, `ha1`, `ha1b`, `merchant_id`, `user_id`, `enable`, `bind_type`)
VALUES ('1001', '123456', 'sip.yunshu.local', '911d5196a061bdebf371a2106c58ab51', '36e61b804dd88fbd03f813409600b1f2', 1001, 2001, 1, 2),
       ('1002', '123456', 'sip.yunshu.local', '222d08151cdc54c2fdd179f82bd3f8da', '8350d2aae13cec6092dd2850462ab11a', 1001, 2002, 1, 2)
ON DUPLICATE KEY UPDATE `password` = VALUES(`password`), `ha1` = VALUES(`ha1`), `ha1b` = VALUES(`ha1b`), `merchant_id` = VALUES(`merchant_id`), `user_id` = VALUES(`user_id`);

-- 播种 Kamailio RTPEngine 媒体代理节点
INSERT INTO `kamailio_rtpengine` (`set_id`, `rtpengine_sock`, `disabled`, `weight`, `description`) 
VALUES (1, 'udp:rtpengine:2223', 0, 1, 'Default RTPEngine Media Proxy')
ON DUPLICATE KEY UPDATE `rtpengine_sock` = VALUES(`rtpengine_sock`);
