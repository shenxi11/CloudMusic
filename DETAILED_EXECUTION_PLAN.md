# 微服务改造详细执行计划（已执行）

> 说明：以下计划已全部执行完成，状态均为 `DONE`。

## Plan A: 数据边界继续拆分（Profile -> Media）

1. `DONE`：Profile 独立 schema（`music_profile`）  
- 收藏/历史迁移到 `music_profile`  
- 仍保留对 `music_users.music_files` 的只读关联

2. `DONE`：Media 独立 schema（`music_media`）  
- 新增 `music_media.media_lyrics_map`  
- `media-service` 优先查私有表，缺失时回退 catalog 并回填

3. `DONE`：配置显式化  
- 新增 `schemas.profile`、`schemas.catalog`、`schemas.media`

## Plan B: 迁移治理自动化

1. `DONE`：构建统一迁移器 `migrator`  
- 按服务过滤 SQL：`event/profile/media/catalog`
- 自动维护各 schema 的 `schema_migrations`
- checksum 防篡改校验

2. `DONE`：迁移入口脚本  
- `migrate.sh`
- `scripts/migrate_all.sh`

3. `DONE`：启动流程自动迁移  
- `start_split.sh` 在服务启动前自动执行迁移（幂等）

## Plan C: 稳定性与验收

1. `DONE`：全量编译和测试  
- `go test ./...`
- `./build.sh`

2. `DONE`：全栈启动验证  
- `./start_split.sh` 启动全部服务
- 健康检查全通过

3. `DONE`：关键接口验证  
- `/user/favorites/*` 在新 profile schema 正常读写
- `/uploads/<folder>/lrc` 在新 media schema 正常返回歌词

4. `DONE`：迁移幂等验证  
- 重复执行 `./migrate.sh` 为“跳过已执行迁移”
