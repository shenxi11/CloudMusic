-- Phase 6.2: media-service schema 拆分（媒体歌词索引）
-- 目标:
-- 1) 新建独立 schema: music_media
-- 2) 初始化 media_lyrics_map
-- 3) 从 music_users.music_files 同步现有歌词映射

CREATE DATABASE IF NOT EXISTS `music_media` DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `music_media`.`media_lyrics_map` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `music_path` VARCHAR(500) NOT NULL,
    `lrc_path` VARCHAR(500) NOT NULL,
    `source` VARCHAR(32) NOT NULL DEFAULT 'catalog',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_music_path` (`music_path`),
    KEY `idx_lrc_path` (`lrc_path`),
    KEY `idx_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='媒体服务歌词索引';

SET @music_files_src_exists := (
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = 'music_users'
      AND table_name = 'music_files'
);
SET @media_sql := IF(
    @music_files_src_exists > 0,
    'INSERT INTO `music_media`.`media_lyrics_map` (`music_path`, `lrc_path`, `source`)
     SELECT m.`path`, m.`lrc_path`, ''catalog''
     FROM `music_users`.`music_files` m
     WHERE m.`lrc_path` IS NOT NULL AND m.`lrc_path` <> ''''
     ON DUPLICATE KEY UPDATE
         `lrc_path` = VALUES(`lrc_path`),
         `source` = ''catalog'',
         `updated_at` = CURRENT_TIMESTAMP',
    'SELECT ''skip media_lyrics_map sync: source table music_users.music_files not found'''
);
PREPARE stmt_media FROM @media_sql;
EXECUTE stmt_media;
DEALLOCATE PREPARE stmt_media;
