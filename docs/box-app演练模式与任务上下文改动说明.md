# box-app 改动说明：演练模式与任务上下文管理

> 对应需求：`ai_docs/编排测试方案-真机测试.md` 第 4 节「盒子端最小改动」
> 状态：**已实现**，需要 box-app 发版。

本次为 box-app 增加两项能力，用于编排的**真机测试**（把编排临时下发到盒子，用测试用例逐帧真实执行）：

| 改动 | 目的 |
| --- | --- |
| 演练模式 `dry_run` | 测试时不真正发送企业微信/MQTT/HTTP/RPA 消息，避免污染真实渠道 |
| 任务上下文管理接口 | 初始化与清理测试用的跨帧状态，避免多次测试互相影响 |

两项都只影响**测试路径**：不传 `dry_run`、不调用新接口时，行为与改动前完全一致。

---

## 1. 演练模式 `dry_run`

### 用法

在手动触发执行的 `trigger_data` 里带上 `dry_run: true`：

```http
POST /api/v1/deployments/{deployment_id}/run
Content-Type: application/json

{
  "task_id": "wf_test_r_123_3",
  "dry_run": true,
  "input_variables": {
    "detections": [{ "class_id": 0, "confidence": 0.92 }],
    "image_base64": ""
  }
}
```

`trigger_data` 的键会被逐一写入工作流上下文（`workflow_executor.cpp` `createAndRunInstance`），因此 `dry_run` 自动成为上下文变量，无需额外解析。

### 生效范围

以下节点在 `dry_run` 为真时**只记录不发送**，输出 `{ dry_run: true, request: <本应发送的内容> }` 并标记成功：

| 节点 | 被跳过的动作 | `request` 内容 |
| --- | --- | --- |
| `wechat_work` | 写入 `wechat_message_queue`（及后续真实发送） | `webhook_key`、`body`（插值后） |
| `mqtt` | 连接 broker 并发布 | `topic`、`payload`、`broker` |
| `http_request` | 发起 HTTP 请求 | `url`、`method`、`headers`、`body`（插值后） |
| `rpa_wechat_proxy` | 调用 RPA 代理发送 | `webhook_url`、`rpa_proxy_url`、`body_text`、`body_image`、`mention_tags` |

`request` 里保留的是**插值完成后的实际内容**，因此断言可以检查报警文案是否正确（例如 `python_script-005.alarm_message` 是否被正确拼进消息体）。

### 判断规则

`NodeExecutor::isDryRun()` 只接受：

- 布尔 `true`
- 字符串 `"true"` 或 `"1"`

其他值（包括 `"false"`、`0`、缺省）一律视为关闭，避免误判导致该发的消息没发。

### 与仿真的契约一致

前端仿真器的「只记录不发送」节点输出同样的 `{ dry_run: true, request: ... }` 结构，因此**同一条断言在仿真和真机两边通用**。

> 注意：仿真的 `request` 结构是按本文档约定实现的（例如企微额外带 `msgtype`、MQTT 额外带 `published`/`payload_size` 等字段供断言使用），而真机是真实执行的输出，**两者字段不完全相同**。写断言时建议只依赖 `request` 里的核心字段（如 `request.body`、`request.payload`），这些是两边都有的。

### 不会跳过校验

`dry_run` 的判断放在**内容校验之后**。例如企业微信节点仍会先检查「消息内容为空」并报错——这是刻意的：测试必须能发现「body 引用写错导致内容为空」这类问题，而不是因为演练模式把它掩盖掉。

---

## 2. 任务上下文管理接口

### 为什么需要

`task_get_context` / `task_set_context` 通过 `TaskDatabase` 按 `task_id` 读写 `tasks.context_data`。

原有的 `setTaskContextData()` **只执行 UPDATE**。而 `tasks` 表的 `dev_id` / `rtsp_url` / `config_json` 是 NOT NULL，真机测试使用的合成 `task_id`（`wf_test_` 前缀）**没有对应的任务行**，UPDATE 匹配 0 行 —— 上下文写入会被**静默丢弃**，跨帧状态直接失效。

因此新增了 upsert 能力，并顺带补上清理接口。

### 接口

| 接口 | 说明 |
| --- | --- |
| `PUT /api/v1/task-contexts/{task_id}` | 写入/初始化上下文；无任务行时按 `allow_create` 决定是否创建占位记录 |
| `GET /api/v1/task-contexts/{task_id}` | 读取上下文 |
| `DELETE /api/v1/task-contexts/{task_id}` | 清理上下文（幂等） |
| `POST /api/v1/task-contexts/cleanup` | 按前缀批量清理测试残留 |

