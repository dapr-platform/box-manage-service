# 前端对接说明：AI 快速编排与编排测试

- 编写日期：2026-10-14
- 面向对象：`front-hub-ng/apps/box-manage-web` 前端开发
- 后端仓库：`box-manage-service`（已实现并通过测试）
- 关联文档：
  - `ai_docs/AI全量编排生成改造方案.md`（AI 快速编排完整背景）
  - `ai_docs/编排测试方案-前端仿真测试.md`（用例格式、断言、面板设计）
  - `ai_docs/编排测试方案-真机测试.md`（真机测试完整背景）
  - `docs/ai_workflow_generation_api.md`（AI 编排接口详情）
  - `docs/编排测试-前端仿真测试说明.md`（仿真器语义与已知差异）
  - `docs/box-app演练模式与任务上下文改动说明.md`（盒子端改动）

> 本文档的**接口字段、错误码、响应结构均直接取自后端源码**（`api/routes.go`、各 controller、`models/workflow_test_case.go`、`service/workflow_device_test_service.go`），非推测。

> ⚠️ **本文档描述的功能前端已经实现完毕**（见 §7 文件清单），因此本文档有双重用途：
> 1. 作为**对接契约**，说明接口怎么用、有哪些坑；
> 2. 作为**验收与回归清单**，特别是 §5 的真机测试部分——盒子端改动尚未编译验证，联调时请按 §5.4 逐项核对。

---

## 一、总览

本轮为编排页新增三大块能力，共 **11 个路由**（9 个不同路径；`/workflow-test-cases/{caseId}` 同时提供 PUT 与 DELETE）：

| 能力 | 路由数 | 前缀 | 状态 |
|---|---|---|---|
| AI 快速编排 | 3 | `/api/v1/workflows/*` | ✅ 可直接联调 |
| 编排测试用例 | 5 | `/api/v1/workflows/{id}/test-cases`、`/api/v1/workflow-test-cases/*` | ✅ 可直接联调 |
| 真机测试 | 3 | `/api/v1/workflow-tests/*` | ⚠️ **依赖盒子端发版**，见 §5 |

所有接口都会自动带 `BASE_CONTEXT` 前缀。前端统一走 `boxManageApi.request({ url: '/box-manage-service/api/v1/...' })`（与现有 `services/workflow.ts` 一致）。

### 统一响应约定

**成功**：

```json
{ "code": 0, "message": "操作成功", "data": { }, "timestamp": "2026-10-14 10:00:00" }
```

前端取数据一律用 `response.data?.data || response.data`（既有代码的写法）。

**失败**：

```json
{ "code": 400, "message": "参数错误", "error": "详细错误信息", "timestamp": "2026-10-14 10:00:00" }
```

HTTP 状态码与 `code` 一致。**展示错误时优先用 `error` 字段**，它包含具体原因（`message` 只是概括）。

---

## 二、通用概念

### 2.1 端口默认值字段：统一用 `default_value`

节点的输入/输出参数里，默认值字段是 **`default_value`**（不是 `default`）。

| 写法 | 状态 |
|---|---|
| `default_value` | ✅ **规范字段**，写入时只能用它 |
| `default` | ⚠️ 仅作兼容读入（历史画布数据），不要写入 |

原因：后端 `variable_definitions.default_value` 列、盒子端 `resolveInputs` 的取值都以此为准。

前端已有统一工具 `src/utils/workflow-port.ts`：

```ts
import { readPortDefault, normalizeNodePorts } from '@/utils/workflow-port'

readPortDefault(port)        // 读取：default_value 优先，回退 default
normalizeNodePorts(props)    // 把 {inputs, outputs} 里的 default 迁移到 default_value
```

**两个必须注意的点**：

1. **`false` 和 `0` 是合法默认值**。用 `!== undefined` 判断，不要用真值判断，否则布尔/数字默认值会被丢掉。
2. 加载历史编排（`structure_json_view`）时**必须调 `normalizeNodePorts`**。否则旧数据里的 `default` 在配置面板（绑定 `default_value`）显示为空，用户一保存就**永久丢失**。

