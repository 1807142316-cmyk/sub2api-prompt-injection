# Source Provenance

This private repository keeps the real upstream Git baseline on `main` and local extensions on `feature/prompt-injection`. It is not an official upstream release. Use the feature branch to build the customized prompt editor.

- Upstream repository: `https://github.com/codkeep/sub2api`
- Upstream remote: `upstream` (fetch source only)
- Baseline tag: `v0.2.37`
- Annotated tag object: `0d4270049db226d1593dd77795cbcc8c24cecb7d`
- Baseline commit: `a47e14d7b053cbf714aee8100dc9cb5b9b51a93c`
- Baseline commit date: `2026-11-24T20:22:23+08:00`
- Source archive SHA-256: `f86520c9871b7502ee7f9aae55bbfed494d77aaf0f30fa6433dec8d593d5029a`
- Verified source files: 4,857 files matched the upstream checkout byte for byte.

`main` points to the actual upstream commit, with its original tree and identity. The history reachable from the baseline commit is retained; unrelated upstream branches and tags were not fetched. The upstream `LICENSE`, copyright notices, documentation, build files, package lockfiles, and executable file modes are retained.

The feature branch first preserves the existing task-based Skills routing and instruction-layer functionality, then adds the prompt editor improvements and group-specific controls. These changes are local extensions. The Go module path inherited from upstream remains `github.com/Wei-Shaw/sub2api`.

See [the branch and upstream maintenance guide](docs/upstream-sync.md) for the update procedure. Synchronizing source branches does not deploy a running service.

The repository does not include a production database, account exports, administrator credentials, client API keys, configured prompt contents, private deployment environment files, or local authentication and upload helpers. New installations must configure their own runtime environment and prompt settings.
