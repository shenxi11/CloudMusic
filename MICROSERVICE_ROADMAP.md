# 微服务改造路线图（从当前单体分层到真实多服务）

## 现状评估
- 当前是“单进程 + DDD 分层”，不是“多服务部署”。
- 已有天然领域边界：`user`、`music`、`usermusic`、`artist`、`video`、`media`。
- 已有 Nginx 网关入口（`:8080`）和后端内网监听（`:18080`）。

## 目标架构（渐进式）
- 网关层：Nginx（后续可替换为 Envoy/Kong）
- 认证服务：`auth-service`（注册、登录）
- 内容服务：`catalog-service`（音乐/歌手查询）
- 用户行为服务：`profile-service`（喜欢、历史）
- 媒体服务：`media-service`（上传、歌词、下载授权）
- 视频服务：`video-service`（列表、流地址）

## 演进原则
1. 接口路径不变（客户端无感）
2. 先拆“低耦合高收益”边界（认证先行）
3. 每次只做一个可回滚的小改动
4. 网关统一流量切换，服务可灰度替换

## 分阶段计划

### Phase 0: 基线与可观测性（已完成）
- Nginx 前置 + Go 内网化（静态资源分离）。
- 对外入口保持 `:8080`。

### Phase 1: 认证服务独立（已完成）
- 新增 `auth-service` 二进制（`cmd/auth/main.go`）。
- 网关将 `/users/register`、`/users/login` 转发到 `auth-service`。
- 保持响应结构与原接口一致。

### Phase 2: 内容服务独立（已完成）
- 抽离 `/files`、`/file`、`/stream`、`/music/search`、`/music/artist`、`/artist/search`。
- 新服务：`catalog-service`（建议端口 `18082`）。
- 网关按路由转发，单体保留兼容一段时间后下线对应 handler。

### Phase 3: 用户行为服务独立（已完成）
- 抽离 `/user/favorites/*`、`/user/history/*`。
- 新服务：`profile-service`（建议端口 `18083`）。
- 后续可拆库（`user_favorite_music`、`user_play_history`）。

### Phase 4: 媒体服务独立（已完成）
- 抽离 `/upload`、`/download`、`/lrc` 等动态媒体接口。
- 静态大文件仍由 Nginx/CDN 直出。

### Phase 4.5: 视频服务独立（已完成）
- 抽离 `/videos`、`/video/stream`。
- 视频文件实体仍由 Nginx 直出 `/video/*`（支持 Range）。

### Phase 5: 数据与通信治理（进行中）
- 服务内私有 schema / 私有库（按服务边界）。
- 引入异步事件处理播放历史、收藏等行为：
  - 已落地最小事件总线（Redis Pub/Sub）。
  - `profile-service` 发布领域事件到 `music.domain.events.v1`。
  - `event-worker` 订阅并将事件落库到 `domain_events`。
- 事件可靠性增强（已完成第一轮）：
  - `profile-service` 新增 `event_outbox` 持久化与后台补偿投递。
  - `event-worker` 新增入库重试与 `domain_event_dlq` 死信队列落库。
  - 事件 ID 升级为纳秒时间戳 + 序列号 + 随机段，降低并发冲突风险。
- 事件总线升级（已完成）：
  - Redis Pub/Sub 升级为 Redis Streams + Consumer Group。
  - 消费确认从“尽力而为”变为 ACK 机制，支持消费端重启后继续处理。
  - 新增 pending 消息接管（XPENDING + XCLAIM），可恢复异常消费者遗留消息。
- 后续可升级为 Kafka/RabbitMQ，并完善跨服务事务（Outbox+Inbox）、消息重放与告警治理。

### Phase 6: 数据边界拆分（进行中）
- `profile-service` 已落地独立 schema（`music_profile`）：
  - 收藏表：`music_profile.user_favorite_music`
  - 历史表：`music_profile.user_play_history`
- `catalog-service` 元数据仍在 `music_users.music_files`，profile 通过跨 schema 只读关联封面信息。
- 新增迁移脚本：`migrations/sql/20260226_profile_schema_split.sql`
- `media-service` 已落地独立 schema（`music_media`）：
  - 歌词索引表：`music_media.media_lyrics_map`
  - 启动时自动从 `music_users.music_files` 同步歌词映射
- 新增迁移脚本：`migrations/sql/20260226_media_schema_split.sql`

### Phase 7: 迁移治理与发布流程（已完成）
- 新增统一迁移器：`cmd/migrator/main.go`
  - 按服务维度执行迁移：`event`、`profile`、`media`、`catalog`
  - 每个 schema 维护独立 `schema_migrations` 版本表
  - 校验脚本 checksum，防止同名脚本被篡改
- 新增一键迁移入口：
  - `migrate.sh`
  - `scripts/migrate_all.sh`
- 启动流程集成自动迁移：
  - `start_split.sh` 在启动服务前先执行迁移

## 每阶段统一验收标准
- 回归：现有客户端接口 100% 可用
- 性能：关键接口 P95/P99 不下降
- 稳定性：服务单点故障可定位、可回滚
- 运维：具备独立启动/停止/日志观测能力

## 回滚策略
- 网关路由是唯一切换点：
  - 若新服务异常，仅回滚网关路由到单体即可
  - 单体 handler 在迁移期保留，直到稳定后再删除
