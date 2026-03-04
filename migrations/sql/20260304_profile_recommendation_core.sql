-- 推荐系统核心表（profile schema）

CREATE TABLE IF NOT EXISTS `user_recommendation_feedback` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `user_account` VARCHAR(64) NOT NULL,
    `song_id` VARCHAR(500) NOT NULL,
    `event_type` VARCHAR(32) NOT NULL,
    `play_ms` BIGINT NOT NULL DEFAULT 0,
    `duration_ms` BIGINT NOT NULL DEFAULT 0,
    `scene` VARCHAR(32) NOT NULL DEFAULT 'home',
    `request_id` VARCHAR(64) DEFAULT NULL,
    `model_version` VARCHAR(64) DEFAULT NULL,
    `event_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_event_time` (`user_account`, `event_at`),
    KEY `idx_song_event_time` (`song_id`, `event_at`),
    KEY `idx_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐反馈行为事件';

CREATE TABLE IF NOT EXISTS `recommendation_model_status` (
    `model_name` VARCHAR(64) NOT NULL,
    `model_version` VARCHAR(64) NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'ready',
    `metrics_json` JSON DEFAULT NULL,
    `trained_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`model_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐模型状态';

CREATE TABLE IF NOT EXISTS `recommendation_training_jobs` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `task_id` VARCHAR(64) NOT NULL,
    `model_name` VARCHAR(64) NOT NULL,
    `model_version` VARCHAR(64) DEFAULT NULL,
    `force_full` TINYINT(1) NOT NULL DEFAULT 0,
    `status` VARCHAR(32) NOT NULL DEFAULT 'queued',
    `trigger_by` VARCHAR(64) DEFAULT NULL,
    `error_message` VARCHAR(512) DEFAULT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `finished_at` TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_task_id` (`task_id`),
    KEY `idx_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐模型训练任务';

INSERT INTO `recommendation_model_status` (`model_name`, `model_version`, `status`, `metrics_json`, `trained_at`)
VALUES ('rule_hybrid', 'rule_hybrid_v1', 'ready', JSON_OBJECT('algo','rule_hybrid','note','bootstrap'), NOW())
ON DUPLICATE KEY UPDATE `model_name` = `model_name`;
