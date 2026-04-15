# 微服务部署包 - 快速参考

## 📦 部署包内容

```
microservice-deploy/
├── music_server          # 编译后的可执行文件
├── config.yaml           # 配置文件（需根据环境修改）
├── config.yaml.example   # 配置文件示例
├── start.sh              # 启动脚本
├── stop.sh               # 停止脚本
├── build.sh              # 编译脚本（仅开发环境）
├── package.sh            # 打包脚本（仅开发环境）
├── README.md             # 使用说明
├── DEPLOY.md             # 详细部署文档
├── uploads/              # 音乐文件上传目录
└── logs/                 # 日志目录
```

## 🚀 快速开始

### 1. 首次部署

```bash
# 1) 解压部署包
tar -xzf music_server_deploy_*.tar.gz
cd microservice-deploy

# 2) 配置文件
cp config.yaml.example config.yaml
vim config.yaml  # 修改必要配置

# 3) 启动服务
export JAMENDO_CLIENT_ID="your_jamendo_client_id"  # 可选：启用 Jamendo 外部曲库
./start.sh
```

### 2. 必须修改的配置

编辑 `config.yaml`：

```yaml
server:
  base_url: "http://your-server-ip:8080"  # 修改为实际地址

database:
  host: "localhost"                        # MySQL地址
  password: "your_password"                # MySQL密码

redis:
  addr: "localhost:6379"                   # Redis地址

external:
  jamendo:
    enabled: true
    client_id: ""                          # 推荐留空，运行时使用 JAMENDO_CLIENT_ID
```

Jamendo 用于补充外部独立音乐搜索：

```bash
export JAMENDO_CLIENT_ID="your_jamendo_client_id"
curl -X POST http://127.0.0.1:8080/external/music/jamendo/search \
  -H 'Content-Type: application/json' \
  -d '{"keyword":"mayday","limit":2}'
```

Jamendo 结果只外链播放，不下载、不缓存、不写入本地曲库。

### 3. 常用命令

```bash
# 启动服务
./start.sh

# 停止服务
./stop.sh

# 重启服务
./stop.sh && ./start.sh

# Docker 模式只重启已有镜像，跳过重新构建
./start_docker.sh --no-build

# 正式部署目录自动判断是否需要重建镜像
./scripts/deploy_from_main.sh

# 明确要求跳过构建并重建容器
./scripts/deploy_from_main.sh --no-build

# 查看日志
tail -f logs/server.log

# 查看运行状态
ps aux | grep music_server
netstat -tuln | grep 8080
```

## 🔧 环境依赖

### 必需（运行时）
- **MySQL 8.0+** - 数据库
- **Redis 7.0+** - 缓存

### 可选（开发时）
- **Go 1.22+** - 仅需编译时

## 📊 目录说明

| 目录/文件 | 用途 | 是否必需 |
|----------|------|---------|
| music_server | 可执行文件 | ✅ 必需 |
| config.yaml | 配置文件 | ✅ 必需 |
| start.sh | 启动脚本 | ✅ 推荐 |
| stop.sh | 停止脚本 | ✅ 推荐 |
| uploads/ | 文件存储 | ✅ 必需 |
| logs/ | 日志存储 | ✅ 必需 |
| build.sh | 编译脚本 | ⚪ 可选 |
| package.sh | 打包脚本 | ⚪ 可选 |

## 🔍 端口说明

- **8080** - HTTP API 端口（默认）
- **3306** - MySQL 端口
- **6379** - Redis 端口

## ⚠️ 防火墙配置

```bash
# Ubuntu/Debian
sudo ufw allow 8080/tcp

# CentOS/RHEL
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

## 📖 详细文档

- **README.md** - 完整使用说明
- **DEPLOY.md** - 详细部署文档

## 🐛 常见问题

### Q1: 启动失败 "端口已被占用"
```bash
# 查找占用进程
sudo lsof -i :8080
# 或
sudo netstat -tuln | grep 8080
```

### Q2: 数据库连接失败
检查 `config.yaml` 中的数据库配置是否正确：
- host 地址
- port 端口
- user 用户名
- password 密码
- dbname 数据库名

### Q3: Redis 连接失败
检查 Redis 是否启动：
```bash
redis-cli ping  # 应返回 PONG
```

### Q4: 权限错误
```bash
# 确保脚本可执行
chmod +x *.sh

# 确保目录有写权限
chmod 755 uploads logs
```

## 📞 技术支持

- 查看详细日志：`tail -f logs/server.log`
- 检查进程状态：`ps aux | grep music_server`
- 测试 API：`curl http://localhost:8080/health`

---

**更多详细信息请参考 README.md 和 DEPLOY.md**
