CREATE INDEX idx_online_hot_chart
ON `music_profile`.`user_play_history` (`is_local`, `play_time`, `music_path`);
