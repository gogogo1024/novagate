# GitHub Actions CI/CD 指南

本项目配置了完整的 GitHub Actions 工作流，用于自动化测试、构建和发布。

## 工作流概览

| Workflow | 触发条件 | 用途 |
|----------|---------|------|
| [ci.yml](.github/workflows/ci.yml) | push/PR | 完整测试套件 + Redis 集成测试 |
| [pre-commit.yml](.github/workflows/pre-commit.yml) | push/PR | 代码格式 + 静态检查 |
| [docker-build.yml](.github/workflows/docker-build.yml) | push/PR/tag | Docker 镜像构建 + 推送 |
| [release.yml](.github/workflows/release.yml) | tag (v*.*.*) | 二进制发布 + GitHub Release |

## 1. CI 测试工作流 (ci.yml)

### 特性

- ✅ **Redis 服务容器**：自动启动 Redis 7-alpine，healthcheck 确保可用
- ✅ **根模块测试**：`go test -race -coverprofile` 检测竞态条件
- ✅ **ACL 模块测试**：独立 go.mod 的子模块测试
- ✅ **命令映射验证**：确保 protocol/commands.go、server、dispatcher 三者一致
- ✅ **覆盖率上传**：可选集成 Codecov（需配置 `CODECOV_TOKEN`）

### 本地模拟

```bash
# 启动 Redis
docker-compose up -d

# 运行相同的测试
mise exec -- go vet ./...
mise exec -- go test -v -race ./...
cd services/acl && go test -v -race ./...
mise exec -- go run ./cmd/validate-commands
```

### 预期结果

- 所有测试 PASS（根模块 + ACL 模块）
- 2 个 Redis store 测试 SKIP（Lua 脚本，正常）
- 命令映射一致性通过

## 2. Pre-commit 检查 (pre-commit.yml)

### 检查项

1. **`go fmt` 格式化**：未格式化代码会失败，输出 diff
2. **`go vet` 静态分析**：检测常见错误
3. **命令映射一致性**：防止 command 定义/注册/使用不一致
4. **TODO/FIXME 警告**：建议添加 issue 引用（如 `TODO(#123)`）
5. **硬编码凭证检查**：基础模式匹配（`password.*=.*["']`）

### 本地运行

```bash
# 格式化检查
mise exec -- go fmt ./...
git diff --exit-code  # 确保无改动

# 静态分析
mise exec -- go vet ./...

# 命令映射
mise exec -- go run ./cmd/validate-commands
```

### 失败处理

- **go fmt 失败**：运行 `mise exec -- go fmt ./...` 并提交
- **硬编码凭证**：检查匹配的文件，确保是 test/example 或移除

## 3. Docker 镜像构建 (docker-build.yml)

### 构建矩阵

两个独立镜像：
- `novagate-server`：网关服务端（根目录 `Dockerfile.server`）
- `novagate-acl`：ACL 子服务（`services/acl/Dockerfile`）

### 镜像标签

| 触发方式 | 标签示例 |
|---------|---------|
| push main | `ghcr.io/gogogo1024/novagate-server:main` |
| PR #42 | `ghcr.io/gogogo1024/novagate-server:pr-42` |
| tag v1.2.3 | `ghcr.io/gogogo1024/novagate-server:1.2.3`, `:1.2`, `:latest` |
| commit abc123 | `ghcr.io/gogogo1024/novagate-server:sha-abc123` |

### 本地测试构建

```bash
# 测试 server 镜像
docker build -f Dockerfile.server -t novagate-server:test .

# 测试 ACL 镜像
docker build -f services/acl/Dockerfile -t novagate-acl:test services/acl

# 运行测试镜像
docker run --rm -p 9000:9000 novagate-server:test
docker run --rm -p 8888:8888 novagate-acl:test
```

### 推送到 GHCR

镜像会自动推送到 GitHub Container Registry：
- 需要 `packages: write` 权限（workflow 已配置）
- 访问：https://github.com/gogogo1024/novagate/pkgs/container/novagate-server

## 4. 发布工作流 (release.yml)

### 触发条件

打 tag：`git tag v1.0.0 && git push origin v1.0.0`

