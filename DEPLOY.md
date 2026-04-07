# 微服务部署详细说明

## 正式环境约束

推荐固定使用双目录工作流：

- `/home/shen/microservice-deploy`：开发、测试、验证
- `/home/shen/CloudMusic`：正式运行

正式运行目录约束：

1. `CloudMusic` 只跟踪 `origin/main`
2. `CloudMusic` 不直接开发功能
3. 正式部署前必须保证工作区干净
4. 正式部署统一执行：

```bash
cd /home/shen/CloudMusic
./scripts/deploy_from_main.sh
```

该脚本会自动执行：

1. 校验当前分支必须为 `main`
2. 校验工作区必须干净
3. `git fetch origin`
4. `git pull --ff-only origin main`
5. 重建 `cloudmusic` 容器
6. 输出 `/health` 健康检查结果

## 📋 部署前准备

### 1. 系统要求
- 操作系统: Linux (Ubuntu 20.04+ / CentOS 7+)
- CPU: 1核心+
- 内存: 2GB+
- 硬盘: 20GB+（根据音乐文件数量调整）

### 2. 必要软件
- MySQL 8.0+
- Redis 7.0+
- Go 1.22+（仅编译时需要）

---

## 🚀 首次部署流程

### Step 1: 安装依赖

#### Ubuntu/Debian
```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装MySQL
sudo apt install -y mysql-server
sudo systemctl start mysql
sudo systemctl enable mysql

# 安装Redis
sudo apt install -y redis-server
sudo systemctl start redis
sudo systemctl enable redis

# 安装其他工具
sudo apt install -y lsof net-tools curl
```

#### CentOS/RHEL
```bash
# 更新系统
sudo yum update -y

# 安装MySQL
sudo yum install -y mysql-server
sudo systemctl start mysqld
sudo systemctl enable mysqld

# 安装Redis
sudo yum install -y redis
sudo systemctl start redis
sudo systemctl enable redis

# 安装其他工具
sudo yum install -y lsof net-tools curl
```

### Step 2: 配置数据库

```bash
# 登录MySQL
sudo mysql -u root

# 创建数据库
CREATE DATABASE music_users CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 创建用户（可选）
CREATE USER 'music_user'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON music_users.* TO 'music_user'@'localhost';
FLUSH PRIVILEGES;

# 退出
EXIT;

# 导入数据（如果有备份）
mysql -uroot -p music_users < database_backup.sql
```

### Step 3: 上传部署包

```bash
# 方式1：从原服务器传输整个目录
scp -r microservice-deploy/ user@new-server:/home/user/

# 方式2：从打包文件解压
scp microservice-deploy.tar.gz user@new-server:/home/user/
tar -xzf microservice-deploy.tar.gz
cd microservice-deploy/
```

### Step 4: 配置服务

```bash
cd microservice-deploy/

# 修改配置文件
vim config.yaml

# 必须修改的项：
# 1. database.password - 数据库密码
# 2. server.base_url - 服务器域名或IP
```

### Step 5: 编译和启动

```bash
# 方式1：自动编译并启动
./build.sh
./start.sh

# 方式2：如果已有编译好的可执行文件
./start.sh
```

### Step 6: 验证部署

```bash
# 健康检查
curl http://localhost:8080/health

# 查看日志
tail -f server.log

# 查看进程
ps aux | grep music_server
```

---

## 🔄 从旧服务器迁移

### 方案A: 整体迁移（最简单）

#### 在原服务器上
```bash
cd /path/to/project

# 1. 停止服务
cd microservice-deploy/
./stop.sh

# 2. 导出数据库
mysqldump -uroot -pchange_me_please music_users > database_backup.sql

# 3. 打包整个目录
cd ..
tar -czf microservice-deploy-full.tar.gz microservice-deploy/

# 4. 传输
scp microservice-deploy-full.tar.gz user@new-server:/home/user/
```

#### 在新服务器上
```bash
# 1. 解压
tar -xzf microservice-deploy-full.tar.gz
cd microservice-deploy/

# 2. 导入数据库
mysql -uroot -p -e "CREATE DATABASE music_users;"
mysql -uroot -p music_users < database_backup.sql

# 3. 修改配置
vim config.yaml
# 修改数据库密码、域名等

# 4. 启动服务
./start.sh
```

