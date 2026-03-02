# 微服务部署包 - 文件说明

## 📦 完整文件列表

### 🔧 可执行脚本
| 文件名 | 大小 | 功能说明 | 使用场景 |
|--------|------|----------|----------|
| `start.sh` | 5.2K | 启动服务 | 启动音乐服务器，包含健康检查 |
| `stop.sh` | 4.2K | 停止服务 | 优雅关闭服务器进程 |
| `build.sh` | 2.2K | 编译程序 | 从源码编译可执行文件（仅开发环境） |
| `backup_database.sh` | 5.2K | 备份数据库 | 导出 MySQL 数据库到 .sql.gz 文件 |
| `restore_database.sh` | 6.9K | 恢复数据库 | 从备份文件恢复 MySQL 数据库 |
| `package.sh` | 6.7K | 打包部署 | 创建完整的部署包（含数据库备份） |

### 📄 配置文件
| 文件名 | 大小 | 说明 |
|--------|------|------|
| `config.yaml` | 384B | **当前配置文件**（包含敏感信息，需根据环境修改） |
| `config.yaml.example` | 2.3K | **配置模板**（带详细注释，新环境参考使用） |

### 📖 文档文件
| 文件名 | 大小 | 内容 | 适用人群 |
|--------|------|------|----------|
| `QUICKSTART.md` | 3.3K | 快速参考指南 | 熟悉的用户，需要快速查命令 |
| `README.md` | 5.3K | 完整使用手册 | 日常运维人员 |
| `DEPLOY.md` | 11K | 详细部署文档 | 首次部署、系统管理员 |
| `MIGRATION_FULL.md` | 9.8K | **完整迁移指南** | 需要迁移整个环境（含数据库） |
| `FILE_MANIFEST.md` | - | 本文件，文件说明 | 了解部署包结构 |

### 📁 目录结构
```
microservice-deploy/
├── music_server              # 编译后的可执行文件（运行时需要）
├── database_backup/          # 数据库备份目录
│   ├── music_users_*.sql.gz # 数据库备份文件
│   └── before_restore/      # 恢复前的安全备份
├── uploads/                  # 音乐文件上传目录
├── logs/                     # 日志文件目录
└── *.sh, *.md, *.yaml       # 脚本、文档、配置文件
```

---

## 🎯 使用场景矩阵

### 场景1: 首次部署到新服务器
**需要的文件：**
- ✅ `music_server` - 可执行文件
- ✅ `config.yaml.example` → 复制为 `config.yaml` 并修改
- ✅ `start.sh`, `stop.sh` - 管理服务
- ✅ `database_backup/*.sql.gz` - 数据库备份（如需恢复数据）
- ✅ `restore_database.sh` - 恢复数据库
- 📖 `DEPLOY.md` - 部署指南

**步骤：**
```bash
1. 解压部署包
2. 阅读 DEPLOY.md
3. 修改 config.yaml
4. 恢复数据库（如有备份）
5. ./start.sh
```

### 场景2: 日常运维
**需要的文件：**
- ✅ `start.sh`, `stop.sh` - 启停服务
- ✅ `backup_database.sh` - 定期备份
- 📖 `README.md` - 运维手册
- 📖 `QUICKSTART.md` - 快速参考

**常用命令：**
```bash
./start.sh              # 启动
./stop.sh               # 停止
./backup_database.sh    # 备份
tail -f logs/server.log # 查看日志
```

### 场景3: 完整环境迁移（当前场景）
**需要的文件：**
- ✅ **全部文件** - 完整打包
- ✅ `package.sh` - 自动打包工具
- ✅ `backup_database.sh` - 数据库备份
- ✅ `restore_database.sh` - 数据库恢复
- 📖 `MIGRATION_FULL.md` - 完整迁移指南

**步骤：**
```bash
# 源服务器
./package.sh  # 选择 y 备份数据库

# 传输
scp *.tar.gz user@new-server:/opt/

# 目标服务器
tar -xzf *.tar.gz
cd microservice-deploy
vim config.yaml
./restore_database.sh database_backup/*.sql.gz
./start.sh
```

