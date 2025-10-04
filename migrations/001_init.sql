-- Create database
CREATE DATABASE IF NOT EXISTS url_shortener CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE url_shortener;

-- Short link mapping table
CREATE TABLE IF NOT EXISTS `url_mappings` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Auto-increment ID',
  `short_code` VARCHAR(15) NOT NULL COMMENT 'Short code',
  `original_url` VARCHAR(2048) NOT NULL COMMENT 'Original URL',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation time',
  `expired_at` TIMESTAMP NULL DEFAULT NULL COMMENT 'Expiration time',
  `visit_count` BIGINT UNSIGNED DEFAULT 0 COMMENT 'Visit count',
  `status` TINYINT DEFAULT 1 COMMENT 'Status: 1-active, 0-disabled',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_short_code` (`short_code`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_expired_at` (`expired_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='URL mapping table';

-- Visit log table (optional, for analytics)
CREATE TABLE IF NOT EXISTS `visit_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_code` VARCHAR(15) NOT NULL,
  `visited_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `ip` VARCHAR(45) DEFAULT NULL,
  `user_agent` VARCHAR(512) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_short_code` (`short_code`),
  KEY `idx_visited_at` (`visited_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Visit log table';