### 构建产物

交叉编译 6 个平台：

**服务端**：
- `novagate-server-linux-amd64`
- `novagate-server-linux-arm64`
- `novagate-server-darwin-amd64`
- `novagate-server-darwin-arm64`

**客户端**：
- `novagate-client-linux-amd64`
- `novagate-client-darwin-arm64`

**ACL**：
- `novagate-acl-linux-amd64`
- `novagate-acl-linux-arm64`
- `novagate-acl-darwin-arm64`

### 发布包

打包为 `.tar.gz`：
- `novagate-v1.0.0-linux-amd64.tar.gz`
- `novagate-v1.0.0-darwin-arm64.tar.gz`
- ...

### Changelog 生成

自动从 git log 生成 changelog（当前版本与上个 tag 之间的提交）

### 本地测试

```bash
# 模拟交叉编译
GOOS=linux GOARCH=amd64 mise exec -- go build -o dist/server-linux ./cmd/server
GOOS=darwin GOARCH=arm64 mise exec -- go build -o dist/server-darwin ./cmd/server

# 打包
tar -czf novagate-test-linux-amd64.tar.gz -C dist server-linux
```

## 配置 Secrets（可选）

某些功能需要配置 GitHub Secrets：

### Codecov 集成

1. 在 https://codecov.io/ 获取 token
2. 添加到仓库 Secrets：`CODECOV_TOKEN`

### Docker Registry（非 GHCR）

如果推送到其他 registry（Docker Hub、私有 Harbor）：

```yaml
- name: Log in to Docker Hub
  uses: docker/login-action@v3
  with:
    username: ${{ secrets.DOCKER_USERNAME }}
    password: ${{ secrets.DOCKER_PASSWORD }}
```

## 本地验证工具

### 1. 验证 workflow 语法

```bash
# 安装 actionlint（可选）
brew install actionlint  # macOS
# 或
go install github.com/rhysd/actionlint/cmd/actionlint@latest

# 运行验证
./scripts/validate-workflows.sh
```

### 2. 使用 act 本地运行（高级）

```bash
# 安装 act（https://github.com/nektos/act）
brew install act  # macOS

# 运行 CI workflow
act push

# 运行特定 job
act -j test
```

**注意**：act 需要 Docker，且不支持所有 GitHub Actions 特性（如服务容器在某些版本有限制）

## 故障排查

### CI 失败：Redis 连接超时

检查 healthcheck 配置：

```yaml
services:
  redis:
    options: >-
      --health-cmd "redis-cli ping"
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
```

### CI 失败：测试超时

可能是竞态条件或死锁，本地运行：

```bash
mise exec -- go test -v -race -timeout 30s ./...
```

### Docker 构建失败：找不到文件

检查 `.dockerignore`，确保关键文件未被排除：

```bash
# 本地测试构建
docker build -f Dockerfile.server -t test .
```

### 发布失败：权限不足

确保 workflow 有 `contents: write` 权限：

```yaml
permissions:
  contents: write
```

## 最佳实践

1. **PR 前本地测试**：运行 `./scripts/test.sh test` 确保通过
2. **提交前格式化**：`mise exec -- go fmt ./...`
3. **命令映射一致性**：修改 command 后运行 `./cmd/validate-commands`
4. **Docker 镜像测试**：发版前本地构建验证
5. **语义化版本**：遵循 `v<major>.<minor>.<patch>` 格式打 tag

## 监控与通知（可选）

### Slack 通知

添加到 workflow：

```yaml
- name: Notify Slack
  if: failure()
  uses: slackapi/slack-github-action@v1
  with:
    webhook-url: ${{ secrets.SLACK_WEBHOOK_URL }}
    payload: |
      {
        "text": "CI failed: ${{ github.repository }} - ${{ github.ref }}"
      }
```

### GitHub Status Checks

在仓库 Settings → Branches → Branch protection rules：
- 启用 "Require status checks to pass before merging"
- 勾选 `test` (ci.yml) 和 `pre-commit-checks`

## 参考资源

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [mise Documentation](https://mise.jdx.dev/)
- [actionlint](https://github.com/rhysd/actionlint)

