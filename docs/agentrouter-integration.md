# AgentRouter Responses 接入

2026-11-28 对主站 `https://agentrouter.org/v1` 的实测记录。服务端能力可能变化，下面区分已通过的调用与尚未验证的能力。实际密钥、代理地址和生产配置不在本仓库。

## 账号设置

- 平台：OpenAI；账号类型：API Key。
- 基础地址：`https://agentrouter.org/v1`。最终生成端点为 `/v1/responses`。
- 启用 HTTP Responses 透传，Responses 协议模式设为强制 Responses。
- 通过实际可连通的 HTTP 代理访问上游，保留 HTTPS 主机名与证书验证；由代理解析域名。
- 启用透传模型过滤，精确配置实际 Key 返回的模型。本次目录为 `gpt-6.2-astra`、`gpt-6.2-sol`，同名映射，不把它们冒充旧版模型。
- 本次普通 User-Agent 请求模型目录得到 `unauthorized_client_error`；使用本机真实 Codex 的 User-Agent 后成功。账号级请求头配置足以处理这一差异。会话、窗口和 turn 标识继续来自实际客户端，按请求转发。
- HTTP 透传与 WebSocket 是独立能力，本次只验收 HTTP。

创建账号会自动触发一次 Responses 能力探测；已直测支持 Responses 的账号应在首次创建时设置 `openai_responses_mode=force_responses`，避免自动探测重复消耗或误判。不用后台模型列表回退结果代替实际上游模型目录。

独立分组可选择“不注入提示词（含 Skills）”。账号属于多个分组时，实际使用哪套提示词取决于调用方 API Key 所属分组；账号同时加入无注入分组不会关闭其他分组的提示词。

## 参数与验收

真实通过的调用：

- 两个模型的流式文本输出，均收到 `response.completed`。
- Astra 的 `tool_choice=auto` 函数调用、严格参数 Schema、`parallel_tool_calls=false`。
- 工具结果回填后返回严格 `text.format=json_schema` 结果。
- `instructions`、`reasoning.effort=low`、`include=["reasoning.encrypted_content"]`、`store=false`、`prompt_cache_key`。
- 工具结果按无状态历史回放，本地及公网 Sub2 入口，逐请求用量归属核对。

每次成功需同时检查 HTTP 状态、流内错误、完整结束事件、实际输出和模型、网关用量记录。HTTP 200 本身不表示流成功；模型目录成功也不证明模型生成可用。

`openai_oauth_passthrough_test.go` 的完整请求回归覆盖 14 类 Responses 字段：`input`、`instructions`、`tools`、`tool_choice`、`parallel_tool_calls`、`reasoning`、`text`、`include`、`store`、`stream`、`previous_response_id`、`metadata`、`prompt_cache_key`、`service_tier`。同时覆盖空值、请求头边界、流式工具事件和受控兼容调整。这些 mock 测试证明网关转发行为，不代表上游支持所有参数组合。

## 已知边界

本次 Astra 请求使用 `tool_choice={"type":"function","name":"..."}` 时，上游发出函数调用事件后结束连接，没有 `response.completed` 或用量。相同请求直连上游可复现；Sub2 正确报告缺少结束事件。`tool_choice=auto` 的工具调用及后续 `tool_choice=none` 回填通过验证。不要通过伪造完成事件或用量掩盖该问题。

尚未真实验收：WebSocket、服务端保存与 `previous_response_id` 续接、其他推理档位、其他 service tier、图片及所有非流式参数组合。应根据调用方需要逐项确认。

代码维护沿用 `main` 上游与 `feature/prompt-injection` 定制分支；同步方式见 [upstream-sync.md](upstream-sync.md)。本接入使用已有账号配置，无需重建生产网关。