### 2.2 统一校验器

后端有一套统一校验器（结构、节点模板、连通性、引用、必填项、Python 静态检查），用于三处：AI 生成结果、`/normalize` 与 `/validate` 接口、发布门禁。

**校验报告结构**：

```ts
interface ValidationReport {
  valid: boolean
  errors: ValidationIssue[]
  warnings: ValidationIssue[]
}

interface ValidationIssue {
  code: string        // 机器可读的错误码，如 "reference_not_upstream"
  message: string     // 中文描述，可直接展示
  node_id?: string    // 有定位信息时可用来高亮/选中节点
  edge_id?: string
}
```

**前端应做的**：错误列表点击时用 `node_id` 选中画布节点（`lf.selectElementById(nodeId, true)`）。

#### 校验器**不会**报错的写法（照抄 box-app 的真实语义，别当 bug）

这几条都是合法写法。校验器曾因为误判它们而产生大量假错误、导致修复轮次无法收敛，现已修正并有回归测试锁定：

| 写法 | 为什么合法 |
|---|---|
| `ref_key_name: "detections"`（**不带点号**） | 无点号 = 引用**上下文变量**名。box-app 的 `resolveReference` 会先按变量名查找再找节点输出。开始节点用它接收推理上下文（`detections`、`image_base64`）、`task_get_context` 用它接收上下文键（`is_alarmed` 等），都是标准用法 |
| `outputs` 上带 `ref_key_name` | box-app 只把 `node.inputs` 交给 `resolveInputs`；输出是按变量名捕获的，outputs 上的 `ref_key_name` 不参与解析 |
| `python_script` 端口用自定义 key_name | Python 节点的入参/出参由用户自由定义。模板里的 `input`/`output` 只是占位示例 |
| `script_content` 里出现 `{...}` | `script_content` **不参与编排插值**。Python f-string（`f"已达{count}帧"`）必须原样放行 |
| `{node.key}` 写在非插值字段里 | 插值只在 4 类节点的特定字段生效，见下表 |

**会发生 `{node.key}` 插值的字段**（只有这些，其余字段里写 `{}` 只是普通文本）：

| 节点 | 字段 |
|---|---|
| `wechat_work` | `body`（递归所有字符串） |
| `mqtt` | `payload` |
| `http_request` | `url`、`headers`、`body`（body 为字符串时） |
| `rpa_wechat_proxy` | `body_text` |

> ⚠️ 联动注意：**Python 节点的 inputs 不会自动接收 `input_params` 之外的东西**。如果用 `python_script` 读上游输出，必须显式写 `ref_key_name`；而脚本里直接用变量名（如 `consecutive_vehicle_count`）是 box-app 注入的同名顶层变量，与编排插值无关。

### 2.3 保存 vs 发布

| 操作 | 校验强度 | 说明 |
|---|---|---|
| 保存草稿 | **不阻断** | 结果只写后端日志。草稿允许是半成品（如只有开始/结束节点） |
| 发布 | **强门禁** | 校验不通过返回 **400 + 错误明细**，且要求**全图连通** |

**发布失败时前端必须展示明细**，否则用户只看到「status code 400」。响应结构：

```json
{
  "code": 400,
  "message": "编排校验未通过，无法发布",
  "error": "编排必须包含一个开始节点；节点 python_script-005 的必填输入 \"detections\" 既没有引用上游也没有默认值",
  "errors": [
    { "code": "missing_start", "message": "编排必须包含一个开始节点" },
    { "code": "required_input_missing", "message": "...", "node_id": "python_script-005" }
  ]
}
```

> 注意：发布接口的 `errors` 字段**只在发布失败时出现**（`ValidationErrorResponse`），其余接口是普通的 `ErrorResponse`（无 `errors`）。

---

## 三、AI 快速编排（3 个路由）

### 3.1 单轮生成

```http
POST /api/v1/workflows/ai-generate
```

**请求**：

