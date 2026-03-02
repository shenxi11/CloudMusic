# 完整环境迁移指南（包含数据库）

本指南说明如何将整个音乐服务器环境（包括代码、配置、数据库数据、上传文件）迁移到新服务器。

---

## 📋 迁移前准备

### 源服务器要求
- ✅ MySQL 8.0+ 正在运行
- ✅ 有 `mysqldump` 工具
- ✅ 数据库账号有导出权限

### 目标服务器要求
- ✅ MySQL 8.0+ 已安装并运行
- ✅ Redis 7.0+ 已安装并运行
- ✅ 有 `mysql` 客户端工具
- ✅ 足够的磁盘空间

---

## 🚀 完整迁移流程

### 方法一：一键打包迁移（推荐）

#### 步骤1: 在源服务器上打包

```bash
cd /path/to/GoTest/microservice-deploy

# 执行打包脚本（会自动提示是否备份数据库）
./package.sh

# 脚本会：
# 1. 检查所有必要文件
# 2. 询问是否备份数据库（输入 y）
# 3. 自动备份数据库到 database_backup/
# 4. 编译可执行文件（如需要）
# 5. 打包成 music_server_deploy_YYYYMMDD_HHMMSS.tar.gz
```

**输出示例：**
```
是否备份数据库？(y/n) y
✓ 数据库备份完成
  原始大小: 15M
  压缩后:   3.2M
  备份文件: database_backup/music_users_20260225_143000.sql.gz

✓ 打包成功！
  文件名: music_server_deploy_20260225_143100.tar.gz
  大小:   12M
```

#### 步骤2: 传输到新服务器

```bash
# 使用 scp 传输
scp music_server_deploy_*.tar.gz user@new-server:/opt/

# 或使用 rsync（更快，支持断点续传）
rsync -avz --progress music_server_deploy_*.tar.gz user@new-server:/opt/
```

#### 步骤3: 在新服务器上解压

```bash
ssh user@new-server

cd /opt
tar -xzf music_server_deploy_*.tar.gz
cd microservice-deploy

# 查看内容
ls -la
# 应该看到：
# - music_server（可执行文件）
# - config.yaml（配置文件）
# - database_backup/music_users_*.sql.gz（数据库备份）
# - start.sh, stop.sh 等脚本
```

#### 步骤4: 配置新服务器环境

```bash
# 1. 修改配置文件
vim config.yaml

# 必须修改的配置：
# server:
#   base_url: "http://your-new-server-ip:8080"  # 改为新服务器IP
#
# database:
#   host: "localhost"                            # 或MySQL服务器地址
#   password: "your_mysql_password"              # MySQL密码
#
# redis:
#   addr: "localhost:6379"                       # 或Redis服务器地址
```

#### 步骤5: 恢复数据库

```bash
# 查看可用的备份文件
ls -lh database_backup/

# 恢复数据库
./restore_database.sh database_backup/music_users_20260225_143000.sql.gz

# 脚本会：
# 1. 测试数据库连接
# 2. 备份当前数据库（如果存在）
# 3. 创建数据库（如果不存在）
# 4. 导入数据
# 5. 显示表统计信息
```

**输出示例：**
```
✓ 数据库连接成功
⚠ 警告: 此操作将覆盖数据库 'music_users' 中的所有数据！
是否继续？(yes/no) yes

✓ 数据库恢复成功！

数据库统计信息：
  数据库: music_users
  表数量: 5

表名及记录数：
  user_favorite_music          1250 条记录
  user_play_history           3800 条记录
  music_files                  500 条记录
  users                        150 条记录
  albums                       80 条记录
```

#### 步骤6: 启动服务

```bash
# 启动服务
./start.sh

# 查看日志
tail -f logs/server.log

# 测试服务
curl http://localhost:8080/health
```

---

### 方法二：分步手动迁移

适合需要更细粒度控制的场景。

#### 1. 备份数据库

```bash
# 在源服务器上
cd microservice-deploy
./backup_database.sh

# 会生成文件：database_backup/music_users_YYYYMMDD_HHMMSS.sql.gz
```

#### 2. 备份上传文件

```bash
# 如果有用户上传的音乐文件
tar -czf uploads_backup.tar.gz uploads/

# 查看大小
du -h uploads_backup.tar.gz
```

#### 3. 传输文件到新服务器

```bash
# 传输可执行文件
scp music_server user@new-server:/opt/microservice-deploy/

# 传输配置
scp config.yaml user@new-server:/opt/microservice-deploy/

# 传输脚本
scp *.sh user@new-server:/opt/microservice-deploy/

# 传输数据库备份
scp database_backup/music_users_*.sql.gz user@new-server:/opt/microservice-deploy/database_backup/

# 传输上传文件（如果有）
scp uploads_backup.tar.gz user@new-server:/opt/microservice-deploy/
```

#### 4. 在新服务器上恢复

```bash
ssh user@new-server
cd /opt/microservice-deploy

# 解压上传文件
tar -xzf uploads_backup.tar.gz

# 恢复数据库
./restore_database.sh database_backup/music_users_*.sql.gz

# 修改配置
vim config.yaml

# 启动服务
chmod +x *.sh
./start.sh
```

---

## 🔍 验证迁移结果

### 1. 检查服务状态

```bash
# 查看进程
ps aux | grep music_server

# 查看端口
netstat -tuln | grep 8080

# 查看日志
tail -n 50 logs/server.log
```

### 2. 测试API接口

