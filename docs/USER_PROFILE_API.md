# 用户资料接口文档

更新时间：2026-04-06

## 1. 背景

当前服务端已经具备登录、注册、在线会话与基础用户体系。为支持客户端“个人信息”页面，服务端新增以下资料能力：

1. 查询当前用户资料
2. 修改用户名
3. 上传/替换头像

本轮只支持用户修改自己的资料，不支持管理员代改。

## 2. 认证方式

三个接口统一使用当前在线会话校验：

1. `account`：当前用户账号
2. `session_token`：登录成功后返回的在线会话 token

说明：

1. 资料查询使用 query 参数传递
2. 修改用户名使用 JSON body 传递
3. 上传头像使用 `multipart/form-data` 传递
4. `session_token` 失效时，服务端返回 `401`

## 3. 字段说明

### 3.1 用户资料

- `account`：用户账号，主身份标识
- `username`：当前用户名
- `avatar_url`：头像完整访问地址；为空表示用户尚未上传头像
- `created_at`：账号创建时间
- `updated_at`：最近一次资料更新时间

### 3.2 头像规则

- 上传字段名：`avatar`
- 支持格式：`jpg/jpeg/png/webp`
- 大小限制：`5MB`
- 存储位置：`uploads/avatars/{account}/avatar.xxx`

## 4. 客户端接入流程

推荐客户端流程：

1. 用户登录：`POST /users/login`
2. 读取返回中的 `online_session_token`
3. 进入“个人信息”页时调用 `GET /users/profile`
4. 用户修改昵称时调用 `POST /users/profile/username`
5. 用户选择新头像后调用 `POST /users/profile/avatar`
6. 修改成功后刷新资料页，或直接使用返回的新 `username/avatar_url` 更新 UI

## 5. 接口定义

### 5.1 查询当前资料

- 方法：`GET`
- 路径：`/users/profile`

请求示例：

```bash
curl "http://127.0.0.1:8080/users/profile?account=root&session_token=demo_token"
```

响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "account": "root",
    "username": "shenxi11",
    "avatar_url": "http://127.0.0.1:8080/uploads/avatars/root/avatar.jpg",
    "created_at": "2026-04-06T13:20:00+08:00",
    "updated_at": "2026-04-06T13:40:20+08:00"
  }
}
```

### 5.2 修改用户名

- 方法：`POST`
- 路径：`/users/profile/username`

请求示例：

```json
{
  "account": "root",
  "session_token": "demo_token",
  "username": "新的用户名"
}
```

响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "message": "更新成功",
    "username": "新的用户名"
  }
}
```

说明：

1. 服务端会校验用户名非空且长度不超过 100
2. 服务端会校验用户名全局唯一
3. 修改成功后，旧兼容表 `user_path.username` 会同步更新，不影响旧收藏链路

### 5.3 上传/替换头像

- 方法：`POST`
- 路径：`/users/profile/avatar`
- Content-Type：`multipart/form-data`

请求示例：

```bash
curl -X POST "http://127.0.0.1:8080/users/profile/avatar" \
  -F "account=root" \
  -F "session_token=demo_token" \
  -F "avatar=@/home/user/avatar.png"
```

响应示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "message": "上传成功",
    "avatar_url": "http://127.0.0.1:8080/uploads/avatars/root/avatar.png",
    "avatar_path": "avatars/root/avatar.png"
  }
}
```

说明：

1. 二次上传会覆盖为最新地址
2. 若旧头像属于服务端受管目录，服务端会自动清理旧文件
3. 上传成功后，建议客户端直接使用返回的 `avatar_url` 刷新 UI

## 6. 常见错误与客户端处理建议

### 400

典型原因：

1. `account` 为空
2. `session_token` 为空
3. `username` 为空
4. 头像文件为空
5. 图片格式不支持
6. 图片超过 `5MB`

建议：

1. 直接提示用户输入或文件不合法
2. 保留当前界面编辑状态，避免丢失输入

### 401

典型原因：

1. 在线会话过期
2. `session_token` 与 `account` 不匹配

建议：

1. 提示“登录已过期，请重新登录”
2. 重新执行登录并拿到新的 `online_session_token`

### 404

典型原因：

1. 用户不存在

建议：

1. 客户端清理当前登录态
2. 跳转登录页

### 409

典型原因：

1. 修改后的用户名已被占用

建议：

1. 提示“用户名已被使用，请换一个”
2. 保留输入框内容，方便用户继续修改

### 500

典型原因：

1. 数据库更新失败
2. 头像文件写入失败

建议：

1. 提供“重试”按钮
2. 不要本地提前覆盖旧头像或旧用户名

## 7. 与现有系统的关系

1. 不影响 `/users/login`
2. 不影响 `/users/register`
3. 不影响 `/users/online/*`
4. 不影响旧兼容收藏链路 `/users/add_music`
5. 登录响应新增 `avatar_url` 字段，客户端可直接读取