```ts
interface WorkflowAIGenerateRequest {
  template_workflow_id?: number          // 参考模板编排 ID，可不传
  description: string                    // 业务需求，必填
  clarifications?: Array<{               // 用户对上一轮反问的回答
    question: string
    answer: string
  }>
  previous?: {                           // 仅修复轮次携带
    raw: string                          // 上一轮返回的 raw，原样回传
    errors: string[]                     // 上一轮的校验错误 message 列表
  }
}
```

**响应**：

```ts
interface WorkflowAIGenerateResult {
  status: 'generated' | 'need_clarification'
  name?: string                          // 模型建议的编排名称
  graph?: any                            // 完整 LogicFlow JSON，可直接渲染画布
  raw?: string                           // 模型返回的精简 JSON，修复轮次要回传
  report?: ValidationReport
  summary?: string                       // 业务逻辑说明（展示给用户核对）
  assumptions?: string[]                 // 模型自行采用的假设（必须提示用户核对）
  questions?: Array<{
    id: string
    question: string
    options?: string[]
    multiple?: boolean
    default?: string
  }>
  model?: string
  template_name?: string
}
```

**关键约定**：

| 约定 | 说明 |
|---|---|
| **不落库** | 接口只生成，不写数据库。渲染到画布后仍是未保存状态，用户确认后走现有保存流程 |
| **`graph` 可直接渲染** | 已是补全后的完整 LogicFlow JSON（含坐标、样式、`node_template_id`） |
| **`raw` 必须回传** | 修复轮次要把它原样放进 `previous.raw`，不要自己改格式 |
| **多轮循环由前端驱动** | 每次请求只调一次大模型。不要指望后端自动重试（会超时） |
| **超时** | 后端 `LLM_REQUEST_TIMEOUT` 默认 **180s**；前端该请求超时必须**严格大于它**（当前 300s）。修复轮次的提示词含上一轮完整 JSON，实测后端需要 60s 以上，两个值都别再调小 |

**错误码**：

| HTTP | 场景 |
|---|---|
| 400 | `description` 为空、请求体格式错误、提示词超长、参考模板不存在 |
| 502 | 模型返回内容不可用 |
| 503 | 后端未配置 LLM key / LLM 不可达（**现场常见**） |

**`need_clarification` 的两种处理**：
- 只给用户「按默认处理」（用每个 `question.default`）
- 或让用户填写，把回答放进 `clarifications` 重新调用

### 3.2 补全（导入）

```http
POST /api/v1/workflows/normalize
{ "graph": { "nodes": [...], "edges": [...] } }
```

`graph` 可以是完整 LogicFlow JSON、精简 JSON，甚至后端 `nodes/lines` 结构。服务会去视图字段、补 `node_template_id` 与坐标（自动布局）、补连线样式，再校验。

**响应**：`{ graph: <补全后的JSON>, report: ValidationReport }`

**用途**：编辑器的「导入」按钮应改为走这个接口，而不是直接 `JSON.parse` 后渲染——这样错误能在导入时就暴露，而不是等到保存或运行时。

### 3.3 校验

```http
POST /api/v1/workflows/validate
{ "graph": { ... } }
```

**响应**：`ValidationReport`（直接是 `data`，没有再包一层）。

**用途**：工具栏「校验」按钮。前端不必再自己写校验规则。

### 3.4 前端多轮循环的停止条件

这块逻辑较绕，已实现在 `src/composables/useWorkflowAIGenerate.ts`，供核对：

```
调用 ai-generate
  ├─ need_clarification ──► 切问题视图，用户回答后带上 clarifications 重调
  ├─ generated 且校验通过 ──► 跑仿真测试（若有用例）──► 通过则导入画布，结束
  └─ generated 但校验有错误
        ├─ 有失败用例 ──► 把测试失败信息也当错误回传，进入下一轮修复
        └─ 自动发起修复轮
             停止条件（满足任一即暂停，交给用户决定）：
               · 达到最大轮次（默认 5）
               · 连续 2 轮错误数没有减少（无进展）
               · 用户点「停止」
               · 反问超过 2 次
             暂停后提供：[继续修复] [补充说明后重新生成] [导入当前结果]
```