### 方案B: 最小化迁移

只迁移必要的文件，在新服务器上重新编译：

#### 需要迁移的文件
```bash
microservice-deploy/
├── start.sh          # 启动脚本
├── stop.sh           # 停止脚本
├── build.sh          # 编译脚本
├── config.yaml       # 配置文件
├── uploads/          # 上传文件（可选单独打包）
└── README.md         # 文档
```

#### 在原服务器上
```bash
cd microservice-deploy/

# 打包（不包含可执行文件和日志）
tar -czf deploy-minimal.tar.gz \
    --exclude='music_server' \
    --exclude='*.log' \
    --exclude='*.pid' \
    start.sh stop.sh build.sh config.yaml README.md

# 单独打包上传文件（如果很大）
tar -czf uploads.tar.gz uploads/

# 导出数据库
mysqldump -uroot -pchange_me_please music_users > database.sql
```

#### 在新服务器上
```bash
# 解压
tar -xzf deploy-minimal.tar.gz
tar -xzf uploads.tar.gz

# 导入数据库
mysql -uroot -p music_users < database.sql

# 编译
./build.sh

# 启动
./start.sh
```

---

## 📂 目录结构详解

```
microservice-deploy/
├── start.sh              # 启动脚本（自动检查环境、端口、启动服务）
├── stop.sh               # 停止脚本（优雅停止、清理PID文件）
├── build.sh              # 编译脚本（编译并复制可执行文件）
├── config.yaml           # 主配置文件（数据库、Redis、服务器配置）
├── music_server          # 可执行文件（编译后生成，10MB左右）
├── music_server.pid      # 进程ID文件（运行时生成）
├── server.log            # 服务日志文件（运行时生成）
├── uploads/              # 上传文件目录（音乐、封面、歌词）
├── logs/                 # 日志目录（可选，存放历史日志）
├── README.md             # 使用说明
└── DEPLOY.md             # 本文件（部署说明）
```

---

## ⚙️ 配置文件说明

### config.yaml 完整示例

```yaml
# 数据库配置
database:
  host: "localhost"          # 数据库主机
  port: 3306                 # 数据库端口
  user: "root"               # 数据库用户
  password: "change_me_please"  # ⚠️ 数据库密码（必须修改）
  dbname: "music_users"      # 数据库名称
  max_open_conns: 100        # 最大连接数
  max_idle_conns: 10         # 最大空闲连接数

# Redis配置
redis:
  host: "localhost"          # Redis主机
  port: 6379                 # Redis端口
  password: ""               # Redis密码（如果有）
  db: 0                      # Redis数据库编号

# 服务器配置
server:
  port: 8080                                    # 监听端口
  host: "0.0.0.0"                              # 监听地址
  base_url: "http://your-domain.com:8080"      # ⚠️ 服务器URL（必须修改）
  read_timeout: 30                             # 读取超时（秒）
  write_timeout: 30                            # 写入超时（秒）

# 日志配置
logging:
  level: "info"              # 日志级别: debug, info, warn, error
  file: "server.log"         # 日志文件
  max_size: 100              # 日志文件最大大小（MB）
  max_backups: 3             # 保留的旧日志文件数量
  max_age: 28                # 日志文件保留天数

# 存储配置
storage:
  upload_dir: "./uploads"    # 上传文件目录
  max_upload_size: 100       # 最大上传大小（MB）
```

---

## 🔐 安全配置

### 1. 修改数据库密码

```bash
# MySQL
mysql -uroot -p
ALTER USER 'root'@'localhost' IDENTIFIED BY 'new_secure_password';
FLUSH PRIVILEGES;

# 更新config.yaml
vim config.yaml
# 修改 database.password 为新密码
```

### 2. 配置防火墙

```bash
# Ubuntu (UFW)
sudo ufw allow 8080/tcp
sudo ufw enable
sudo ufw status

# CentOS (firewalld)
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
sudo firewall-cmd --list-all
```

### 3. 文件权限

