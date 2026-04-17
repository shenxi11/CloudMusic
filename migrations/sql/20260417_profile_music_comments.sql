CREATE TABLE IF NOT EXISTS `music_profile`.`music_comment_threads` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `music_path` VARCHAR(500) NOT NULL COMMENT '歌曲线程主标识',
  `source` VARCHAR(32) NOT NULL DEFAULT 'catalog' COMMENT '歌曲来源',
  `source_id` VARCHAR(64) DEFAULT NULL COMMENT '外部源真实ID',
  `music_title` VARCHAR(255) DEFAULT NULL COMMENT '歌曲标题',
  `artist` VARCHAR(255) DEFAULT NULL COMMENT '歌手',
  `cover_art_path` VARCHAR(500) DEFAULT NULL COMMENT '封面路径',
  `root_comment_count` INT NOT NULL DEFAULT 0 COMMENT '主评论数',
  `total_comment_count` INT NOT NULL DEFAULT 0 COMMENT '总评论数',
  `last_commented_at` TIMESTAMP NULL DEFAULT NULL COMMENT '最后评论时间',
  `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_music_path` (`music_path`),
  KEY `idx_source_source_id` (`source`, `source_id`),
  KEY `idx_last_commented_at` (`last_commented_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='歌曲评论线程';

CREATE TABLE IF NOT EXISTS `music_profile`.`music_comments` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `thread_id` BIGINT NOT NULL COMMENT '评论线程ID',
  `root_comment_id` BIGINT NOT NULL DEFAULT 0 COMMENT '所属主评论ID，主评论固定为0',
  `reply_to_comment_id` BIGINT DEFAULT NULL COMMENT '本次回复目标评论ID',
  `user_account` VARCHAR(50) NOT NULL COMMENT '评论用户账号',
  `username_snapshot` VARCHAR(100) NOT NULL COMMENT '评论用户名快照',
  `avatar_path_snapshot` VARCHAR(500) DEFAULT NULL COMMENT '评论头像快照',
  `reply_to_user_account` VARCHAR(50) DEFAULT NULL COMMENT '被回复用户账号',
  `reply_to_username_snapshot` VARCHAR(100) DEFAULT NULL COMMENT '被回复用户名快照',
  `content` TEXT NOT NULL COMMENT '评论内容',
  `is_deleted` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已删除',
  `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_thread_root_created` (`thread_id`, `root_comment_id`, `created_at`),
  KEY `idx_root_created` (`root_comment_id`, `created_at`),
  KEY `idx_user_created` (`user_account`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='歌曲评论与回复';
