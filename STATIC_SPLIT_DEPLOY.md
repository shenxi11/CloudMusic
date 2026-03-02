# 静态资源分离改造说明（兼容现有接口）

## 目标
- 保持现有接口路径不变（客户端无需改接口路径）。
- 将大流量静态资源从 Go 应用中分离到 Nginx 直出。
- Go 服务仅处理业务 API 和必须依赖数据库的特殊资源接口。

## 详细 Plan
1. 识别兼容点  
   - 维持 `/uploads/*`、`/video/*`、`/videos`、`/video/stream`、`/stream` 等路径不变。  
   - 保留 `/uploads/<folder>/lrc` 走后端（该接口依赖数据库映射歌词路径）。
2. 解耦监听地址和公开地址  
   - Go 后端改为仅监听 `127.0.0.1:18080`（内网端口）。  
   - 对外仍由 `http://<域名>:8080` 提供服务。
3. 引入 Nginx 网关（成熟方案）  
   - `location /uploads/`：Nginx 直出音频/封面等静态文件。  
   - `location /video/`：Nginx 直出视频文件（支持 Range）。  
   - `location = /video/stream`、`location ~ ^/uploads/.+/lrc$`：回源 Go。  
   - 其他 API 路径全部回源 Go。
4. 无缝切换  
   - 先启动 Go 内网服务，再启动 Nginx 外网网关。  
   - 对外入口仍是原路径，不影响客户端接口调用。
5. 回归验证  
   - 健康检查、登录、文件列表、视频列表、视频流 URL、视频分片、歌词接口逐项验证。  
6. 回滚预案  
   - 一条命令停止 split 架构；恢复到旧模式时直接运行原 `start.sh`。

## 已完成改造
- Go 配置新增字段：`public_port`、`public_base_url`。  
  文件：`internal/common/config/config.go`
- URL 生成逻辑支持“内部端口”和“公开地址”分离。  
  文件：`cmd/monolith/main.go`
- 新增 split 配置并切到内外分离：  
  - 后端监听：`127.0.0.1:18080`  
  - 对外地址：`http://slcdut.xyz:8080`  
  文件：`configs/config.yaml`、`config.yaml`
- 新增 Nginx split 配置：  
  文件：`deploy/nginx/nginx.split.conf`
- 第 1 个微服务拆分：认证服务独立部署  
  - 新增服务入口：`cmd/auth/main.go`  
  - 网关将 `/users/register`、`/users/login` 转发到认证服务（`127.0.0.1:18081`）
- 第 2 个微服务拆分：内容服务独立部署  
  - 新增服务入口：`cmd/catalog/main.go`  
  - 网关将 `/files`、`/file`、`/stream`、`/get_music`、`/music/artist`、`/music/search`、`/artist/search` 转发到内容服务（`127.0.0.1:18082`）
- 第 3 个微服务拆分：用户行为服务独立部署  
  - 新增服务入口：`cmd/profile/main.go`  
  - 网关将 `/user/favorites/*`、`/user/history/*` 转发到用户行为服务（`127.0.0.1:18083`）
- 第 4 个微服务拆分：媒体服务独立部署  
  - 新增服务入口：`cmd/media/main.go`  
  - 网关将 `/upload`、`/lrc`、`/download`、`/files/*` 以及 `/uploads/*/lrc` 转发到媒体服务（`127.0.0.1:18084`）
- 第 5 个微服务拆分：视频服务独立部署  
  - 新增服务入口：`cmd/video/main.go`  
  - 网关将 `/videos`、`/video/stream` 转发到视频服务（`127.0.0.1:18085`）  
  - 视频文件体 `/video/*` 仍由 Nginx 静态直出
- Phase 5 第一步：最小事件总线  
  - 新增事件总线组件：`internal/common/eventbus`  
  - `profile-service` 发布用户行为事件（收藏/历史变更）  
  - 新增 `event-worker` 消费器：`cmd/eventworker/main.go`  
  - 事件持久化表：`domain_events`
- Phase 5 第二步：事件可靠性增强  
  - `profile-service` 新增 `event_outbox`（失败事件落库 + 后台重试补偿）  
  - `event-worker` 新增重试 + `domain_event_dlq` 死信落库  
  - 迁移脚本：`migrations/sql/20260226_event_reliability.sql`
- Phase 5 第三步：事件总线升级  
  - Redis Pub/Sub 升级为 Redis Streams + Consumer Group  
  - 保持 `eventbus.Publisher/Subscriber` 调用接口不变  
  - `event-worker` 改为流消费并 ACK
  - 增加 pending 接管机制（`XPENDING + XCLAIM`），恢复异常消费者遗留消息
- Phase 6 第一步：profile 数据 schema 拆分  
  - 新增独立 schema：`music_profile`  
  - `profile-service` 读写 `music_profile.user_favorite_music` / `music_profile.user_play_history`  
  - 与 `catalog` 的 `music_users.music_files` 通过跨 schema 只读关联  
  - 迁移脚本：`migrations/sql/20260226_profile_schema_split.sql`
- Phase 6 第二步：media 数据 schema 拆分  
  - 新增独立 schema：`music_media`  
  - `media-service` 读写 `music_media.media_lyrics_map`  
  - 启动时从 `music_users.music_files` 同步歌词映射  
  - 迁移脚本：`migrations/sql/20260226_media_schema_split.sql`
- Phase 7：迁移治理自动化  
  - 新增迁移器：`cmd/migrator/main.go`  
  - `start_split.sh` 启动前自动执行迁移（幂等）  
  - 各 schema 维护独立 `schema_migrations`
- 新增启动/停止脚本：  
  - `start_split.sh`  
  - `stop_split.sh`  
  - `scripts/start_split_stack.sh`  
  - `scripts/stop_split_stack.sh`

## 启动方式（新架构）
```bash
cd /home/shen/microservice-deploy
./start_split.sh
```

## 停止方式（新架构）
```bash
cd /home/shen/microservice-deploy
./stop_split.sh
```

## 验证命令
```bash
# 网关健康（对外入口）
curl http://127.0.0.1:8080/health

# 后端健康（内网端口）
curl http://127.0.0.1:18080/health

# 视频列表
curl http://127.0.0.1:8080/videos

# 视频流 URL
curl -H 'Content-Type: application/json' \
  -d '{"path":"an_hao.mp4"}' \
  http://127.0.0.1:8080/video/stream

# 视频分片（206）
curl -I -H 'Range: bytes=0-1023' \
  http://127.0.0.1:8080/video/an_hao.mp4
```

## 回滚方案
```bash
cd /home/shen/microservice-deploy
./stop_split.sh

# 使用旧方式直接启动 Go（单体直出）
./start.sh
```
