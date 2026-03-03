# 音乐平台微服务部署包

## 📁 目录结构

```
microservice-deploy/
├── start.sh          # 启动脚本
├── stop.sh           # 停止脚本
├── build.sh          # 编译脚本
├── config.yaml       # 配置文件
├── music_server      # 可执行文件（编译后生成）
├── uploads/          # 上传文件目录
├── logs/             # 日志目录
├── README.md         # 本文件
└── DEPLOY.md         # 部署说明
```

## 🚀 快速开始

### 1. 一键 Docker 部署（主流程，推荐）

```bash
# 1) 拉代码
git clone <your-repo-url>
cd microservice-deploy

# 2) 首次启动（会自动生成 .env.docker 和 configs/config.docker.generated.yaml）
./start_docker.sh

# 3) 按需修改 .env.docker 后再次启动
# 必改项（至少检查）：
#   HOST_UPLOAD_DIR=/你的音频目录
#   HOST_VIDEO_DIR=/你的视频目录
#   MYSQL_ROOT_PASSWORD=你的数据库密码
#   SERVER_PUBLIC_HOST=你的服务器IP或域名
#   SERVER_PUBLIC_BASE_URL=http://你的服务器IP或域名:8080
#   ADMIN_USERNAME=后台管理员用户名
#   ADMIN_PASSWORD=后台管理员密码
./start_docker.sh

# 4) 查看状态和日志
./scripts/docker.sh ps
./scripts/docker.sh logs gateway

# 5) 健康检查
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/client/ping
curl http://127.0.0.1:8080/client/bootstrap
curl http://127.0.0.1:8080/files
curl http://127.0.0.1:8080/videos

# 6) 停止
./stop_docker.sh
```

管理后台入口（默认）：
- 地址：`http://127.0.0.1:8080/admin`
- 默认管理员：`admin / admin123456`（建议首次启动后在 `.env.docker` 或 `configs/config.yaml` 修改）

说明：
- Docker 模式会自动拉起 `MySQL + Redis + Gateway + 各微服务`。
- 静态资源目录通过 `.env.docker` 的 `HOST_UPLOAD_DIR/HOST_VIDEO_DIR/HOST_HLS_DIR` 配置，不需要改代码。
- 如果数据库是空库，只会自动建表，不会自动生成业务数据；需自行导入数据或执行你的媒体入库脚本。
- 同机部署多套环境时，修改 `.env.docker` 的 `COMPOSE_PROJECT_NAME`，避免复用旧容器和 volume。

### 2. 静态资源目录入库（音频/视频元数据写库）

当你替换了本地 `uploads` / `video` 目录后，可执行一次“扫描目录并写入数据库”。

本地模式（非 Docker）：
```bash
./scripts/sync_media_db.sh \
  -config configs/config.yaml \
  -audio-dir /your/uploads/path \
  -video-dir /your/video/path
```

Docker 模式（推荐）：
```bash
# 使用 compose 中挂载的 /data/uploads 和 /data/video
./scripts/docker.sh sync-media

# 或传自定义参数（例如 dry-run）
./scripts/docker.sh sync-media -dry-run -config /app/configs/config.yaml -audio-dir /data/uploads -video-dir /data/video
```

说明：
- 脚本会幂等 upsert 到 `music_users.music_files`（音频 `is_audio=1`，视频 `is_audio=0`）。
- 会自动 upsert `music_users.artists`（基于音频 artist 字段）和 `music_media.media_lyrics_map`。
- Docker 镜像已内置 `ffmpeg/ffprobe`，默认可读取时长与标签；本地非 Docker 模式无 `ffprobe` 时会回退为文件名信息，`duration_sec=0`。
### 2.5 管理后台功能（新增）

- 管理员登录：`/admin` 登录后进入后台页面
- 音频/视频资源：按关键词筛选，支持批量删库、批量删库+删除物理文件
- 上传歌曲：支持上传歌曲+歌词+封面；未上传封面时自动用 `ffmpeg` 提取
- 用户在线状态：查看全部客户端用户，并展示在线/离线与最后在线时间