```bash
cd microservice-deploy/

# 脚本可执行权限
chmod +x start.sh stop.sh build.sh

# 配置文件只读（保护密码）
chmod 600 config.yaml

# 可执行文件
chmod 755 music_server

# 上传目录
chmod 755 uploads/
```

### 4. 使用systemd（推荐生产环境）

```bash
# 创建服务文件
sudo vim /etc/systemd/system/music-server.service
```

```ini
[Unit]
Description=Music Server
After=network.target mysql.service redis.service

[Service]
Type=simple
User=your-username
WorkingDirectory=/path/to/microservice-deploy
ExecStart=/path/to/microservice-deploy/music_server
Restart=on-failure
RestartSec=5
StandardOutput=append:/path/to/microservice-deploy/server.log
StandardError=append:/path/to/microservice-deploy/server.log

[Install]
WantedBy=multi-user.target
```

```bash
# 启用服务
sudo systemctl daemon-reload
sudo systemctl enable music-server
sudo systemctl start music-server

# 管理命令
sudo systemctl status music-server   # 查看状态
sudo systemctl restart music-server  # 重启
sudo systemctl stop music-server     # 停止
sudo journalctl -u music-server -f   # 查看日志
```

---

## 📊 监控和维护

### 日常检查

```bash
# 检查服务状态
./status.sh  # 或者
ps aux | grep music_server

# 查看实时日志
tail -f server.log

# 检查资源使用
top -p $(cat music_server.pid)

# 检查磁盘空间
df -h
du -sh uploads/
```

### 日志管理

```bash
# 查看最近日志
tail -100 server.log

# 搜索错误
grep ERROR server.log

# 按日期查看
grep "2026-02-25" server.log

# 日志归档
mv server.log server.log.$(date +%Y%m%d)
gzip server.log.*
```

### 性能监控

```bash
# 端口连接数
netstat -an | grep :8080 | wc -l

# CPU和内存使用
ps aux | grep music_server

# 数据库连接
mysql -uroot -p -e "SHOW PROCESSLIST;"

# Redis状态
redis-cli info stats
```

---

## ❗ 常见问题

### Q1: 启动失败，提示端口被占用
```bash
# 查看占用进程
lsof -i :8080

# 停止占用进程
kill -9 $(lsof -t -i :8080)

# 或修改配置文件端口
vim config.yaml  # 修改 server.port
```

### Q2: 数据库连接失败
```bash
# 检查MySQL状态
sudo systemctl status mysql

# 测试连接
mysql -uroot -p -e "SHOW DATABASES;"

# 检查防火墙
sudo ufw status  # Ubuntu
sudo firewall-cmd --list-all  # CentOS

# 检查配置
cat config.yaml | grep -A5 "database:"
```

### Q3: Redis连接失败
```bash
# 检查Redis状态
sudo systemctl status redis

# 测试连接
redis-cli ping

# 检查配置
redis-cli info server
```

### Q4: 上传文件404
```bash
# 检查uploads目录
ls -la uploads/

# 检查权限
chmod 755 uploads/

# 检查配置
grep "upload_dir" config.yaml
```

### Q5: 编译失败
```bash
# 检查Go环境
go version

# 检查项目结构
ls -la ../cmd/monolith/

# 清理并重新编译
go clean
go mod tidy
./build.sh
```

---

## 🔧 升级和更新

### 升级应用

```bash
# 1. 备份当前版本
cp music_server music_server.backup
cp config.yaml config.yaml.backup

# 2. 停止服务
./stop.sh

# 3. 更新代码（从git或新的源码包）
cd ..
git pull  # 或解压新版本

# 4. 重新编译
cd microservice-deploy/
./build.sh

# 5. 启动新版本
./start.sh

# 6. 验证
curl http://localhost:8080/health
tail -f server.log
```

### 回滚版本

```bash
# 停止服务
./stop.sh

# 恢复备份
cp music_server.backup music_server
cp config.yaml.backup config.yaml

# 启动
./start.sh
```

---

## 📞 技术支持

- 📖 使用文档: `README.md`
- 🚀 迁移指南: `../MIGRATION_QUICKSTART.md`
- 📝 完整部署指南: `../docs/deployment_guide.md`
- 🔧 API文档: `../docs/`

---

**部署愉快！** 🎉
