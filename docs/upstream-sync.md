# 分支与上游同步

日常使用、修改和构建定制版本请选择 `feature/prompt-injection`。私有仓库的默认分支也应设为这一分支。

| 分支 | 用途 |
| --- | --- |
| `main` | 保留真实上游提交，只按明确版本快进，不直接提交定制代码或维护文档。 |
| `feature/prompt-injection` | 保存任务 Skills、分层提示词、提示词编辑器、分组控制和账号 Codex 请求兼容，以及相关测试和维护说明。 |

当前上游基线为 `codkeep/sub2api` 的 `v0.2.38`，提交为 `18b25db6eda44fb0dd18c51902a3b74c684b4961`。该版本于北京时间 2026-11-28 00:02 发布。`main` 使用这个真实上游提交，功能分支通过合并继承它；定制仍保留在功能分支。不要用上游目录或官方镜像覆盖定制版本。

`upstream` 指向 `https://github.com/codkeep/sub2api.git`，只用于获取上游源码；`origin` 指向自己的私有仓库，推送时明确指定 `origin`。两个远程地址都应先用 `git remote -v` 核对。

## 1. 获取指定版本

先确认工作区干净，并查看目标版本的变更。下面所有 `vX.Y.Z` 都要替换为本次已经选定的真实标签，不自动追随最新版本。

```sh
git status --short
git remote -v
git fetch --no-tags upstream "refs/tags/vX.Y.Z:refs/tags/vX.Y.Z"
git show --no-patch --format=fuller "vX.Y.Z^{commit}"
git diff --stat main "vX.Y.Z^{commit}"
git merge-base --is-ancestor main "vX.Y.Z^{commit}"
git branch "backup/before-vX.Y.Z" feature/prompt-injection
git bundle create "../before-vX.Y.Z.bundle" main feature/prompt-injection
```

`git status --short` 应没有输出。最后一条命令必须成功，才能继续更新 `main`。

不要因为祖先检查失败而重置或强推 `main`。如果两个版本分叉，保留现有分支并先检查标签、提交来源和历史；若自己曾使用 `--depth` 克隆，还应先补齐目标版本的历史。

## 2. 快进主分支并合并定制

```sh
git switch main
git merge --ff-only "vX.Y.Z^{commit}"
git switch feature/prompt-injection
git merge --no-ff main
```

`main` 上只允许快进。普通合并提交和定制修改都留在功能分支，不把功能分支合回 `main`。

出现冲突时，在功能分支中逐个核对上游行为与定制行为，解决后对相关文件执行 `git add`，再执行 `git commit` 完成合并。不要整批用某一侧覆盖冲突。需要放弃本次合并时，在尚未提交的合并中执行 `git merge --abort`。

合并完成后更新 `UPSTREAM_SOURCE.md` 中的基线版本、提交和验证记录。保留上游许可证、作者信息及构建文件。

## 3. 验证功能

使用仓库声明的 Node、pnpm 和 Go 版本。前端先从锁文件安装依赖，再运行提示词和账号相关测试及生产构建：

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm test:run src/components/admin/settings/__tests__/instructionLayersEditor.spec.ts src/components/admin/settings/__tests__/PromptTextEditor.spec.ts src/components/admin/settings/__tests__/InstructionLayersPanel.spec.ts
pnpm test:run src/components/account/__tests__/CreateAccountModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/credentialsBuilder.spec.ts src/components/account/__tests__/codexRequestCompat.spec.ts src/components/account/__tests__/CodexRequestCompatSettings.spec.ts
pnpm build
cd ../backend
go test -tags=unit ./internal/service ./internal/server/middleware -run 'InstructionLayer|Skill|HeaderOverride|CodexRequestCompat|Passthrough' -count=1
cd ..
git diff --check
git status --short
```

还应根据上游改动涉及的模块补充鉴权、模型准入、协议转换、流式错误、计费和管理 API 回归。实际检查提示词页面的编辑、预览、保存、未保存提醒与分组三种模式，并确认四种协议保留原请求内容、旧配置可读取、分组关闭能同时跳过自动 Skills。账号兼容开关应能保存、关闭和恢复，已有 User-Agent、安装标识及透传设置不能被覆盖。预览不代表已保存，也不包含完整的 Skills 选择结果。

按根目录 `AGENTS.md` 的要求，用相同工具链、机器、CPU 参数和数据分别测试升级前后的性能。保留升级前源码快照，并在两边运行相同的 `BenchmarkCustomizationSkills`，比较耗时、分配和设置读取次数。微基准通过不能代替生产性能观察。

不要将构建缓存、`node_modules`、生成的 `dist`、运行环境文件或实际提示词内容提交到源码仓库。需要记录的修复和来源说明应先在功能分支提交。

## 4. 推送源码

确认验证通过、工作区干净，再明确推送两个源码分支：

```sh
git push origin main
git push origin feature/prompt-injection
```

不向 `upstream` 推送，不使用 `--force`、`--mirror` 或 `--tags`。推送后核对仓库仍为 private、默认分支为功能分支、两个分支 SHA 正确、没有额外协作者或邀请。保留上游发布工作流；不依赖它自动部署本机定制镜像。

## 5. 部署与回退

1. 从已提交且验证通过的功能分支构建。前端产物位于 `backend/internal/web/dist`，后端需以 `-tags embed` 编译；镜像使用新的唯一版本号并记录功能分支完整提交号。
2. 部署前备份数据库、应用数据和当前 Compose 配置，记录旧镜像 ID。检查 `backend/migrations` 的真实变化；有新增迁移时先验证兼容和回退方案。本次 v0.2.37 到 v0.2.38 的迁移内容完全相同。
3. 临近切换时重新读取提示词、账号、分组、Key 及额度配置，避免覆盖升级期间用户的新修改。只替换 Compose 的 gateway 镜像，使用 `docker compose up -d --no-deps gateway`，沿用原数据库、Redis、代理和网络服务。
4. 检查容器健康、公开版本、前端资源、登录和设置回读、提示词与账号配置保持一致、中转域名维护服务正常。验证关键模型时确认用量记录的实际账号，不能只看 HTTP 200 或模型目录。
5. 新版异常时，将 gateway 镜像恢复为部署前记录的镜像并重建该服务。无迁移时通常只回退镜像；不要直接还原整库或整份旧设置，否则会丢掉用户刚保存的配置和使用记录。有迁移时按先前验证的兼容方案处理。

不要直接在生产执行“拉取官方 latest 并覆盖容器”。更新源码不会自动删除定制提交，但合并冲突仍需按行为处理并验证，无法保证每次上游更新都零冲突。网络域名自动维护属于独立运维配置，应继续保留，不能提交包含凭据的生产文件到 GitHub。
