# 客户端用户资料改造对接摘要

更新时间：2026-04-06

## 1. 本次服务端新增内容

本次服务端围绕“个人信息页”新增了以下能力：

1. 查询当前用户资料
2. 修改用户名
3. 上传/替换头像
4. 登录响应新增 `avatar_url`

完整接口文档见：[USER_PROFILE_API.md](./USER_PROFILE_API.md)

## 2. 客户端需要改什么

### 2.1 登录态缓存新增字段

客户端登录成功后，除了原有字段，还需要缓存：

- `account`
- `username`
- `online_session_token`
- `avatar_url`

说明：

1. `account` 仍然是用户主身份标识
2. `online_session_token` 用于资料查询和资料修改
3. `avatar_url` 可直接用于头像显示

### 2.2 个人信息页的数据来源

个人信息页建议不要只依赖本地缓存，而是进入页面时主动拉一次：

- `GET /users/profile`

这样可以保证：

1. 用户名修改后页面及时刷新
2. 头像更新后页面及时刷新
3. 多端登录场景下资料显示一致

### 2.3 用户名修改交互

客户端点击“修改用户名”后：

1. 先做本地非空校验
2. 调用 `POST /users/profile/username`
3. 成功后：
   - 更新本地缓存中的 `username`
   - 刷新个人信息页显示
   - 如果客户端顶部、侧栏、评论区等位置展示用户名，也要同步刷新

### 2.4 头像上传交互

客户端点击“更换头像”后：

1. 打开本地文件选择器
2. 限制图片类型：`jpg/jpeg/png/webp`
3. 调用 `POST /users/profile/avatar`，使用 `multipart/form-data`
4. 成功后：
   - 用返回的 `avatar_url` 直接更新头像显示
   - 同步更新本地缓存中的 `avatar_url`

## 3. 推荐对接流程

### 3.1 登录后初始化

1. 调用 `POST /users/login`
2. 保存返回中的：
   - `account`
   - `username`
   - `avatar_url`
   - `online_session_token`
3. 首页或个人中心直接使用本地缓存渲染基础信息
4. 进入个人信息页时再调用 `GET /users/profile` 做一次服务端同步

### 3.2 打开个人信息页

请求：

```http
GET /users/profile?account={account}&session_token={online_session_token}
```

拿到的数据用于：

- 用户名输入框默认值
- 当前头像显示
- 账号信息展示
- 资料更新时间展示（如果界面需要）

### 3.3 修改用户名

请求：

```json
{
  "account": "root",
  "session_token": "online_session_token",
  "username": "新的用户名"
}
```

成功后建议：

1. 直接使用返回的 `username`
2. 更新全局用户状态
3. 必要时重新请求 `GET /users/profile`

### 3.4 上传头像

请求类型：`multipart/form-data`

字段：

- `account`
- `session_token`
- `avatar`

成功后建议：

1. 直接使用返回的 `avatar_url`
2. 给头像控件追加时间戳或重新绑定地址，避免图片缓存导致界面不刷新
3. 更新本地缓存

## 4. 与旧版本相比的变化

### 4.1 登录响应新增字段

旧客户端登录响应中只有：

- `success`
- `success_bool`
- `username`
- `song_path_list`
- `online_session_token`
- `online_heartbeat_interval_sec`
- `online_ttl_sec`

现在新增：

- `avatar_url`

说明：

1. 老客户端不读取该字段也不会出错
2. 新客户端应优先使用该字段展示头像

### 4.2 新增三个资料接口

- `GET /users/profile`
- `POST /users/profile/username`
- `POST /users/profile/avatar`

这三个接口都依赖：

- `account`
- `session_token`

所以客户端必须在登录成功后正确保存 `online_session_token`。

## 5. 错误处理建议

### 400

含义：参数问题或文件不合法。

客户端建议：

1. 用户名为空时直接本地拦截
2. 头像格式不支持时直接弹提示
3. 图片过大时提示重新选择

### 401

含义：登录会话失效。

客户端建议：

1. 提示“登录已过期，请重新登录”
2. 清理本地会话数据
3. 跳转登录页

### 409

含义：用户名重复。

客户端建议：

1. 提示“用户名已被使用”
2. 保留输入框内容，不要自动清空

### 500

含义：服务端执行失败。

客户端建议：

1. 提示“修改失败，请稍后重试”
2. 不要提前覆盖本地缓存中的旧用户名或旧头像

## 6. 客户端验收清单

客户端联调时至少验证以下场景：

1. 登录后能够正确显示头像和用户名
2. 未上传头像的账号能正常显示默认头像
3. 进入个人信息页时能拉到服务端最新资料
4. 修改用户名成功后，页面显示即时更新
5. 用户名重复时能正确提示
6. 上传头像成功后，个人页和首页头像都同步更新
7. 会话过期时，资料接口返回 `401` 后客户端能跳回登录页

## 7. 备注

1. 本轮服务端只支持“用户修改自己的资料”
2. 本轮不提供“删除头像”接口
3. 本轮不支持通过 URL 或 Base64 上传头像，只支持本地图片文件上传
