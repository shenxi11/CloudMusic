-- Recommendation schema draft (manual apply)
-- Date: 2026-03-03
-- Target: MySQL 8.0+
-- Note: This file is a design draft and is not auto-loaded by migrator.

CREATE SCHEMA IF NOT EXISTS music_recommend
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE music_recommend;

CREATE TABLE IF NOT EXISTS model_versions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  model_name VARCHAR(64) NOT NULL,
  model_version VARCHAR(64) NOT NULL,
  model_type ENUM('als', 'lightfm', 'content', 'hybrid') NOT NULL DEFAULT 'hybrid',
  status ENUM('training', 'ready', 'failed', 'archived') NOT NULL DEFAULT 'training',
  params_json JSON NULL,
  metrics_json JSON NULL,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_model_name_version (model_name, model_version),
  KEY idx_model_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS training_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  task_id VARCHAR(64) NOT NULL,
  model_name VARCHAR(64) NOT NULL,
  model_version VARCHAR(64) NULL,
  status ENUM('queued', 'running', 'success', 'failed', 'canceled') NOT NULL DEFAULT 'queued',
  trigger_by VARCHAR(64) NULL,
  trigger_source VARCHAR(32) NOT NULL DEFAULT 'admin',
  error_message VARCHAR(512) NULL,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_training_task (task_id),
  KEY idx_training_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_account VARCHAR(64) NOT NULL,
  song_id VARCHAR(512) NOT NULL,
  event_type ENUM('impression', 'click', 'play', 'finish', 'like', 'skip', 'share', 'dislike') NOT NULL,
  play_ms INT UNSIGNED NOT NULL DEFAULT 0,
  duration_ms INT UNSIGNED NULL,
  scene VARCHAR(32) NOT NULL DEFAULT 'home',
  request_id VARCHAR(64) NULL,
  model_version VARCHAR(64) NULL,
  device_id VARCHAR(128) NULL,
  event_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_user_event_time (user_account, event_at),
  KEY idx_song_event_time (song_id, event_at),
  KEY idx_event_type_time (event_type, event_at),
  KEY idx_request_id (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS song_features (
  song_id VARCHAR(512) NOT NULL,
  title VARCHAR(255) NULL,
  artist VARCHAR(255) NULL,
  album VARCHAR(255) NULL,
  duration_sec DECIMAL(10,3) NULL,
  file_size BIGINT UNSIGNED NULL,
  bpm DECIMAL(8,3) NULL,
  energy DECIMAL(8,5) NULL,
  danceability DECIMAL(8,5) NULL,
  valence DECIMAL(8,5) NULL,
  acousticness DECIMAL(8,5) NULL,
  feature_json JSON NULL,
  embedding_json JSON NULL,
  feature_version VARCHAR(32) NOT NULL DEFAULT 'v1',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (song_id),
  KEY idx_artist_album (artist, album),
  KEY idx_feature_version (feature_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_profile_features (
  user_account VARCHAR(64) NOT NULL,
  embedding_json JSON NULL,
  preferred_artists_json JSON NULL,
  tag_weight_json JSON NULL,
  feature_version VARCHAR(32) NOT NULL DEFAULT 'v1',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_account),
  KEY idx_profile_feature_version (feature_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS similar_song_index (
  song_id VARCHAR(512) NOT NULL,
  model_version VARCHAR(64) NOT NULL,
  neighbors_json JSON NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (song_id),
  KEY idx_similar_model_version (model_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS recommendation_cache (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_account VARCHAR(64) NOT NULL,
  scene VARCHAR(32) NOT NULL DEFAULT 'home',
  request_hash CHAR(64) NOT NULL,
  model_version VARCHAR(64) NOT NULL,
  items_json JSON NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_recommend_cache (user_account, scene, request_hash),
  KEY idx_recommend_cache_exp (expires_at),
  KEY idx_recommend_cache_user_scene (user_account, scene, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
