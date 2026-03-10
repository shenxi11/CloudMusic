# Web 端音视频平台（React）

更新时间：2026-03-10

## 1. 项目位置

- 前端工程目录：`web-player/`
- 技术栈：`React + TypeScript + Vite`

## 2. 已实现功能

1. 连接服务器页（输入 `http://ip:port`，调用 `/client/ping` 验证）
2. 登录/注册页（`/users/login`、`/users/register`）
3. 主界面（客户端风格布局）
4. 音乐库播放（`/files` + `/get_music`）
5. 视频库播放（`/videos` + `/video/stream`）
6. 推荐列表（`/recommendations/audio`）
7. 推荐行为上报（`/recommendations/feedback`）
8. 喜欢列表（`/user/favorites`、`/user/favorites/add`）
9. 播放历史写入（`/user/history/add`）
10. 播放队列（上一首/下一首）
11. 搜索页（`/music/search`）
12. 歌词面板（优先使用 `lrc_url`）
13. HLS 视频播放支持（`.m3u8` 自动走 `hls.js`）
14. 推荐场景切换（`home/radio/detail`）
15. 播放模式切换（顺序 / 单曲循环 / 随机）
16. 歌词时间轴高亮（LRC 时间标签解析）
17. HLS 模块按需懒加载（仅播放 m3u8 时加载）

## 3. 本地运行

```bash
cd web-player
npm install
npm run dev
```

默认访问：`http://127.0.0.1:5173`

## 4. 生产构建

```bash
cd web-player
npm run build
```

产物目录：`web-player/dist/`

## 5. 对接约定

1. 首次进入先连接服务器（保存到 `localStorage`）
2. 登录成功后，使用 `account` 作为推荐与用户行为接口的 `user_id/user_account`
3. 推荐播放会自动上报 `play`，播放结束上报 `finish`
4. 点击“喜欢”会写入收藏并上报推荐 `like` 事件（若来自推荐流）

## 6. 后续扩展建议

1. 增加歌词滚动高亮（按时间戳）
2. 增加用户历史与喜欢的批量管理
3. 接入更细粒度埋点（曝光/点击/跳过时长）
4. 采用路由级代码分割（页面 chunk 进一步拆分）
5. 支持歌词翻译/音译双轨显示
