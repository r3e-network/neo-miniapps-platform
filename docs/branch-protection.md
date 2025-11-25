# Branch Protection & Required Checks

Keep `master` stable by enforcing branch protection on GitHub with required status
checks. These are the build gates we expect to stay green:

- Workflow: `neo-smoke`
  - Job: `neo` (runs `go test ./...`, dashboard `npm run typecheck`, and a mocked `/neo/status` curl).
- Workflow: `dashboard-e2e` (optional, recommended as a required check once stable)
  - Job: `e2e` (runs Playwright smoke against a running stack when triggered via `workflow_dispatch` or scheduled windows).

Recommended GitHub settings (Repository → Settings → Branches → Add rule):

1) Branch name pattern: `master`
2) Require a pull request before merging (enable code owners if applicable).
3) Require status checks to pass before merging, and select `neo-smoke` (this will
   enforce the `neo` job including dashboard typecheck).
4) Require branches to be up to date before merging.
5) (Optional) Require signed commits, restrict who can push, and allow force-push
   only for administrators if you need hotfix overrides.

If you add more CI workflows, append them here and mark them as required in the
same rule to keep coverage consistent (e.g., perf/load tests or lint suites).
