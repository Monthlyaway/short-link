-- Migration to fix short_code column length from VARCHAR(10) to VARCHAR(15)
-- This migration is needed for existing databases created with the old schema

USE url_shortener;

-- Alter url_mappings table
ALTER TABLE `url_mappings`
  MODIFY COLUMN `short_code` VARCHAR(15) NOT NULL COMMENT 'Short code';

-- Alter visit_logs table
ALTER TABLE `visit_logs`
  MODIFY COLUMN `short_code` VARCHAR(15) NOT NULL;