**反问限制**：每轮最多 5 个问题，每个都带 `default`，支持「按默认处理」；总反问不超过 2 次。

**修复过程中用例只读**：模型只能改编排，不能改用例——否则它会通过改用例让测试「通过」。

---

## 四、编排测试用例（5 个路由）

### 4.1 用例数据结构

用例按 **`workflow_key_name`** 关联（不是自增 ID），所以**编排升级版本后用例仍然可用**，作为回归测试。

```ts
interface WorkflowTestCase {
  id?: number
  workflow_key_name?: string   // 后端自动填，前端不用传
  name: string                 // 必填
  description?: string
  content: string              // 用例 JSON 的字符串形式
  created_by?: number
  updated_by?: number
  created_at?: string
  updated_at?: string
}
```

**`content` 的结构**（存成字符串）：

```ts
interface TestCaseContent {
  name?: string
  description?: string
  initial_context?: Record<string, any>   // 第一帧之前的任务上下文
  frames: TestCaseFrame[]
}

interface TestCaseFrame {
  label?: string          // 帧标签，会出现在失败信息里
  repeat?: number         // 同样输入重复 N 帧，默认 1
  expect_each?: boolean   // true 时每帧都判断断言；默认只在最后一帧判断
  inputs?: Record<string, any>   // 该帧开始节点的输入
  mocks?: Record<string, any>    // 仅仿真使用：为不能仿真的节点提供输出
  expect?: TestExpectation[]
}

interface TestExpectation {
  path?: string    // "<节点id>.<输出key>"，可继续用 . 访问嵌套字段
  op?: 'eq' | 'ne' | 'gt' | 'gte' | 'lt' | 'lte' | 'contains' | 'exists' | 'not_exists'
  value?: any
  node?: string    // 判断某节点本帧是否执行时用
  executed?: boolean
}
```

**用例示例**：

```json
{
  "name": "持续有人触发一次报警",
  "description": "覆盖进入、首次报警、人数增加重报、恢复正常",
  "initial_context": {},
  "frames": [
    {
      "label": "有人 1~3 帧，未进入入侵状态",
      "repeat": 3,
      "inputs": { "detections": [{ "class_id": 0, "confidence": 0.92 }], "image_base64": "" },
      "expect": [
        { "path": "python_script-005.is_intrusion_detected", "op": "eq", "value": false },
        { "node": "wechat_work-007", "executed": false }
      ]
    },
    {
      "label": "继续有人 10 帧，最后一帧首次报警",
      "repeat": 10,
      "inputs": { "detections": [{ "class_id": 0, "confidence": 0.92 }] },
      "expect": [
        { "path": "python_script-005.send_alarm", "op": "eq", "value": true },
        { "node": "wechat_work-007", "executed": true }
      ]
    }
  ]
}
```

**隐含断言**：每一帧的执行必须成功，任何节点失败都直接判定该帧失败。

