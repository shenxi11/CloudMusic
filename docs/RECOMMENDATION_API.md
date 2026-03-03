# 音频推荐接口与算法落地草案

更新时间：2026-03-03

## 1. 目标

为客户端提供“按用户听歌偏好推荐音频”的能力，并保持对现有网关接口兼容，不影响登录、注册、播放等现有流程。

## 2. 接口草案

完整 OpenAPI 草案见：

- `docs/recommendation-openapi.yaml`

核心接口：

1. `GET /recommendations/audio`：按用户个性化推荐歌曲列表
2. `GET /recommendations/similar/{song_id}`：基于当前歌曲找相似歌曲
3. `POST /recommendations/feedback`：上报曝光/点击/播放/喜欢/跳过等反馈
4. `POST /admin/recommend/retrain`：管理员触发模型重训
5. `GET /admin/recommend/model-status`：查看当前模型版本与状态

## 3. 数据结构草案

可执行 SQL 草案见：

- `docs/RECOMMENDATION_SCHEMA.sql`

核心表：

1. `music_recommend.user_events`：行为事件日志（play/like/skip 等）
2. `music_recommend.song_features`：歌曲特征与向量
3. `music_recommend.user_profile_features`：用户画像与偏好向量
4. `music_recommend.similar_song_index`：歌曲相似索引
5. `music_recommend.recommendation_cache`：推荐结果缓存
6. `music_recommend.model_versions`：模型版本与指标
7. `music_recommend.training_jobs`：训练任务状态

## 4. 推荐算法建议（开源优先）

### 4.1 第一阶段（快速上线）

1. 协同过滤召回：`implicit`（ALS/BPR）
2. 内容召回：歌曲特征相似度（余弦相似度）
3. 热门兜底：近 7 天高完播、高收藏歌曲
4. 混合打分：`score = 0.55*cf + 0.30*content + 0.15*hot`

### 4.2 第二阶段（效果提升）

1. 引入 `LightFM`：将协同信息和内容特征一起建模
2. 使用 `faiss` / `hnswlib` 做向量近邻召回
3. 训练轻量排序模型（XGBoost/LightGBM）做精排

### 4.3 可选算法框架

1. `RecBole`：快速验证多种推荐模型
2. `TensorFlow Recommenders`：深度推荐路线
3. 音频特征提取：`librosa`、`Essentia`

## 5. 客户端接入时序

1. 客户端请求 `GET /recommendations/audio`
2. 客户端展示推荐列表并携带 `request_id`
3. 客户端在播放/喜欢/跳过时调用 `POST /recommendations/feedback`
4. 服务端离线任务定时训练（建议每日一次）
5. 模型切换后通过 `model_version` 灰度验证

## 6. 关键工程策略

1. 冷启动：新用户优先热门+内容相似，不依赖历史行为
2. 去重：同歌手过多时做配额限制，提升多样性
3. 新鲜度：增加新歌探索权重，避免推荐长期固化
4. 可解释性：每首推荐返回 `reason/source` 便于客户端展示
5. 可回滚：模型版本化，异常时一键切回旧模型

## 7. 上线验收指标

1. `CTR@20`：推荐位点击率
2. `PlayRate@20`：推荐歌曲播放率
3. `FinishRate`：完播率
4. `LikeRate`：收藏/喜欢率
5. `Diversity`：歌手/曲风多样性指标

## 8. 实施顺序建议

1. 先落地 `feedback` 事件采集
2. 上线 ALS + 热门兜底版本（MVP）
3. 增加内容特征召回与向量检索
4. 增加精排模型与 A/B 实验
