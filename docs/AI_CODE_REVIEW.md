# AI 自动 Code Review 配置

仓库已提供 GitHub Actions 工作流：`.github/workflows/ai-code-review.yml`。

## 生效方式

当 PR 发生以下事件时，工作流会自动运行并在 PR 下创建或更新一条 AI review 评论：

- `opened`
- `synchronize`
- `reopened`
- `ready_for_review`

默认仅处理当前仓库内的分支 PR，不处理 fork PR。

## 必需配置

在 GitHub 仓库 `Settings -> Secrets and variables -> Actions` 中新增：

- Secret: `OPENAI_API_KEY`

## 可选配置

可选新增以下 Repository Variables：

- `OPENAI_CODE_REVIEW_MODEL`
  默认值：`gpt-4.1-mini`
- `OPENAI_CODE_REVIEW_MAX_DIFF_CHARS`
  默认值：`120000`
- `OPENAI_CODE_REVIEW_SKIP_DRAFTS`
  默认值：`true`

## 说明

- 工作流使用 OpenAI Responses API 生成审查意见。
- 评审内容聚焦当前 diff 的正确性、回归风险、安全问题和测试缺口。
- 如果同一个 PR 后续有新提交，原评论会被自动更新，不会持续刷屏.