```bash
# 健康检查
curl http://localhost:8080/health

# 测试数据库连接
curl http://localhost:8080/api/test/db

# 测试用户功能（需要实际的用户账号）
curl -H "X-User-Account: test_user" http://localhost:8080/user/favorites
```

### 3. 验证数据完整性

```bash
# 登录 MySQL 检查
mysql -u root -p music_users

# 查看表数据
SELECT COUNT(*) FROM user_favorite_music;
SELECT COUNT(*) FROM user_play_history;
SELECT COUNT(*) FROM music_files;

# 检查最新记录
SELECT * FROM user_play_history ORDER BY play_time DESC LIMIT 5;
```

### 4. 检查文件权限

```bash
# 确保目录有正确权限
ls -la uploads/
ls -la logs/

# 如果权限不对，修复：
chmod 755 uploads logs
```

---

## 📊 数据库迁移详解

### 备份内容

`backup_database.sh` 会备份：
- ✅ 所有表结构
- ✅ 所有表数据
- ✅ 存储过程和函数
- ✅ 触发器
- ✅ 事件调度器
- ✅ 索引和约束

### 备份选项说明

```bash
mysqldump 
  --single-transaction      # 保证 InnoDB 表数据一致性
  --routines                # 备份存储过程
  --triggers                # 备份触发器
  --events                  # 备份事件
  --hex-blob                # 二进制数据转十六进制
  --default-character-set=utf8mb4  # UTF-8 编码
```

### 恢复注意事项

1. **数据库字符集**：确保使用 `utf8mb4` 以支持完整的 Unicode
2. **权限检查**：MySQL 用户需要有 `CREATE`, `INSERT`, `UPDATE` 权限
3. **安全备份**：恢复前会自动备份当前数据到 `database_backup/before_restore/`
4. **空间检查**：确保有足够磁盘空间（至少是备份文件的 3 倍）

---

## ⚙️ 高级配置

### 1. 大数据库优化

如果数据库很大（> 1GB），可以使用以下优化：

```bash
# 分表备份
./backup_database.sh --per-table

# 并行恢复（需要手动拆分 SQL）
mysql -u root -p music_users < table1.sql &
mysql -u root -p music_users < table2.sql &
wait
```

### 2. 增量备份

```bash
# 首次全量备份
./backup_database.sh

# 后续只备份变更（需要启用 binlog）
mysqlbinlog --start-datetime="2026-02-25 00:00:00" \
            --stop-datetime="2026-02-25 23:59:59" \
            /var/log/mysql/mysql-bin.000001 > incremental.sql
```

### 3. 跨版本迁移

如果 MySQL 版本不同：

```bash
# 源服务器（MySQL 5.7）
mysqldump --compatible=mysql8 ... > backup.sql

# 目标服务器（MySQL 8.0）
mysql -u root -p music_users < backup.sql
```

---

## 🐛 故障排查

### 问题1: 数据库连接失败

```bash
# 检查 MySQL 是否运行
systemctl status mysql

# 测试连接
mysql -h localhost -u root -p

# 检查防火墙
sudo ufw status
```

### 问题2: 恢复数据失败

```bash
# 查看详细错误
./restore_database.sh backup.sql.gz 2>&1 | tee restore.log

# 手动恢复
gunzip -c backup.sql.gz > backup.sql
mysql -u root -p music_users < backup.sql
```

### 问题3: 数据不完整

```bash
# 检查备份文件
gunzip -c backup.sql.gz | head -n 100
gunzip -c backup.sql.gz | tail -n 100

# 验证 SQL 文件完整性
gunzip -t backup.sql.gz
```

### 问题4: 权限错误

```bash
# 授予权限
mysql -u root -p
GRANT ALL PRIVILEGES ON music_users.* TO 'your_user'@'localhost';
FLUSH PRIVILEGES;
```

---

## 📁 完整文件清单

迁移后的 `microservice-deploy/` 目录应包含：

```
microservice-deploy/
├── music_server                    # ✅ 可执行文件
├── config.yaml                     # ✅ 配置文件
├── config.yaml.example             # 📄 配置模板
├── start.sh                        # 🚀 启动脚本
├── stop.sh                         # 🛑 停止脚本
├── backup_database.sh              # 💾 数据库备份
├── restore_database.sh             # 🔄 数据库恢复
├── build.sh                        # 🔧 编译脚本
├── package.sh                      # 📦 打包脚本
├── QUICKSTART.md                   # 📖 快速指南
├── README.md                       # 📖 使用手册
├── DEPLOY.md                       # 📖 部署文档
├── MIGRATION_FULL.md              # 📖 本文档
├── database_backup/                # 💾 数据库备份目录
│   └── music_users_*.sql.gz       # 数据库备份文件
├── uploads/                        # 📁 上传文件目录
└── logs/                           # 📁 日志目录
```

---

## ✅ 迁移检查清单

迁移完成后，请检查：

- [ ] 数据库已恢复，表数据完整
- [ ] Redis 连接正常
- [ ] 配置文件已更新（base_url, 数据库密码等）
- [ ] 服务成功启动（端口 8080 正在监听）
- [ ] API 接口响应正常
- [ ] 日志文件正常写入
- [ ] 上传文件目录权限正确
- [ ] 防火墙规则已配置
- [ ] （可选）配置了 systemd 服务
- [ ] （可选）配置了 Nginx 反向代理

---

## 📞 需要帮助？

- 查看日志：`tail -f logs/server.log`
- 数据库日志：`tail -f /var/log/mysql/error.log`
- Redis 日志：`tail -f /var/log/redis/redis-server.log`

**祝您迁移顺利！** 🎉