#### PUT 写入

```http
PUT /api/v1/task-contexts/wf_test_r_123_3
Content-Type: application/json

{
  "context_data": { "is_intrusion_detected": false, "count": 0 },
  "allow_create": true
}
```

- `context_data` 必须是 JSON 对象；也支持直接传对象作为请求体（不带 `context_data` 外层）。
- `allow_create` 默认 `true`。
- **已有真实任务行时走 UPDATE**（不动任务的其他字段）；没有任务行时创建占位记录，`config_json` 里打 `__test_context_only` 标记便于识别与清理。
- 真实任务的上下文写入路径不受影响。

#### DELETE 清理

- 有真实任务行：只把 `context_data` 清空为 `{}`，**不删任务**。
- 无任务行（测试占位记录）：整行删除，避免残留。
- 幂等：没删到行也返回成功，附带 `deleted: false`。

#### POST cleanup 批量清理

```http
POST /api/v1/task-contexts/cleanup
{ "prefix": "wf_test_" }
```

返回 `{ "success": true, "affected": 3 }`。**前缀为空会被拒绝**（400），避免误清全部任务上下文。

### 触发时初始化上下文（可选）

`trigger_data` 支持 `initial_task_context`，在触发前重置该 `task_id` 的持久化上下文：

```json
{
  "task_id": "wf_test_r_123_3",
  "initial_task_context": { "count": 0, "is_alarmed": false },
  "dry_run": true,
  "input_variables": { "detections": [] }
}
```

不传时完全不影响正常运行时的行为（沿用任务已有的上下文）。

---

## 3. 改动文件

| 文件 | 改动 |
| --- | --- |
| `include/node_executor.hpp` | 声明 `isDryRun()` |
| `src/node_executor.cpp` | 实现 `isDryRun()`；为企微/MQTT/HTTP/RPA 四个节点增加演练分支 |
| `include/database_manager.hpp` | 声明 `upsertTaskContextData` / `deleteTaskContextData` / `deleteTaskContextDataByPrefix` |
| `src/database_manager.cpp` | 实现上述三个方法 |
| `src/workflow_executor.cpp` | 支持 `trigger_data.initial_task_context` |
| `include/httplib_server.hpp` | 声明四个任务上下文 handler |
| `src/httplib_server_workflow.cpp` | 注册任务上下文路由并实现 handler |

---

## 4. 验证建议

改动涉及 C++ 运行时代码，**必须在真实盒子上验证**。建议按以下顺序：

1. **演练模式不发送**
   - 配一个含 `wechat_work` 节点的编排，`run` 时带 `dry_run: true`。
   - 确认 `GET /api/v1/workflow-instances/{id}/nodes` 里该节点输出含 `dry_run: true` 与 `request`。
   - **确认目标企微群没有收到消息。**

2. **非演练模式仍然真实发送**（回归）
   - 去掉 `dry_run`（或不传）再跑一次，确认消息正常发出、行为与改动前一致。

3. **上下文写入（关键）**
   - `PUT /api/v1/task-contexts/wf_test_demo_1` 写入 `{ "count": 0 }`。
   - 跑一个含 `task_get_context` → `task_set_context` 的编排，`task_id` 传 `wf_test_demo_1`。
   - 确认跨帧状态能正确累加（这是原 `setTaskContextData` 静默失败的场景）。

4. **清理**
   - `DELETE /api/v1/task-contexts/wf_test_demo_1`，确认返回 `deleted: true`。
   - `POST /api/v1/task-contexts/cleanup` 传 `wf_test_`，确认 `affected` 符合预期。
   - 确认真实任务的上下文**没有**被清理。

5. **安全检查**
   - `POST /api/v1/task-contexts/cleanup` 传空前缀，确认返回 400。

---

## 5. 已知限制

- **KVM 节点未纳入演练模式**：box-app 中不存在 `kvm` 节点类型（`node_executor.cpp` 的分发链里没有），因此无需处理。若后续新增该类型且带外部副作用，需要同样加演练分支。
- **`request` 结构无历史对照**：`dry_run` 是全新能力，其输出结构由本文档定义。真机测试前端已按此结构对接，但**仿真与真机的 `request` 字段不完全一致**（见上文说明）。
- **占位记录会残留在 `tasks` 表**：正常流程由 `DELETE`/`cleanup` 清理；若盒子在测试中途断电，可能残留带 `__test_context_only` 标记的行，可用 `cleanup` 按前缀兜底清理。
