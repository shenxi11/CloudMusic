# 双目录开发与部署工作流

更新时间：2026-04-07

## 目录职责

- `/home/shen/microservice-deploy`
  - 开发仓库
  - 功能分支开发
  - 本地编译、接口测试、Docker 临时验证
- `/home/shen/CloudMusic`
  - 正式运行目录
  - 固定跟踪 `origin/main`
  - 只用于拉取代码并启动 `cloudmusic` 服务

## 推荐流程

### 1. 开发侧

```bash
cd /home/shen/microservice-deploy

git checkout main
git pull --ff-only origin main
git checkout -b feature/xxx

# 开发、测试、验证
go test ./...
# 按需执行 docker / http 冒烟测试

# 提交并推送
git add <files>
git commit -m "feat: ..."
git push -u origin feature/xxx
```

功能合并到 `main` 后，才允许进入正式部署流程。

### 2. 正式部署侧

```bash
cd /home/shen/CloudMusic
./scripts/deploy_from_main.sh
```

## 正式部署脚本行为

`./scripts/deploy_from_main.sh` 会自动：

1. 检查当前分支必须为 `main`
2. 检查工作区必须干净
3. 检查远端 `origin` 可访问
4. 拉取最新 `origin/main`
5. 生成 `configs/config.docker.generated.yaml`
6. 调用 `./start_docker.sh` 部署 `cloudmusic`
7. 输出 `http://127.0.0.1:8080/health` 健康检查结果

## 禁止事项

1. 不要在 `CloudMusic` 上停留在功能分支运行服务
2. 不要在 `CloudMusic` 上直接修改业务代码后再启动服务
3. 不要把 `microservice-deploy` 当作正式运行目录长期启动
4. 不要跳过 Git 同步直接拿本地脏代码部署正式环境
