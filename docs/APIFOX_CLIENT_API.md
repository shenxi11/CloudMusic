# Apifox 客户端接口统一管理

本文件说明如何把 CloudMusic 服务端的客户端接口统一导入 Apifox，方便桌面客户端开发、联调和测试。

## 文件来源

- 规范源文件：`docs/openapi.yaml`
- Apifox 导入文件：`docs/apifox-client-openapi.yaml`
- 默认测试环境：`http://192.168.1.208:8080`

`docs/openapi.yaml` 是仓库内的接口事实来源；`docs/apifox-client-openapi.yaml` 是面向客户端开发裁剪后的 Apifox 导入文件，只包含客户端会用到的接口。

## Apifox 导入步骤

1. 打开 Apifox，新建项目或进入 CloudMusic 客户端接口项目。
2. 选择“导入数据”或“导入 OpenAPI/Swagger”。
3. 选择 `docs/apifox-client-openapi.yaml`。
4. 导入后创建或确认环境变量：`baseUrl=http://192.168.1.208:8080`。
5. 先调用 `GET /client/ping` 和 `GET /client/bootstrap` 验证服务连通性。
6. 调用 `POST /users/login` 后，从响应里复制 `account` 和 `online_session_token`，填入需要登录态的接口参数。

## 接口范围

当前 Apifox 导入文件覆盖桌面客户端接口：服务探活、账号资料、在线状态、曲库搜索、收藏历史、歌单、评论、热歌榜、推荐、媒体上传下载、本地播放辅助和视频接口。

不包含后台管理、调试、根页面和历史测试接口，例如 `/admin/*`、`/records`、`/add`、`/stats`、`/ack`、`/`。

## 维护规则

1. 服务端新增或修改客户端接口时，先更新 `docs/openapi.yaml`。
2. 再重新生成或同步更新 `docs/apifox-client-openapi.yaml`。
3. 提交前运行：`./scripts/check_openapi_client_scope.sh`。
4. 在 Apifox 中重新导入 `docs/apifox-client-openapi.yaml`。

## 常用测试顺序

1. `GET /client/ping`
2. `GET /client/bootstrap`
3. `POST /users/login`
4. `GET /files`
5. `GET /music/charts/hot`
6. `GET /recommendations/audio`
7. `GET /music/local/playback-info?music_path=<relative_path>`
