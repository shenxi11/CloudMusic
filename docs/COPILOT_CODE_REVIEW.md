# GitHub Copilot Code Review 配置

本仓库使用 GitHub 官方提供的 Copilot code review，不再使用自定义 OpenAI API workflow。

## 当前仓库内已提供的内容

- 仓库级 Copilot review 指令文件：`.github/copilot-instructions.md`

该文件会在 Copilot 审查 PR 时提供额外上下文，帮助它更关注接口兼容性、事务安全、前端回归等项目重点。

## 如何开启自动 Review

按 GitHub 官方文档，仓库管理员可以在仓库规则里开启自动 Copilot review：

1. 进入仓库 `Settings`
2. 打开 `Rules`
3. 点击 `Rulesets`
4. 新建或编辑一个 `Branch ruleset`
5. 将 `Enforcement status` 设为 `Active`
6. 选择目标分支，例如默认分支
7. 在 `Branch rules` 中勾选 `Automatically request Copilot code review`
8. 可选勾选：
   - `Review new pushes`
   - `Review draft pull requests`

## 手动请求 Review

如果没有开启自动 review，也可以在 PR 页面手动请求：

1. 打开一个 Pull Request
2. 在 `Reviewers` 菜单中选择 `Copilot`

## 注意事项

- Copilot review 是 GitHub 官方能力，不需要仓库 secret `OPENAI_API_KEY`
- Copilot review 留下的是 `Comment` 类型 review，不会自动 `Approve`，也不会自动 `Request changes`
- 如果启用了自动 review，但没有开启 `Review new pushes`，Copilot 默认只会在 PR 初次触发时审查一次
- Copilot code review 会读取 base branch 中的 `.github/copilot-instructions.md`