### 2.6 后台示例图片

管理员后台页面示例：

![后台示例图1](示例图片/003b59560dd8c669d497e0bdc1b6d4a2.png)

![后台示例图2](示例图片/3505978380956e0086b958c179819029.png)

### 3. Docker 常用命令

```bash
# 启动/停止/重启
./scripts/docker.sh up
./scripts/docker.sh down
./scripts/docker.sh restart

# 查看状态与日志
./scripts/docker.sh ps
./scripts/docker.sh logs
./scripts/docker.sh logs gateway

# 单独执行迁移
./scripts/docker.sh migrate
```

### 4. 本地非 Docker 模式（可选）

```bash
# 首次编译并迁移
./build.sh
./migrate.sh

# 启动 / 停止
./start.sh
./stop.sh
```

### 5. 静态资源分离模式（Nginx + Go）

```bash
# 启动：Nginx(8080) + Go后端(127.0.0.1:18080)
./start_split.sh

# 停止
./stop_split.sh
```

详细说明见 `STATIC_SPLIT_DEPLOY.md`。微服务演进路线见 `MICROSERVICE_ROADMAP.md`。

### 6. API 文档（OpenAPI）

```bash
# 网关 API OpenAPI 3.0 文档
docs/openapi.yaml

# 客户端启动连接接口文档
docs/CLIENT_CONNECT_API.md

# 用户在线状态接口文档（会话+心跳+状态+下线）
docs/ONLINE_PRESENCE_API.md

# 推荐系统接口与算法草案（新增）
docs/RECOMMENDATION_API.md
docs/recommendation-openapi.yaml
docs/RECOMMENDATION_SCHEMA.sql

# 启动 Swagger UI 预览（默认 http://127.0.0.1:18090/swagger-ui.html）
./scripts/openapi_preview.sh start

# 查看状态 / 停止
./scripts/openapi_preview.sh status
./scripts/openapi_preview.sh stop
```

可直接导入 Swagger UI / Redoc。当前文档覆盖网关可访问接口（含兼容接口）。最近一次实测报告见 `API_DOC_TEST_REPORT.md`。

### 7. 重新编译（本地模式）

```bash
# 停止服务
./stop.sh

# 重新编译
./build.sh

# 启动服务
./start.sh
```

## ⚙️ 配置说明

### config.yaml 关键配置

```yaml
# 数据库配置
database:
  host: "localhost"
  port: 3306
  user: "root"
  password: "change_me_please"  # 修改为实际密码
  name: "music_users"

# 服务器配置
server:
  port: 8080
  base_url: "http://your-domain.com:8080"  # 修改为实际域名

# Redis配置
redis:
  host: "localhost"
  port: 6379

# 事件可靠性配置（Phase 5.2）
event:
  bus:
    stream: "music.domain.events.v1"
    group: "music-domain-events-group"
    consumer: ""
    pending_min_idle_ms: 30000
    pending_batch_size: 50
  outbox:
    poll_interval_ms: 2000
    batch_size: 50
    max_retry: 10
    retry_base_delay_ms: 1000
  worker:
    max_retry: 3
    retry_delay_ms: 300

# 领域数据 schema（Phase 6.1）
schemas:
  profile: "music_profile"
  catalog: "music_users"
  media: "music_media"

# 管理后台账号
admin:
  username: "admin"
  password: "请改成强密码"
  session_ttl_minutes: 1440
```

### 数据迁移命令

```bash
# 迁移全部服务（幂等）
./migrate.sh

# 指定服务迁移
./migrator -config configs/config.yaml -service profile
./migrator -config configs/config.yaml -service media
./migrator -config configs/config.yaml -service event
```

## 📦 迁移到新服务器

### 方案一：整体迁移（推荐）

```bash
# 在原服务器上
cd /path/to/project
tar -czf microservice-deploy.tar.gz microservice-deploy/

# 传输到新服务器
scp microservice-deploy.tar.gz user@new-server:/path/

# 在新服务器上
tar -xzf microservice-deploy.tar.gz
cd microservice-deploy/
./start.sh
```

