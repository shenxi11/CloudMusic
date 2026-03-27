CREATE TABLE IF NOT EXISTS `music_profile`.`user_playlists` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_account` VARCHAR(50) NOT NULL COMMENT '用户账号',
  `name` VARCHAR(128) NOT NULL COMMENT '歌单名称',
  `description` VARCHAR(1000) DEFAULT NULL COMMENT '歌单简介',
  `cover_path` VARCHAR(500) DEFAULT NULL COMMENT '歌单封面路径',
  `track_count` INT NOT NULL DEFAULT 0 COMMENT '歌曲数量',
  `total_duration_sec` FLOAT NOT NULL DEFAULT 0 COMMENT '总时长（秒）',
  `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_account_updated_at` (`user_account`, `updated_at`),
  KEY `idx_user_account_created_at` (`user_account`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户私有歌单';

CREATE TABLE IF NOT EXISTS `music_profile`.`user_playlist_items` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `playlist_id` BIGINT NOT NULL COMMENT '歌单ID',
  `user_account` VARCHAR(50) NOT NULL COMMENT '用户账号',
  `position` INT NOT NULL COMMENT '歌单排序',
  `music_path` VARCHAR(500) NOT NULL COMMENT '音乐路径',
  `music_title` VARCHAR(255) DEFAULT NULL COMMENT '歌曲名称',
  `artist` VARCHAR(255) DEFAULT NULL COMMENT '歌手',
  `album` VARCHAR(255) DEFAULT NULL COMMENT '专辑',
  `duration_sec` FLOAT DEFAULT NULL COMMENT '时长（秒）',
  `is_local` TINYINT(1) DEFAULT 0 COMMENT '是否本地歌曲',
  `cover_art_path` VARCHAR(500) DEFAULT NULL COMMENT '封面路径',
  `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '加入时间',
  `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_playlist_music` (`playlist_id`, `music_path`),
  UNIQUE KEY `uk_playlist_position` (`playlist_id`, `position`),
  KEY `idx_user_playlist` (`user_account`, `playlist_id`),
  KEY `idx_playlist_created_at` (`playlist_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户歌单歌曲';