### 4.2 接口清单

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/workflows/{id}/test-cases` | 查询该编排的全部用例（`{id}` 传编排 ID） |
| `POST` | `/api/v1/workflows/{id}/test-cases` | 新建用例 |
| `PUT` | `/api/v1/workflow-test-cases/{caseId}` | 修改用例 |
| `DELETE` | `/api/v1/workflow-test-cases/{caseId}` | 删除用例 |
| `POST` | `/api/v1/workflows/{id}/test-cases/ai-generate` | AI 生成用例（只返回草稿，不保存） |

**注意路由分组**：新建/查询挂在编排下（需要 `{id}` 定位 `key_name`），修改/删除挂在 `workflow-test-cases` 下（只需 `caseId`）。别拼错。

**保存用例的前置条件**：编排**必须先保存**（用例按 `key_name` 关联，未保存的编排没有 `key_name`）。用 `workflowId` 是否存在来判断，不存在时提示用户先保存。

### 4.3 AI 生成用例

```http
POST /api/v1/workflows/{id}/test-cases/ai-generate
{ "description": "业务需求原文", "assumptions": ["..."] }
```

`assumptions` 可选，传 AI 编排返回的 `assumptions` 可作为阈值参考。

**响应**：

```ts
{
  name: string
  description?: string
  content: TestCaseContent     // 注意：这里直接是对象，不是字符串
  warnings?: string[]
  model?: string
}
```

**⚠️ 关键差异**：AI 生成返回的 `content` 是**对象**；而保存用例时 `WorkflowTestCase.content` 是**字符串**。前端需要 `JSON.stringify(draft.content, null, 2)` 转换后再放进编辑器。这条已实现在 `WorkflowAIQuickModal` → `handleAIGenerateConfirm`。

**生成后必须让用户检查再保存**，不要自动保存。`warnings` 里会提示「预期值可能与实际业务约定不一致，请先人工核对」。

**超时**：与 AI 编排同理，前端 300s（必须大于后端 `LLM_REQUEST_TIMEOUT` 的 180s）。

### 4.4 AI 生成用例的设计原则（重要）

**用例从需求出发，而不是从编排出发。**

后端提示词只把**节点与输出的名称**告诉模型（用于拼断言路径），**不给脚本内容**。否则模型会照着实现写期望值，测试退化成「证明代码等于代码」。

这条有单测锁定（`TestBuildNodeOutlineOnlyExposesNamesNotScripts`）。前端不需要做什么，但**不要在界面上诱导用户「照着实现在用例里写期望」**。

---

## 五、真机测试（3 个路由）

> ⚠️ **本节依赖盒子端（box-app）改动**，改动已完成但**尚未编译验证与联调**。详见 `docs/box-app演练模式与任务上下文改动说明.md`。
> 盒子发版前，前端应允许用户看到该模式但给出明确提示（当前实现会在真机模式下显示副作用节点提示）。

### 5.1 与仿真的关系

| | 前端仿真 | 真机测试 |
|---|---|---|
| 执行位置 | 浏览器（TS + Pyodide） | 测试盒子上的 box-app |
| 速度 | 每帧毫秒级 | 每帧约 1 秒 |
| 依赖 | 无 | 在线盒子 + 盒子固件支持 |
| 结果地位 | 开发参考 | **以此为准** |

**两者产出完全相同的帧结果结构**，前端用**同一份断言函数**判断（`src/utils/workflow-test-assert.ts`）。

### 5.2 启动运行

```http
POST /api/v1/workflow-tests/run
```

```ts
interface WorkflowDeviceTestRunRequest {
  workflow_id: number
  box_id: number
  case_ids?: number[]     // 不传则跑全部用例
  dry_run?: boolean       // 默认 true，强烈建议保持 true
}
```

**响应**（立即返回，后台执行）：

```ts
interface DeviceTestRun {
  run_id: string
  workflow_id: number
  box_id: number
  status: 'running' | 'finished' | 'stopped' | 'error'
  dry_run: boolean
  progress: {
    case_index: number
    frame_index: number
    total_frames: number
  }
  cases: Array<{
    case_id: number
    name: string
    frames: DeviceFrameResult[]
    error?: string
  }>
  error?: string
  started_at: string
  finished_at?: string
}
```

**`DeviceFrameResult`**（与仿真的帧结果结构一致）：

```ts
interface DeviceFrameResult {
  index: number
  label?: string
  status: 'success' | 'failed'
  executed_nodes: string[]
  node_outputs: Record<string, Record<string, any>>
  node_inputs: Record<string, Record<string, any>>
  errors: string[]
}
```

### 5.3 查询与停止

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/workflow-tests/runs/{runId}` | 查询进度与结果（**前端轮询，建议 1 秒一次**） |
| `POST` | `/api/v1/workflow-tests/runs/{runId}/stop` | 停止运行（会触发清理） |

