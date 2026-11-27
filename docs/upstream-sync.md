# 分支与上游同步

日常使用、修改和构建定制版本请选择 `feature/prompt-injection`。私有仓库的默认分支也应设为这一分支。

| 分支 | 用途 |
| --- | --- |
| `main` | 保留真实上游提交，只按明确版本快进，不直接提交定制代码或维护文档。 |
| `feature/prompt-injection` | 保存任务 Skills、分层提示词、提示词编辑器和分组控制，以及相关测试和维护说明。 |

当前上游基线为 `codkeep/sub2api` 的 `v0.2.37`，提交为 `a47e14d7b053cbf714aee8100dc9cb5b9b51a93c`。已保留该提交及其上游祖先，没有抓取其他上游分支或标签。`main` 使用这个真实提交，功能分支以它为共同祖先，便于后续合并。

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

使用仓库声明的 Node、pnpm 和 Go 版本。前端先从锁文件安装依赖，再运行提示词相关测试和生产构建：

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm test:run src/components/admin/settings/__tests__/instructionLayersEditor.spec.ts src/components/admin/settings/__tests__/PromptTextEditor.spec.ts src/components/admin/settings/__tests__/InstructionLayersPanel.spec.ts
pnpm build
cd ../backend
go test -tags=unit ./internal/service ./internal/server/middleware -run 'InstructionLayer|Skill' -count=1
cd ..
git diff --check
git status --short
```

还应根据上游改动涉及的模块补充相应测试。实际检查提示词页面的编辑、预览、保存、未保存提醒与分组三种模式，并确认四种协议保留原请求内容、旧配置可读取、分组关闭能同时跳过自动 Skills。预览不代表已保存，也不包含完整的 Skills 选择结果。

不要将构建缓存、`node_modules`、生成的 `dist`、运行环境文件或实际提示词内容提交到源码仓库。需要记录的修复和来源说明应先在功能分支提交。

## 4. 推送源码

确认验证通过、工作区干净，再明确推送两个源码分支：

```sh
git push origin main
git push origin feature/prompt-injection
```

不向 `upstream` 推送，不使用 `--force`、`--mirror` 或 `--tags`。保留现有上游发布工作流，但本流程不创建发行标签，也不自动部署线上服务；部署是另一个独立步骤。
