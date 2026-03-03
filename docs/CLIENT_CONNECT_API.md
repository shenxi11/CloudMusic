# 客户端启动连接接口说明

更新时间：2026-03-03

## 目标

客户端在显示登录/注册页面前，先让用户输入服务器 `IP:端口`，然后通过本文件定义的接口验证服务可达和可用。

推荐流程：

1. 调用 `GET /client/ping` 验证网络可达与网关路由正确。
2. 调用 `GET /client/bootstrap` 验证后端依赖是否就绪（`ready=true`）。
3. 仅当步骤 1、2 成功后，进入登录/注册界面，使用 `/users/login`、`/users/register`。

## 1) 连通性检查

- 方法：`GET`
- 路径：`/client/ping`
- 认证：无
- 含义：确认“这个 IP/端口”确实是 CloudMusic 服务

示例请求：

```bash
curl -s http://127.0.0.1:8080/client/ping
```

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "service": "cloudmusic-server",
    "status": "ok",
    "api_version": "2026-03-03",
    "timestamp": 1772501000,
    "server_time": "2026-03-03T09:23:20+08:00"
  }
}
```

## 2) 启动引导与可用性检查

- 方法：`GET`
- 路径：`/client/bootstrap`
- 认证：无
- 含义：返回服务状态、基础入口地址、登录注册接口地址

示例请求：

```bash
curl -s http://127.0.0.1:8080/client/bootstrap
```

示例响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "service": "cloudmusic-server",
    "api_version": "2026-03-03",
    "ready": true,
    "timestamp": 1772501000,
    "server_time": "2026-03-03T09:23:20+08:00",
    "public_base_url": "http://127.0.0.1:8080",
    "checks": {
      "database": true,
      "redis": true
    },
    "endpoints": {
      "ping": "http://127.0.0.1:8080/client/ping",
      "bootstrap": "http://127.0.0.1:8080/client/bootstrap",
      "register": "http://127.0.0.1:8080/users/register",
      "login": "http://127.0.0.1:8080/users/login"
    }
  }
}
```

字段说明：

- `ready`：是否可进入登录/注册流程。建议 `true` 才允许继续。
- `checks.database`：数据库可用性。
- `checks.redis`：Redis 可用性。
- `public_base_url`：服务建议的外部访问基础 URL。

## 3) 客户端判定建议

- `ping` 请求失败：提示“IP 或端口不可达/网关错误”。
- `ping` 成功但 `bootstrap.ready=false`：提示“服务器启动中或依赖未就绪，请稍后重试”。
- `bootstrap.ready=true`：进入登录/注册页面。

## 4) 兼容性说明

- 两个接口均无需登录，不会改写业务数据。
- 对应网关（Docker 与 split Nginx）已增加路由转发规则。
- OpenAPI 已同步：`docs/openapi.yaml`。
