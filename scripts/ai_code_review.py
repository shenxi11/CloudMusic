#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import textwrap
import urllib.error
import urllib.request


COMMENT_MARKER = "<!-- openai-ai-code-review -->"
DEFAULT_MODEL = "gpt-4.1-mini"
DEFAULT_MAX_DIFF_CHARS = 120000
DEFAULT_SKIP_DRAFTS = True

SYSTEM_PROMPT = textwrap.dedent(
    """\
    你是一名资深代码审查工程师。

    你的任务是基于 pull request 的标题、描述、变更文件和 diff，给出简洁、可执行的代码审查意见。

    输出规则：
    1. 仅审查当前 diff，不要猜测未改动文件。
    2. 优先关注：正确性、回归风险、安全问题、边界条件、数据一致性、错误处理、并发问题、缺失测试。
    3. 先给“发现的问题”，再给“建议验证点”。
    4. 如果没有明显阻塞问题，明确写“未发现明显阻塞问题”。
    5. 使用简体中文输出，保持 Markdown 格式，内容控制在 400 字以内。
    """
)


def getenv_bool(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    return raw.strip().lower() not in {"0", "false", "no", "off"}


def run_command(args):
    return subprocess.check_output(args, text=True)


def collect_text(node, sink):
    if isinstance(node, dict):
        node_type = node.get("type")
        text = node.get("text")
        if isinstance(text, str) and node_type in {"output_text", "text"}:
            sink.append(text)
        for value in node.values():
            collect_text(value, sink)
        return

    if isinstance(node, list):
        for item in node:
            collect_text(item, sink)


def build_diff(base_sha: str, head_sha: str, max_chars: int) -> tuple[str, bool]:
    diff = run_command(["git", "diff", "--unified=1", f"{base_sha}...{head_sha}"])
    truncated = len(diff) > max_chars
    if truncated:
        diff = diff[:max_chars].rstrip() + "\n\n[diff truncated]"
    return diff, truncated


def build_changed_files(base_sha: str, head_sha: str) -> str:
    return run_command(["git", "diff", "--name-status", f"{base_sha}...{head_sha}"]).strip()


def openai_review(payload: dict, api_key: str) -> str:
    body = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        "https://api.openai.com/v1/responses",
        data=body,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )

    with urllib.request.urlopen(request) as response:
        result = json.loads(response.read().decode("utf-8"))

    texts = []
    collect_text(result.get("output", []), texts)
    review = "\n".join(part.strip() for part in texts if part and part.strip()).strip()
    if not review:
        raise RuntimeError("OpenAI response did not contain review text.")
    return review


def github_request(url: str, token: str, method: str = "GET", data: dict | None = None):
    encoded = None if data is None else json.dumps(data).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=encoded,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
            "Content-Type": "application/json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
        method=method,
    )

    with urllib.request.urlopen(request) as response:
        return json.loads(response.read().decode("utf-8"))


def upsert_comment(repository: str, issue_number: int, token: str, body: str):
    comments_url = f"https://api.github.com/repos/{repository}/issues/{issue_number}/comments"
    comments = github_request(comments_url, token)

    existing = next(
        (comment for comment in comments if COMMENT_MARKER in comment.get("body", "")),
        None,
    )

    if existing:
        github_request(existing["url"], token, method="PATCH", data={"body": body})
        return "updated"

    github_request(comments_url, token, method="POST", data={"body": body})
    return "created"


def main():
    github_token = os.getenv("GITHUB_TOKEN")
    openai_api_key = os.getenv("OPENAI_API_KEY")
    event_path = os.getenv("GITHUB_EVENT_PATH")
    repository = os.getenv("GITHUB_REPOSITORY")

    if not github_token:
        print("GITHUB_TOKEN is required.", file=sys.stderr)
        return 1

    if not openai_api_key:
        print("OPENAI_API_KEY is not set. Skipping AI review.")
        return 0

    if not event_path or not repository:
        print("Missing GitHub event context.", file=sys.stderr)
        return 1

    with open(event_path, "r", encoding="utf-8") as handle:
        event = json.load(handle)

    pull_request = event["pull_request"]
    if pull_request.get("draft") and getenv_bool("AI_REVIEW_SKIP_DRAFTS", DEFAULT_SKIP_DRAFTS):
        print("Draft pull request detected. Skipping AI review.")
        return 0

    base_sha = pull_request["base"]["sha"]
    head_sha = pull_request["head"]["sha"]
    pr_number = event["number"]
    pr_title = pull_request["title"]
    pr_body = pull_request.get("body") or "(no description)"
    pr_url = pull_request["html_url"]

    max_diff_chars = int(os.getenv("AI_REVIEW_MAX_DIFF_CHARS") or DEFAULT_MAX_DIFF_CHARS)
    model = os.getenv("OPENAI_MODEL") or DEFAULT_MODEL

    changed_files = build_changed_files(base_sha, head_sha)
    diff, truncated = build_diff(base_sha, head_sha, max_diff_chars)

    prompt = textwrap.dedent(
        f"""\
        PR: {pr_title}
        URL: {pr_url}
        Base SHA: {base_sha}
        Head SHA: {head_sha}

        PR 描述:
        {pr_body}

        变更文件:
        {changed_files or "(no changed files found)"}

        Diff:
        {diff or "(empty diff)"}
        """
    )

    response_payload = {
        "model": model,
        "instructions": SYSTEM_PROMPT,
        "input": prompt,
    }

    try:
        review = openai_review(response_payload, openai_api_key)
    except urllib.error.HTTPError as error:
        details = error.read().decode("utf-8", errors="replace")
        print(f"OpenAI API error: {error.code} {details}", file=sys.stderr)
        return 1
    except RuntimeError as error:
        print(str(error), file=sys.stderr)
        return 1

    review_body = textwrap.dedent(
        f"""\
        {COMMENT_MARKER}
        ## AI Code Review

        模型：`{model}`
        {'注：本次 diff 已按长度截断。' if truncated else ''}

        {review}

        ---
        本评论由 GitHub Actions 自动更新。
        """
    ).strip()

    try:
        action = upsert_comment(repository, pr_number, github_token, review_body)
    except urllib.error.HTTPError as error:
        details = error.read().decode("utf-8", errors="replace")
        print(f"GitHub API error: {error.code} {details}", file=sys.stderr)
        return 1

    print(f"AI review comment {action} for PR #{pr_number}.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
