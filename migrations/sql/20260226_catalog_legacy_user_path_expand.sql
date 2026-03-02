-- 修复遗留兼容接口 /users/add_music 在长路径场景写入失败的问题
-- 根因: music_users.user_path.music_path 为 VARCHAR(50)，无法容纳当前音乐相对路径。
-- 方案: 若 user_path 存在，则将 music_path 扩容到 VARCHAR(500)。

SET @has_user_path := (
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
      AND table_name = 'user_path'
);

SET @ddl := IF(
    @has_user_path > 0,
    'ALTER TABLE `user_path` MODIFY COLUMN `music_path` VARCHAR(500) NOT NULL',
    'SELECT \"skip: user_path table not found\"'
);

PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