**轮询建议**：每 1 秒一次；整体超时给 10 分钟。`status !== 'running'` 时停止轮询。

**注意**：`GetRun` 返回的 `cases[].frames` 是**累积**的，每轮拿到的可能是增量结果。前端只需在 `status` 结束时用最终结果做断言。

### 5.4 联调必查项 ⚠️

盒子端改动未验证，以下**五条必须实测**（来自 box-app 改动说明）：

| # | 检查项 | 期望 |
|---|---|---|
| 1 | 两个新接口是否活着 | `POST /api/v1/task-contexts/cleanup {"prefix":"wf_test_"}` → `{"success":true,"affected":0}` |
| 2 | **演练模式确实不发消息** | `dry_run:true` 触发后，企微节点输出含 `dry_run:true`，且**目标群没收到消息** |
| 3 | 非演练模式回归 | 去掉 `dry_run` 后消息正常发出，行为与改动前一致 |
| 4 | **上下文跨帧写入** | 含 `task_get/set_context` 的编排能正确累加状态（原实现会**静默失败**） |
| 5 | 清理不误伤 | `cleanup` 后真实任务上下文未被清掉 |

**第 2 和第 4 条最关键**：第 2 条不做的话测试会真实发消息污染群；第 4 条不成立的话跨帧用例全是错的。

### 5.5 一个尚未核对的风险

后端解析盒子节点结果时按 `node_id` / `output_data` / `input_data` / `status` 取值（`service/workflow_device_test_service.go` 的 `runOneFrame`）。这些字段名是**从 box-app 源码读出来的，未经真实响应验证**。

联调时请贴一份 `GET /api/v1/workflow-instances/{id}/nodes` 的**原始响应**给后端同学核对。字段对不上的话，`node_outputs` 会是空的，断言全部失败。

### 5.6 仿真与真机的一致性比对

同一用例在两边都跑过后，应比对每帧的节点输出。不一致的地方要么是仿真器写错了，要么是 box-app 的行为没被正确理解——**这是校准仿真器的唯一依据**（方案第 7 节）。

前端可在 `TestRunResult.vue` 展示比对结果，不一致的节点输出标红。**当前实现尚未做这块比对**，属于可选增强。

---

## 六、已知坑汇总

| # | 坑 | 后果 | 规避 |
|---|---|---|---|
| 1 | 写入 `default` 而不是 `default_value` | 默认值丢失 | 只用 `default_value`；加载旧数据时调 `normalizeNodePorts` |
| 2 | 用真值判断默认值 | `false`/`0` 被丢弃 | 用 `!== undefined` |
| 3 | AI 生成的 `content` 是对象不是字符串 | 直接存会失败 | `JSON.stringify` 后再给编辑器 |
| 4 | 未保存编排就调用例接口 | 后端报找不到 `key_name` | 用 `workflowId` 判断并提示先保存 |
| 5 | 前端超时小于后端 `LLM_REQUEST_TIMEOUT` | 前端先超时，看不到后端的真实错误 | 前端必须**严格大于**后端（当前 300s > 180s） |
| 6 | 真机测试不传 `dry_run`（或传 false） | **真实发送企微/MQTT 消息** | 默认 `true`，界面上提示风险 |
| 7 | 发布失败只显示「400」 | 用户无法定位问题 | 读 `error` 或 `errors[]` 展示明细 |
| 8 | 真机模式传 `mocks` | 盒子会忽略，用例预期落空 | 界面上提示「真机忽略 mocks」 |
| 9 | 盒子返回节点字段名与预期不符 | `node_outputs` 为空、断言全挂 | 联调时核对（见 §5.5） |

---

## 七、前端文件清单

以下文件**均已实现**，供核对与维护：

### services

| 文件 | 职责 |
|---|---|
| `src/services/workflow-ai.ts` | AI 生成、补全（导入）、校验 |
| `src/services/workflow-test.ts` | 用例增删改查、AI 生成用例、真机测试运行 |

### composables

