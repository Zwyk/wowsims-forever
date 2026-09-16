# Workflow safety

Two workflows are active:

- `run_tests.yml` validates pushes and pull requests for the `forever`
  integration branch, including the Classic 60 preview build and WASM smoke test.
  It can also be called by another allowlisted workflow.
- `pages.yml` publishes **only** the standalone Classic 60 diagnostic from
  `dist/forever-preview`, after the complete reusable test workflow succeeds.
  It runs on pushes to `forever` or a manual dispatch on `forever`, and only in
  `Zwyk/wowsims-forever`. It never deploys pull-request code.

Both default to `contents: read`. Only the Pages deploy job receives
`pages: write` and `id-token: write`; that job neither checks out nor executes
repository code. Deployment uses the protected `github-pages` environment,
SHA-pinned official actions, and an independently validated artifact. No PAT,
upstream deployment repository, `gh-pages` branch push, automatic Pages
enablement, or repository-settings modification is used.

The Pages pipeline deliberately reruns the reusable checks on its exact commit
instead of trusting another workflow's artifacts or latest status. PR builds
exercise the same build and smoke test without Pages credentials or uploads.

The inherited write-capable, release, deployment, database-update, labeling, and upstream webhook workflows are retained as `*.yml.disabled` for reference. Review their repository names, secrets, permissions, products, and destinations before re-enabling any of them.

`tools/ci/check_workflow_policy.sh` enforces this allowlist in CI and in the
installed pre-push hook. Run `make setup` after cloning to install the hook.

See [Pages preview setup and testing](../../docs/forever_pages.md) for the required
one-time repository settings and the limits of the published diagnostic.
