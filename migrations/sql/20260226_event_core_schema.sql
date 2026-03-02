-- Phase 5.x 基础核心表（空库首启可用）
-- 目标:
-- 1) 初始化认证与兼容接口依赖表: users, user_path
-- 2) 初始化内容域表: artists, music_files
-- 3) 保证脚本可重复执行（IF NOT EXISTS）

CREATE TABLE IF NOT EXISTS `users` (
    `account` VARCHAR(50) NOT NULL,
    `password` VARCHAR(255) NOT NULL,
    `username` VARCHAR(100) NOT NULL,
    PRIMARY KEY (`account`),
    UNIQUE KEY `uk_users_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `artists` (
    `id` INT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(255) NOT NULL COMMENT '歌手名称',
    `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_artists_name` (`name`),
    KEY `idx_artists_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='歌手表';

CREATE TABLE IF NOT EXISTS `music_files` (
    `id` INT NOT NULL AUTO_INCREMENT,
    `path` VARCHAR(255) NOT NULL,
    `title` VARCHAR(255) NOT NULL,
    `artist` VARCHAR(255) DEFAULT '',
    `album` VARCHAR(255) DEFAULT '',
    `duration_sec` DOUBLE NOT NULL DEFAULT 0,
    `size_bytes` BIGINT NOT NULL DEFAULT 0,
    `file_type` VARCHAR(32) NOT NULL,
    `is_audio` TINYINT(1) NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `lrc_path` VARCHAR(255) DEFAULT NULL,
    `cover_art_path` VARCHAR(255) DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_music_files_path` (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='音乐文件元数据';

CREATE TABLE IF NOT EXISTS `user_path` (
    `id` INT NOT NULL AUTO_INCREMENT,
    `username` VARCHAR(100) DEFAULT NULL,
    `music_path` VARCHAR(500) NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_user_path_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户路径（兼容旧接口）';