| 文件 | 职责 |
|---|---|
| `src/composables/useWorkflowAIGenerate.ts` | 多轮生成循环、停止条件、自动测试闭环 |
| `src/composables/useWorkflowTestRun.ts` | 测试运行入口（仿真/真机分流）、状态管理 |

### simulator（仿真器）

| 文件 | 职责 |
|---|---|
| `src/simulator/resolve.ts` | 输入解析、引用解析、模板插值、条件求值 |
| `src/simulator/nodes.ts` | 各节点类型实现 + 支持范围白名单 |
| `src/simulator/engine.ts` | 遍历：单指针推进、条件分支、并发、循环、失败中断 |
| `src/simulator/runner.ts` | 用例 + 引擎 + 断言串联，生成给大模型的失败信息 |
| `src/simulator/python-runtime.ts` | Pyodide Worker 封装（懒加载、超时、异常重建） |
| `src/simulator/python.worker.ts` | Worker 内运行 Pyodide |
| `scripts/sync-pyodide.mjs` | 同步 Pyodide 运行时到 `public/pyodide/` |

### utils / 组件

| 文件 | 职责 |
|---|---|
| `src/utils/workflow-port.ts` | 端口默认值统一读写 |
| `src/utils/workflow-test-assert.ts` | 断言（纯函数，仿真与真机共用） |
| `src/pages/workflow/components/WorkflowAIQuickModal.vue` | AI 快速编排弹窗 |
| `src/pages/workflow/components/test/WorkflowTestDrawer.vue` | 测试抽屉 |
| `src/pages/workflow/components/test/TestCaseEditor.vue` | 用例编辑 |
| `src/pages/workflow/components/test/TestRunResult.vue` | 结果展示 |

### 改动文件

| 文件 | 改动 |
|---|---|
| `src/pages/workflow/WorkflowList.vue` | 新增「AI 快速编排」按钮；发布失败展示校验明细 |
| `src/pages/workflow/components/WorkflowEditorDialog.vue` | 接入 AI 弹窗与测试抽屉；导入改走 normalize；校验改走后端；画布高亮 |
| `src/pages/workflow/editor/EditorToolbar.vue` | 新增「AI 编排」「测试」按钮 |
| `src/pages/workflow/editor/NodeConfigPanel.vue` | 默认值字段改用 `default_value` |
| `src/pages/workflow/editor/WorkflowCanvas.vue` | 模板变量转换用 `default_value`；撤销状态改监听 `history:change` |
| `src/pages/workflow/editor/custom-nodes/BaseNode.ts`、`nodes.ts` | 测试结果的高亮渲染 |
| `src/utils/workflow-transform.ts` | 用 `readPortDefault` 统一读取 |
| `src/types/workflow.ts` | `default_value` 标注为规范字段 |

---

## 八、本地运行与自测

```bash
# 前端：仿真器语义与遍历单测（37 个用例，覆盖校验器/条件求值/遍历等高风险语义）
cd front-hub-ng/apps/box-manage-web && npx vitest run

# 前端构建
pnpm run build

# 后端
cd box-manage-service && go test -vet=off ./...
```

**Pyodide 资源**：约 13MB，`pnpm run sync-pyodide` 会从 `node_modules` 复制到 `public/pyodide/`（已挂 `prebuild`，该目录在 `.gitignore` 中）。**部署流水线若跳过 `pnpm install` 或未执行同步，首次打开测试面板会报「运行时未就绪」**。

**Swagger**：`{BASE_CONTEXT}/swagger/index.html`，本轮所有接口已收录。

---

## 九、对接顺序建议

1. **AI 快速编排**（§3）——不依赖盒子，可立即用真实 LLM 联调。先跑通「选模板 + 输入需求 → 生成 → 导入画布 → 保存」。
2. **编排测试用例 + 前端仿真**（§4）——不依赖盒子。用 AI 生成几个用例，跑仿真，确认断言和画布高亮正常。
3. **真机测试**（§5）——**等盒子端编译发版后**再开始，先按 §5.4 手工验接口，再走前端全链路。