### 场景4: 仅更新代码（不涉及数据库）
**需要的文件：**
- ✅ `music_server` - 新的可执行文件
- ✅ `stop.sh`, `start.sh` - 重启服务

**步骤：**
```bash
./stop.sh
# 替换 music_server 文件
./start.sh
```

### 场景5: 开发环境编译
**需要的文件：**
- ✅ `build.sh` - 编译脚本
- ✅ 源码目录（上级目录）

**步骤：**
```bash
./build.sh  # 从 ../cmd/monolith/main.go 编译
```

---

## 📊 文件依赖关系

```
部署包根目录
│
├─ 核心运行文件 (必需)
│  ├─ music_server ━━━━━━━━━━━━━━━┓
│  └─ config.yaml ━━━━━━━━━━━━━━━┫━━> 启动服务
│                                  ┃
├─ 管理脚本                        ┃
│  ├─ start.sh ━━━━━━━━━━━━━━━━━━┛
│  └─ stop.sh
│
├─ 数据库管理
│  ├─ backup_database.sh ━━━━━━━> database_backup/*.sql.gz
│  └─ restore_database.sh ━━━━━┛
│
├─ 部署工具
│  ├─ package.sh ━━━> 完整部署包.tar.gz
│  └─ build.sh ━━━> music_server
│
└─ 文档 (辅助)
   ├─ QUICKSTART.md    - 快速参考
   ├─ README.md        - 使用手册
   ├─ DEPLOY.md        - 部署指南
   └─ MIGRATION_FULL.md - 迁移指南
```

---

## 🔐 敏感文件提醒

### ⚠️ 不要公开的文件
- `config.yaml` - 包含数据库密码
- `database_backup/*.sql.gz` - 包含用户数据
- `*.pid` - 进程 ID 文件（运行时生成）
- `logs/*.log` - 可能包含敏感信息

### ✅ 可以公开的文件
- 所有 `.sh` 脚本（不含敏感信息）
- 所有 `.md` 文档
- `config.yaml.example`（配置模板）

---

## 📏 文件大小参考

| 类型 | 典型大小 | 说明 |
|------|----------|------|
| `music_server` | 10-20 MB | 取决于编译选项 |
| `database_backup/*.sql.gz` | 1-50 MB | 取决于数据量 |
| `uploads/*` | 不定 | 用户上传的音乐文件 |
| `logs/*.log` | 0-100 MB | 建议定期清理 |
| 完整部署包 | 10-100 MB | 含可执行文件和数据库备份 |

---

## 🗑️ 清理建议

### 可以安全删除的文件
```bash
# 旧的日志文件（保留最近7天）
find logs/ -name "*.log" -mtime +7 -delete

# 旧的数据库备份（保留最近5个）
ls -t database_backup/*.sql.gz | tail -n +6 | xargs rm -f

# 旧的部署包
rm -f *.tar.gz

# 临时文件
rm -f *.pid
```

### 不要删除的文件
- `music_server` - 可执行文件
- `config.yaml` - 当前配置
- 所有 `.sh` 脚本
- `uploads/` 目录内容（用户数据）

---

## 📋 部署前检查清单

使用此清单确保所有必要文件都已准备：

- [ ] `music_server` - 可执行文件存在
- [ ] `config.yaml` - 配置文件已修改
- [ ] `database_backup/*.sql.gz` - 数据库备份存在（如需要）
- [ ] `start.sh`, `stop.sh` - 脚本可执行 (chmod +x)
- [ ] `uploads/` - 目录存在且有写权限
- [ ] `logs/` - 目录存在且有写权限
- [ ] MySQL - 数据库服务运行中
- [ ] Redis - 缓存服务运行中

---

## 🆘 获取帮助

根据您的问题，查看对应的文档：

| 问题类型 | 查看文档 |
|----------|----------|
| "怎么快速启动？" | QUICKSTART.md |
| "如何日常维护？" | README.md |
| "首次部署步骤？" | DEPLOY.md |
| "如何迁移整个环境？" | **MIGRATION_FULL.md** ⭐ |
| "文件都是干什么的？" | FILE_MANIFEST.md (本文档) |

---

**最后更新：2026-02-25**
