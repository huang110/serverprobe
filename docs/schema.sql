-- ServerProbe MySQL schema (manual initialization)
-- 注意：Server 启动时 GORM AutoMigrate 也会自动创建/补齐这些表，
-- 因此本文件主要用于手动初始化数据库或 DBA 审核。
--
-- 原始指标分区表 metrics_raw_YYYYMM 由 Server 启动时自动创建，
-- 不在此文件中写死月份。

CREATE DATABASE IF NOT EXISTS `serverprobe`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE `serverprobe`;

CREATE TABLE IF NOT EXISTS `agents` (
  `id` varchar(64) NOT NULL,
  `hostname` varchar(255) NOT NULL,
  `ip` varchar(45) DEFAULT NULL,
  `internal_ip` varchar(45) DEFAULT NULL,
  `os` varchar(50) DEFAULT NULL,
  `os_version` varchar(100) DEFAULT NULL,
  `kernel_version` varchar(100) DEFAULT NULL,
  `cpu_cores` int NOT NULL DEFAULT 0,
  `memory_total` bigint NOT NULL DEFAULT 0,
  `disk_total` bigint NOT NULL DEFAULT 0,
  `arch` varchar(20) DEFAULT NULL,
  `agent_version` varchar(50) DEFAULT NULL,
  `token` varchar(128) NOT NULL,
  `token_expires_at` bigint NOT NULL DEFAULT 0,
  `last_heartbeat_at` bigint NOT NULL DEFAULT 0,
  `status` varchar(20) NOT NULL DEFAULT 'online',
  `lite_mode` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_hostname` (`hostname`),
  KEY `idx_ip` (`ip`),
  KEY `idx_token_expires_at` (`token_expires_at`),
  KEY `idx_last_heartbeat_at` (`last_heartbeat_at`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `agent_labels` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `agent_id` varchar(64) NOT NULL,
  `key` varchar(100) NOT NULL,
  `value` varchar(255) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_key` (`agent_id`, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `metrics_aggregated` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `agent_id` varchar(64) NOT NULL,
  `granularity` varchar(10) NOT NULL,
  `timestamp` bigint NOT NULL,
  `cpu_avg` double DEFAULT NULL,
  `cpu_max` double DEFAULT NULL,
  `memory_avg` bigint DEFAULT NULL,
  `memory_max` bigint DEFAULT NULL,
  `load1_avg` double DEFAULT NULL,
  `load1_max` double DEFAULT NULL,
  `net_rx_bytes_sum` bigint DEFAULT NULL,
  `net_tx_bytes_sum` bigint DEFAULT NULL,
  `tcp_established_avg` double DEFAULT NULL,
  `sample_count` int DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_agent_granularity_time` (`agent_id`, `granularity`, `timestamp`),
  KEY `idx_metrics_aggregated_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `metrics_disk` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `agent_id` varchar(64) NOT NULL,
  `mount_point` varchar(255) NOT NULL,
  `timestamp` bigint NOT NULL,
  `device` varchar(100) DEFAULT NULL,
  `fs_type` varchar(50) DEFAULT NULL,
  `total` bigint DEFAULT NULL,
  `used` bigint DEFAULT NULL,
  `usage_percent` double DEFAULT NULL,
  `inode_percent` double DEFAULT NULL,
  `read_bytes` double DEFAULT NULL,
  `write_bytes` double DEFAULT NULL,
  `io_util` double DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_agent_mount_time` (`agent_id`, `mount_point`, `timestamp`),
  KEY `idx_metrics_disk_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `metrics_network` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `agent_id` varchar(64) NOT NULL,
  `interface` varchar(100) NOT NULL,
  `timestamp` bigint NOT NULL,
  `ip_address` varchar(45) DEFAULT NULL,
  `rx_bytes` bigint DEFAULT NULL,
  `tx_bytes` bigint DEFAULT NULL,
  `rx_packets` bigint DEFAULT NULL,
  `tx_packets` bigint DEFAULT NULL,
  `rx_errors` bigint DEFAULT NULL,
  `tx_errors` bigint DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_agent_iface_time` (`agent_id`, `interface`, `timestamp`),
  KEY `idx_metrics_network_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `alert_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `agent_id` varchar(64) DEFAULT NULL,
  `metric` varchar(50) NOT NULL,
  `condition` varchar(20) NOT NULL,
  `threshold` double NOT NULL,
  `duration` int NOT NULL DEFAULT 60,
  `severity` varchar(20) NOT NULL DEFAULT 'warning',
  `enabled` tinyint(1) NOT NULL DEFAULT 1,
  `expression` text,
  `notify_url` varchar(500) DEFAULT NULL,
  `webhook_url` varchar(500) DEFAULT NULL,
  `message` varchar(500) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_agent_id` (`agent_id`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `alert_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `rule_id` bigint unsigned NOT NULL,
  `agent_id` varchar(64) NOT NULL,
  `metric` varchar(50) NOT NULL,
  `threshold` double NOT NULL,
  `actual_value` double NOT NULL,
  `severity` varchar(20) NOT NULL,
  `message` text,
  `status` varchar(20) NOT NULL DEFAULT 'firing',
  `fired_at` bigint NOT NULL,
  `resolved_at` bigint DEFAULT NULL,
  `notified_at` bigint DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_rule_id` (`rule_id`),
  KEY `idx_alert_records_agent_id` (`agent_id`),
  KEY `idx_severity` (`severity`),
  KEY `idx_alert_records_status` (`status`),
  KEY `idx_fired_at` (`fired_at`),
  KEY `idx_resolved_at` (`resolved_at`),
  KEY `idx_alert_records_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
