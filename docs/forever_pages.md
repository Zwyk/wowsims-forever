# Classic 60 preview on GitHub Pages

The published preview is a small browser diagnostic that runs the Go Classic
level-60 character-baseline code through WebAssembly. Its purpose is to make
the foundation inspectable while a complete Classic-like engine is being built.
It is **not a combat/DPS simulator**, and does not enable Forever mechanics.
The inherited TBC UI is deliberately not published: its engine remains level 70
and many of its asset/runtime URLs are fixed to `/tbc/`.

## One-time GitHub setup

In [repository Settings > Pages](https://github.com/Zwyk/wowsims-forever/settings/pages),
set **Build and deployment > Source > GitHub Actions**. Do not select a branch
publisher or create an upstream `pages-deploy` destination. The workflow does not
enable Pages or change repository settings on your behalf.

In **Settings > Environments > github-pages**, restrict deployment branches to
`forever`. If the environment does not exist yet, create it with that name before
the first deployment. Required reviewers may be added, but will intentionally
pause each automatic deployment for approval.

The workflow must be merged into `forever` before automatic publishing begins.
The default project-site URL, unless a custom domain is configured, is:

[zwyk.github.io/wowsims-forever/](https://zwyk.github.io/wowsims-forever/)

That link is the **expected destination**, not evidence of a successful deployment.
Use the successful deployment job's environment URL as the authoritative link.
See GitHub's [publishing-source instructions](https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site)
and [custom-workflow requirements](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).

## Automatic publishing and safety

`Publish Classic 60 Preview` runs for pushes to `forever` and manual dispatches
on `forever`, only in `Zwyk/wowsims-forever`. To retry after adjusting Pages
settings, open the workflow in Actions and select **Run workflow > forever**.
Manual dispatch requires the workflow to be present on the repository's default
branch; otherwise use a subsequent push to `forever`.

The workflow first calls the full `Run Tests` workflow. Only after it succeeds
does a read-only job check out the same triggering commit, build and smoke-test
the preview, validate its static files, and upload the Pages artifact. The final
job has only the Pages/OIDC permissions needed to publish and does not execute
repository code. Deployments share one concurrency group, and an in-progress
deployment is not canceled by a later push.

Pull requests build and smoke-test the preview in normal read-only CI. They do
not upload or publish Pages artifacts, and there is no `pull_request_target` or
privileged `workflow_run` path. All action references are SHA-pinned.

The artifact checker rejects missing or empty required files, symbolic links,
hard links, hidden content, non-regular files, and oversized output. The upload
uses GitHub's [official Pages artifact format](https://github.com/actions/upload-pages-artifact).
Build failures or failed engine tests leave the last successful website in place.
The build metadata identifies the revision being tested; check it before
reporting behavior, especially after a failed or still-running deployment.

## Local build and checks

Use the repository's Go 1.25.4, Protoc 29.3, protobuf Go plugin 1.36.10, and Node
version from `.nvmrc` (see [installation](installation.md)). From the repo root:

```sh
make sim/core/proto/api.pb.go
node tools/classic60preview/build.mjs
node tools/classic60preview/smoke.mjs dist/forever-preview
node --test tools/ci/check_pages_artifact.test.mjs
node tools/ci/check_pages_artifact.mjs dist/forever-preview
```

The publishable directory is `dist/forever-preview`. Serve that directory through
an HTTP static server; opening `index.html` via `file://` will not load WASM.
All preview assets use relative URLs so it can run under the repository's Pages
subpath without rewriting compiled output or changing the inherited TBC paths.

For a subpath check, serve `dist` and open `/forever-preview/`. Verify that the
page loads its WASM engine, shows the expected revision, accepts supported
original race/class combinations, and clearly labels its unfinished scope.
The smoke test checks the actual WASM API; the artifact check checks packaging,
not browser rendering.

## Limits and next steps

Publishing does not activate a level-60 profile in the normal simulator. Class
spells, talents, gear, combat scheduling, encounter integration, and other
remaining engine conversions must be completed and tested before the diagnostic
is presented as a playable Classic simulator. Forever-specific changes remain a
separate layer after that baseline is healthy. Follow the
[core rules ledger](forever_core.md) for the current implementation boundaries.
