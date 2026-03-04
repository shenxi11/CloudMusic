# 推荐接口对接文档（已落地）

更新时间：2026-03-04  
适用版本：`main` 分支（已实现）

## 1. 总览

本次服务端已落地推荐能力，接口由网关统一暴露，核心包括：

1. 个性化推荐列表
2. 相似歌曲推荐
3. 推荐反馈上报
4. 推荐模型状态查询
5. 触发重训任务（管理侧）

基础地址（示例）：

- `http://127.0.0.1:8080`

响应包络统一格式：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

## 2. 客户端接入时序（推荐）

1. 登录成功后拿到 `user_account`（或你客户端自己的 `user_id`）
2. 拉取推荐列表：`GET /recommendations/audio`
3. 展示后记录 `request_id`
4. 用户播放/喜欢/跳过时，上报 `POST /recommendations/feedback`
5. 歌曲详情页调用 `GET /recommendations/similar/{song_id}`

## 3. 接口明细

## 3.1 获取个性化推荐

- 方法：`GET`
- 路径：`/recommendations/audio`

参数：

- `user_id`（推荐，query）
- `user_account`（兼容，query）
- `X-User-Account`（兼容，header）
- `scene`（可选：`home`/`radio`/`detail`，默认 `home`）
- `limit`（可选，默认 `20`，最大 `100`）
- `exclude_played`（可选，默认 `true`）
- `cursor`（保留参数，当前可传空）

成功响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "request_id": "rec_f0f7d57e31f8e3f2f8a1",
    "user_id": "10001",
    "scene": "home",
    "model_version": "rule_hybrid_v1",
    "items": [
      {
        "song_id": "jay/七里香.mp3",
        "path": "jay/七里香.mp3",
        "title": "七里香",
        "artist": "周杰伦",
        "album": "七里香",
        "duration_sec": 294.2,
        "cover_art_url": "http://127.0.0.1:8080/uploads/jay/%E4%B8%83%E9%87%8C%E9%A6%99.jpg",
        "stream_url": "http://127.0.0.1:8080/uploads/jay/%E4%B8%83%E9%87%8C%E9%A6%99.mp3",
        "lrc_url": "http://127.0.0.1:8080/uploads/jay/%E4%B8%83%E9%87%8C%E9%A6%99.lrc",
        "score": 0.9231,
        "reason": "based_on_play_history",
        "source": "cf"
      }
    ]
  }
}
```

失败场景：

- `400`：缺少 `user_id`

## 3.2 获取相似歌曲推荐

- 方法：`GET`
- 路径：`/recommendations/similar/{song_id}`

说明：

- `song_id` 建议做 URL Path Encode（尤其包含中文或空格时）
- `limit` 可选，默认 `20`

示例：

```bash
curl "http://127.0.0.1:8080/recommendations/similar/jay%2F%E4%B8%83%E9%87%8C%E9%A6%99.mp3?limit=10"
```

失败场景：

- `404`：锚点歌曲不存在

## 3.3 上报推荐反馈

- 方法：`POST`
- 路径：`/recommendations/feedback`

请求体：

- `user_id`（必填，若 body 不传可走 `X-User-Account` 兜底）
- `song_id`（必填）
- `event_type`（必填）：`impression|click|play|finish|like|skip|share|dislike`
- `play_ms`（可选）
- `duration_ms`（可选）
- `scene`（可选，默认 `home`）
- `request_id`（建议传）
- `model_version`（建议传）
- `event_at`（可选，RFC3339；不传使用服务端时间）

请求示例：

```json
{
  "user_id": "10001",
  "song_id": "jay/七里香.mp3",
  "event_type": "play",
  "play_ms": 35000,
  "duration_ms": 294000,
  "scene": "home",
  "request_id": "rec_f0f7d57e31f8e3f2f8a1",
  "model_version": "rule_hybrid_v1"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true
  }
}
```

## 3.4 触发重训任务（管理）

- 方法：`POST`
- 路径：`/admin/recommend/retrain`
- 状态码：`202 Accepted`

请求体：

- `model_name`（可选，默认 `rule_hybrid`）
- `force_full`（可选，默认 `false`）

响应示例：

```json
{
  "code": 0,
  "message": "accepted",
  "data": {
    "task_id": "rec_task_1772600123456789000",
    "status": "queued"
  }
}
```

## 3.5 查询模型状态（管理）

- 方法：`GET`
- 路径：`/admin/recommend/model-status`
- 参数：`model_name`（可选，默认 `rule_hybrid`）

响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "model_name": "rule_hybrid",
    "model_version": "rule_hybrid_20260304_101530",
    "status": "ready",
    "trained_at": "2026-03-04T10:15:30+08:00",
    "metrics": {
      "algo": "rule_hybrid",
      "refreshed": true
    }
  }
}
```

## 4. 当前推荐策略（已实现）

当前是可运行的 `rule_hybrid` 策略（无外部模型依赖）：

1. 用户偏好信号：
   - 播放历史（按歌曲/歌手聚合）
   - 喜欢列表（加权增强）
2. 全局热度信号：
   - 全站播放历史聚合热度
3. 反馈修正信号：
   - `like/finish/play` 提升
   - `skip/dislike` 降权
4. 融合打分：
   - `score = 0.55*cf + 0.30*content + 0.15*hot + adjust`

返回 `reason/source` 字段给客户端用于解释展示。

## 5. 数据表（已落地）

迁移脚本：

- `migrations/sql/20260304_profile_recommendation_core.sql`

核心表：

1. `user_recommendation_feedback`
2. `recommendation_model_status`
3. `recommendation_training_jobs`

## 6. 对客户端改造建议

1. 推荐页首屏调用：`/recommendations/audio`
2. 卡片曝光、点击、播放、跳过、喜欢都上报 `/recommendations/feedback`
3. 详情页“相似推荐”调用 `/recommendations/similar/{song_id}`
4. 保留 `request_id + model_version` 在埋点中回传，便于效果分析与回溯

## 7. 联调检查清单

1. 用户登录后 `user_id` 传递是否稳定
2. `song_id` 是否与服务端 `music_files.path` 一致（路径大小写、编码）
3. URL Path Encode 是否正确处理中文与空格
4. 反馈事件是否覆盖至少 `play/finish/skip/like`
