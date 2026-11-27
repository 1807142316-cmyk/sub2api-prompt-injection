# Source Provenance

This private repository keeps the real upstream Git baseline on `main` and local extensions on `feature/prompt-injection`. It is not an official upstream release. Use the feature branch to build the customized prompt editor.

- Upstream repository: `https://github.com/codkeep/sub2api`
- Upstream remote: `upstream` (fetch source only)
- Baseline tag: `v0.2.38`
- Annotated tag object: `a83a7643259d8aff6ee5430ffefe7b5f493109ea`
- Baseline commit: `18b25db6eda44fb0dd18c51902a3b74c684b4961`
- Baseline commit date: `2026-11-27T23:52:54+08:00`
- Release published: `2026-11-27T16:02:01Z` (November 28 in Asia/Shanghai)
- Verification: fetched the explicit upstream tag, checked the tag's peeled commit,
  and fast-forwarded `main` from the previous v0.2.37 baseline. No archive hash or
  file-count claim from the previous release applies to this baseline.

`main` points to the actual upstream commit, with its original tree and identity. The history reachable from the baseline commit is retained; unrelated upstream branches and tags were not fetched. The upstream `LICENSE`, copyright notices, documentation, build files, package lockfiles, and executable file modes are retained.

The feature branch preserves task-based Skills routing, instruction layers, the
prompt editor and group-specific controls. It also adds per-account Codex request
compatibility controls. The v0.2.38 merge retains upstream Skills caching and
authenticated header variables alongside the local authenticated-group opt-out.
These changes are local extensions. The Go module path inherited from upstream
remains `github.com/Wei-Shaw/sub2api`.

See [the branch and upstream maintenance guide](docs/upstream-sync.md) for the update procedure. Synchronizing source branches does not deploy a running service.

The repository does not include a production database, account exports, administrator credentials, client API keys, configured prompt contents, private deployment environment files, or local authentication and upload helpers. New installations must configure their own runtime environment and prompt settings.
