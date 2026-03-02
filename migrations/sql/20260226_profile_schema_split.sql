-- Phase 6.1: profile-service schema 拆分（先拆用户行为数据）
-- 目标:
-- 1) 新建独立 schema: music_profile
-- 2) 将 user_favorite_music / user_play_history 迁移到独立 schema
-- 3) 保留 catalog 元数据在 music_users.music_files

CREATE DATABASE IF NOT EXISTS `music_profile` DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `music_profile`.`user_favorite_music` (
    `id` int NOT NULL AUTO_INCREMENT,
    `user_account` varchar(50) NOT NULL COMMENT '用户账号',
    `music_path` varchar(500) NOT NULL COMMENT '音乐文件路径',
    `music_title` varchar(255) DEFAULT NULL COMMENT '音乐标题',
    `artist` varchar(255) DEFAULT NULL COMMENT '歌手',
    `duration_sec` float DEFAULT NULL COMMENT '时长（秒）',
    `is_local` tinyint(1) DEFAULT '0' COMMENT '是否本地音乐',
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '添加时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_music` (`user_account`,`music_path`),
    KEY `idx_user_account` (`user_account`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户喜欢的音乐';

CREATE TABLE IF NOT EXISTS `music_profile`.`user_play_history` (
    `id` int NOT NULL AUTO_INCREMENT,
    `user_account` varchar(50) NOT NULL COMMENT '用户账号',
    `music_path` varchar(500) NOT NULL COMMENT '音乐文件路径',
    `music_title` varchar(255) DEFAULT NULL COMMENT '音乐标题',
    `artist` varchar(255) DEFAULT NULL COMMENT '歌手',
    `album` varchar(255) DEFAULT NULL COMMENT '专辑',
    `duration_sec` float DEFAULT NULL COMMENT '时长（秒）',
    `is_local` tinyint(1) DEFAULT '0' COMMENT '是否本地音乐',
    `play_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '播放时间',
    PRIMARY KEY (`id`),
    KEY `idx_user_account` (`user_account`),
    KEY `idx_play_time` (`play_time`),
    KEY `idx_user_play_time` (`user_account`,`play_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户播放历史';

SET @favorite_src_exists := (
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = 'music_users'
      AND table_name = 'user_favorite_music'
);
SET @favorite_sql := IF(
    @favorite_src_exists > 0,
    'INSERT IGNORE INTO `music_profile`.`user_favorite_music` (`user_account`, `music_path`, `music_title`, `artist`, `duration_sec`, `is_local`, `created_at`)
     SELECT `user_account`, `music_path`, `music_title`, `artist`, `duration_sec`, `is_local`, `created_at`
     FROM `music_users`.`user_favorite_music`',
    'SELECT ''skip user_favorite_music migration: source table not found'''
);
PREPARE stmt_favorite FROM @favorite_sql;
EXECUTE stmt_favorite;
DEALLOCATE PREPARE stmt_favorite;

SET @history_src_exists := (
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = 'music_users'
      AND table_name = 'user_play_history'
);
SET @history_sql := IF(
    @history_src_exists > 0,
    'INSERT INTO `music_profile`.`user_play_history` (`user_account`, `music_path`, `music_title`, `artist`, `album`, `duration_sec`, `is_local`, `play_time`)
     SELECT h.`user_account`, h.`music_path`, h.`music_title`, h.`artist`, h.`album`, h.`duration_sec`, h.`is_local`, h.`play_time`
     FROM `music_users`.`user_play_history` h
     LEFT JOIN `music_profile`.`user_play_history` p
       ON p.`user_account` = h.`user_account`
      AND p.`music_path` = h.`music_path`
      AND p.`play_time` = h.`play_time`
     WHERE p.`id` IS NULL',
    'SELECT ''skip user_play_history migration: source table not found'''
);
PREPARE stmt_history FROM @history_sql;
EXECUTE stmt_history;
DEALLOCATE PREPARE stmt_history;
