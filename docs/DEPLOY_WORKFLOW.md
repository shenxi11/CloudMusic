# 双目录开发与部署工作流

更新时间：2026-04-17

## 目录职责

- `/home/shen/microservice-deploy`
  - 唯一开发仓库
  - 唯一允许的代码修改、测试、提交、推送目录
  - 功能分支开发
  - 本地编译、接口测试、Docker 临时验证
- `/home/shen/CloudMusic`
  - 唯一正式运行目录
  - 固定跟踪 `origin/main`
  - 只用于拉取代码并启动 `cloudmusic` 服务
  - 不允许作为代码提交目录或功能修复目录

## 强制流程

### 1. 开发侧

```bash
cd /home/shen/microservice-deploy

git checkout main
git pull --ff-only origin main
git checkout -b feature/xxx

# 开发、测试、验证
go test ./...
# 若改动 HTTP 路由、handler、service：补接口冒烟测试
# 若改动 Dockerfile、docker-compose.yml、部署脚本、配置生成脚本：补 Docker / 部署冒烟
# 若改动 migrations/sql：补迁移验证和至少一个受影响接口或查询验证

# 提交并推送
git add <files>
git commit -m "feat: ..."
git push -u origin feature/xxx
```

功能合并到 `origin/main` 后，才允许进入正式部署流程。

### 2. 正式部署侧

```bash
cd /home/shen/CloudMusic
./scripts/deploy_from_main.sh
```

正式目录只允许做这些事情：

- `git fetch` / `git pull --ff-only origin main`
- `./scripts/deploy_from_main.sh`
- 健康检查、日志查看、容器状态排查

## 正式部署脚本行为

`./scripts/deploy_from_main.sh` 会自动：

1. 检查当前分支必须为 `main`
2. 检查工作区必须干净
3. 检查远端 `origin` 可访问
4. 拉取最新 `origin/main`
5. 生成 `configs/config.docker.generated.yaml`
6. 调用 `./start_docker.sh` 部署 `cloudmusic`
7. 输出 `http://127.0.0.1:8080/health` 健康检查结果

`./start_docker.sh` / `./scripts/docker.sh` 的 `restart` 流程必须遵循：

1. 先渲染配置
2. 先完成镜像构建
3. 构建成功后再 `down`
4. 最后 `up -d`

这样即使构建失败，也不会因为“先停服务再构建”而把线上环境直接打空。

## 网络与构建前置

正式机如果无法稳定访问 Docker Hub 或 `proxy.golang.org`，必须先补齐下面两个前置条件，再执行部署：

1. Docker registry mirror

示例 `/etc/docker/daemon.json`：

```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io"
  ]
}
```

修改后执行：

```bash
systemctl restart docker
```

2. Go 模块代理

在 `.env.docker` 中显式配置：

```bash
GOPROXY=https://goproxy.cn,direct
GOSUMDB=sum.golang.google.cn
GOPRIVATE=
```

`docker-compose.yml` 会把这些变量透传给 `Dockerfile` 的构建阶段，避免正式机因为访问 `proxy.golang.org` 超时而构建失败。

## 禁止事项

1. 不要在 `CloudMusic` 上停留在功能分支运行服务
2. 不要在 `CloudMusic` 上直接修改业务代码后再启动服务
3. 不要把 `microservice-deploy` 当作正式运行目录长期启动
4. 不要跳过 Git 同步直接拿本地脏代码部署正式环境
5. 不要在 `CloudMusic` 上提交代码、`cherry-pick` 热修、打临时补丁
6. 不要通过“宿主机构建二进制后注入现有镜像”等旁路方式发布正式环境
7. 线上故障也不能绕过本流程；修复必须回到 `microservice-deploy` 完成后再发版