### 方案二：仅迁移必要文件

需要迁移的文件：
- ✅ `start.sh`, `stop.sh`, `build.sh` - 脚本
- ✅ `config.yaml` - 配置文件
- ✅ `uploads/` - 上传文件目录
- ⚠️ `music_server` - 可执行文件（建议重新编译）

不需要迁移：
- ❌ `server.log` - 日志文件
- ❌ `music_server.pid` - PID文件

## 🔧 依赖环境

### 必须安装
- Go 1.22+（仅编译时需要）
- MySQL 8.0+
- Redis 7.0+

### 安装命令

#### Ubuntu/Debian
```bash
sudo apt update
sudo apt install -y mysql-server redis-server

# Go安装（编译时）
wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

#### CentOS/RHEL
```bash
sudo yum install -y mysql-server redis
sudo systemctl start mysql redis
sudo systemctl enable mysql redis
```

## 📊 监控和日志

### 查看日志
```bash
# 实时日志
tail -f server.log

# 查看最近100行
tail -100 server.log

# 搜索错误
grep ERROR server.log

# 按时间筛选
grep "2026-02-25" server.log
```

### 监控进程
```bash
# 查看进程状态
ps aux | grep music_server

# 查看端口占用
lsof -i :8080

# 查看资源使用
top -p $(cat music_server.pid)
```

## 🔒 安全建议

1. **修改默认密码**
   ```yaml
   # config.yaml
   database:
     password: "your-secure-password"  # 不要使用默认密码
   ```

2. **配置防火墙**
   ```bash
   # Ubuntu
   sudo ufw allow 8080/tcp
   
   # CentOS
   sudo firewall-cmd --permanent --add-port=8080/tcp
   sudo firewall-cmd --reload
   ```

3. **定期备份**
   ```bash
   # 备份数据库
   mysqldump -uroot -p music_users > backup_$(date +%Y%m%d).sql
   
   # 备份上传文件
   tar -czf uploads_backup_$(date +%Y%m%d).tar.gz uploads/
   ```

4. **日志轮转**
   ```bash
   # 定期清理日志
   # 添加到crontab
   0 0 * * * cd /path/to/microservice-deploy && mv server.log server.log.$(date +%Y%m%d) && gzip server.log.*
   ```

## ❗ 故障排查

### 问题1：启动失败
```bash
# 查看日志
tail -50 server.log

# 检查端口
lsof -i :8080

# 检查配置
cat config.yaml
```

### 问题2：数据库连接失败
```bash
# 检查MySQL状态
sudo systemctl status mysql

# 测试连接
mysql -uroot -p -e "SHOW DATABASES;"

# 检查配置
grep -A5 "database:" config.yaml
```

### 问题3：Redis连接失败
```bash
# 检查Redis状态
sudo systemctl status redis

# 测试连接
redis-cli ping
```

### 问题4：进程僵死
```bash
# 查找进程
ps aux | grep music_server

# 强制停止
kill -9 $(cat music_server.pid)

# 清理并重启
./stop.sh
./start.sh
```

## 🎯 性能优化

1. **使用systemd管理**（推荐）
   ```bash
   # 创建服务文件
   sudo cp systemd/music-server.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable music-server
   sudo systemctl start music-server
   ```

2. **配置Nginx反向代理**
   ```nginx
   # /etc/nginx/sites-available/music-server
   server {
       listen 80;
       server_name your-domain.com;
       
       location / {
           proxy_pass http://localhost:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
       }
   }
   ```

3. **启用HTTPS**
   ```bash
   # 使用Let's Encrypt
   sudo certbot --nginx -d your-domain.com
   ```

## 📞 技术支持

- 📖 完整部署指南: `../docs/deployment_guide.md`
- 🚀 快速迁移指南: `../MIGRATION_QUICKSTART.md`
- 📝 API文档: `../docs/`

## 📝 版本信息

- **版本**: v3.0
- **更新日期**: 2026-02-25
- **架构**: 领域驱动设计 (DDD)
- **Go版本**: 1.22+

---

**祝您使用愉快！** 🎉
