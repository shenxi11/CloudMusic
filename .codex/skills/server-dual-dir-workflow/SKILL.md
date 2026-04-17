---
name: server-dual-dir-workflow
description: Enforce the CloudMusic backend dual-directory workflow. Use when handling service-side development, testing, release, deployment, hotfix, rollback, or production troubleshooting so code changes happen only in /home/shen/microservice-deploy and /home/shen/CloudMusic is used only to pull origin/main and deploy.
---

# Server Dual-Dir Workflow

Use this skill for any CloudMusic backend task that touches development, testing, Git flow, release, deployment, hotfix, rollback, or production troubleshooting.

## Directory Contract

- `/home/shen/microservice-deploy`
  - The only directory for backend code changes
  - The only directory for tests, commits, pushes, and feature branches
- `/home/shen/CloudMusic`
  - The only production runtime directory
  - Must track `origin/main`
  - Only used to pull code and deploy with `./scripts/deploy_from_main.sh`

## Required First Step

Before doing anything substantial, explicitly classify the task as one of:

- `dev/test/commit`
- `deploy/verify`

Then confirm you are in the correct directory for that class of work.

## Dev/Test/Commit Rules

Run all development work in `/home/shen/microservice-deploy`.

Required flow:

```bash
cd /home/shen/microservice-deploy
git checkout main
git pull --ff-only origin main
git checkout -b feature/<topic>
```

Minimum validation gate before commit:

- Always run `go test ./...`
- If HTTP routes, handlers, or services changed: run the affected HTTP smoke checks
- If `Dockerfile`, `docker-compose.yml`, deploy scripts, or config render scripts changed: run Docker or deploy smoke checks
- If `migrations/sql/` changed: verify the migration path and at least one affected API or query

After validation:

- Commit in `/home/shen/microservice-deploy`
- Push from `/home/shen/microservice-deploy`
- Merge to `origin/main`

Do not claim production is updated until the code is merged to `origin/main` and deployed from `CloudMusic`.

## Deploy/Verify Rules

Run production deployment work only in `/home/shen/CloudMusic`.

Allowed actions in `CloudMusic`:

- `git fetch`, `git pull --ff-only origin main`
- `./scripts/deploy_from_main.sh`
- health checks, status checks, and log inspection

Forbidden actions in `CloudMusic`:

- editing backend code
- creating commits
- `cherry-pick`, ad hoc patches, or local hotfix branches
- compiling binaries for release by hand
- injecting binaries into an existing image or any other release bypass

If deployment fails, fix the real problem in `/home/shen/microservice-deploy`, submit it to Git, and redeploy from `CloudMusic`. Production incidents do not override this rule.

## Response Contract

For backend workflow requests, state these items early in the reply:

- current work class: `dev/test/commit` or `deploy/verify`
- correct working directory
- any conflict between the user request and this workflow

If the user asks to modify code directly in `CloudMusic` or to bypass the Git-to-deploy flow, refuse that path and redirect to the standard workflow above.
