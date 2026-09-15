# Workflow safety

Only `run_tests.yml` is active while this experimental fork is being bootstrapped.
It validates pushes and pull requests for the `forever` integration branch.

The inherited write-capable, release, deployment, database-update, labeling, and upstream webhook workflows are retained as `*.yml.disabled` for reference. Review their repository names, secrets, permissions, products, and destinations before re-enabling any of them.

`tools/ci/check_workflow_policy.sh` enforces this allowlist in CI and in the
installed pre-push hook. Run `make setup` after cloning to install the hook.
