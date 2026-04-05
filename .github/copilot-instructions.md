When performing a code review for this repository:

- Focus first on correctness, regressions, data consistency, API compatibility, and missing validation.
- Treat this codebase as a Go microservice platform with a React frontend under `web-player/`.
- For backend changes, pay extra attention to HTTP handlers, schema changes, transaction safety, and compatibility with existing `/user/*`, `/recommendations/*`, and gateway-routed APIs.
- For frontend changes in `web-player/`, pay extra attention to mobile responsiveness, playback behavior, and regressions in API integration.
- Prefer concise review comments in Simplified Chinese.
- Do not suggest large refactors unless they are necessary to fix a concrete bug or major maintainability risk in the changed code.
