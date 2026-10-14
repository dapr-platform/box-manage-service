# AI 快速编排接口说明

## 功能说明

AI 快速编排让用户**选一个已有编排当样板 + 用一句话说清要做什么**，由大模型生成整张可直接导入画布的编排。

系统把以下四类内容一起发给大模型：

1. **基础节点说明**：由启用中的节点模板表动态生成（`type`、名称、分组、固定输入/输出、配置项）。
2. **编排规则**：固定文本，说明变量如何定义与引用、条件如何写、跨帧上下文如何存取、Python 脚本约定。
3. **参考模板编排**：用户所选编排的 `structure_json_view`，去掉视图字段后作为 few-shot 示例。
4. **业务需求**：用户输入；修复轮次还会带上上一轮结果与错误列表。

大模型只负责业务内容；**坐标、连线样式、`node_template_id`、节点文本由系统自动补全**，模型不需要也不应该生成这些字段。

生成结果**不自动落库**：渲染到画布后仍是未保存状态，用户可以撤销、微调，确认后走现有的「保存」流程。

三个接口都是无副作用的计算型接口，不写数据库、不修改已有编排。

## 接口一览

| 接口 | 用途 |
| --- | --- |
| `POST /api/v1/workflows/ai-generate` | 单轮生成：返回反问问题 或 可导入的编排 JSON + 校验报告 |
| `POST /api/v1/workflows/normalize` | 补全 + 校验：用于「导入」按钮，也接受手工编写或外部大模型生成的精简 JSON |
| `POST /api/v1/workflows/validate` | 只校验，返回带节点/连线定位的报告 |

如果配置了 `BASE_CONTEXT`，需要把它加在路径前面。例如 `BASE_CONTEXT=/api/box-manage-service` 时：

```text
POST /api/box-manage-service/api/v1/workflows/ai-generate
```

## 1. 单轮生成

```text
POST /api/v1/workflows/ai-generate
```

### 请求参数

```json
{
  "template_workflow_id": 12,
  "description": "检测到车辆在禁停区停留超过 30 帧就推送企业微信文字和图片，车辆离开 20 帧后解除报警",
  "clarifications": [
    { "question": "车辆对应的检测类别 ID 是多少？", "answer": "2" }
  ],
  "previous": {
    "raw": "{\"status\":\"generated\",\"nodes\":[]}",
    "errors": ["节点 python_script-005 声明输出 send_alarm 在脚本中没有被赋值"]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `template_workflow_id` | `uint` | 否 | 参考模板编排 ID。不传也能生成，但缺少写法示例，效果会差一些 |
| `description` | `string` | 是 | 业务需求，不能为空 |
| `clarifications` | `array` | 否 | 用户对上一轮反问的回答，格式为「问题 → 回答」 |
| `previous` | `object` | 否 | 仅修复轮次携带：`raw` 是上一轮模型返回的精简 JSON，`errors` 是校验错误列表 |

> `previous` 只带**上一轮**，不带全部历史，避免提示词无限增长。

### 响应：生成成功

```json
{
  "code": 0,
  "message": "AI生成编排成功",
  "data": {
    "status": "generated",
    "name": "车辆违停累计报警",
    "graph": {
      "nodes": [
        {
          "id": "start-001",
          "type": "start",
          "x": 120,
          "y": 100,
          "text": { "value": "开始", "x": 120, "y": 100 },
          "properties": {
            "key_name": "start-001",
            "name": "开始",
            "type": "start",
            "node_template_id": 1,
            "config": {},
            "inputs": [
              { "key_name": "detections", "name": "检测结果", "type": "array" }
            ],
            "outputs": []
          }
        }
      ],
      "edges": [
        {
          "id": "line-001",
          "type": "polyline",
          "sourceNodeId": "start-001",
          "targetNodeId": "detection_filter-002",
          "properties": {
            "condition_type": "none",
            "style": { "stroke": "#666", "strokeWidth": 1 }
          }
        }
      ]
    },
    "raw": "{\"status\":\"generated\",\"name\":\"车辆违停累计报警\",\"nodes\":[...],\"edges\":[...]}",
    "report": {
      "valid": true,
      "errors": [],
      "warnings": []
    },
    "summary": "1. 过滤出目标类别；2. 累计连续帧数，超过 30 帧触发报警；3. 车辆离开 20 帧后解除报警。",
    "assumptions": ["需求未说明停留帧数阈值，按 30 帧处理", "需求未说明解除帧数，按 20 帧处理"],
    "model": "deepseek-v4-flash",
    "template_name": "人员入侵累计检测编排-2669"
  },
  "timestamp": "2026-10-13 23:20:00"
}
```

`graph` 就是完整的 LogicFlow JSON，可以**直接渲染到当前画布**（与「导入」走同一条渲染路径）。

| 字段 | 说明 |
| --- | --- |
| `status` | `generated`（已生成）或 `need_clarification`（需要反问） |
| `name` | 模型建议的编排名称；编排名称为空时前端可用它预填 |
| `graph` | 补全后的完整 LogicFlow JSON，可直接导入 |
| `raw` | 模型返回的精简 JSON 字符串，**修复轮次需要原样回传** |
| `report` | 统一校验器的结果，`errors` 为空表示通过 |
| `summary` | 业务逻辑说明，展示给用户核对 |
| `assumptions` | 模型自行采用的假设（例如未说明的阈值），需提示用户核对 |

### 响应：需要反问

```json
{
  "code": 0,
  "message": "AI需要确认以下信息",
  "data": {
    "status": "need_clarification",
    "questions": [
      {
        "id": "q1",
        "question": "车辆对应的检测类别 ID 是多少？",
        "options": ["2", "自定义"],
        "multiple": false,
        "default": "2"
      }
    ],
    "raw": "{\"status\":\"need_clarification\",...}",
    "model": "deepseek-v4-flash"
  }
}
```

用户回答后，把这些回答放进下一次请求的 `clarifications` 重新调用即可。

### 响应：模型返回非法 JSON

`status` 仍为 `generated`，但 `graph` 为 `null`、`report.errors` 里是解析错误。前端据此发起修复轮次，后端不单独重试。

### 错误响应

| HTTP 状态码 | 场景 |
| --- | --- |
| `400` | JSON 格式错误、`description` 为空、提示词超过 `LLM_MAX_REQUEST_BYTES`、参考模板不存在 |
| `502` | 模型返回空内容 |
| `503` | 后端未配置 `LLM_API_KEY`、LLM 地址不可用或请求超时 |

错误格式：

```json
{
  "code": 503,
  "message": "AI生成编排失败",
  "error": "LLM API key is not configured",
  "timestamp": "2026-10-13 23:20:00"
}
```

## 2. 补全编排（导入）

```text
POST /api/v1/workflows/normalize
```

请求体：

```json
{
  "graph": { "nodes": [], "edges": [] }
}
```

`graph` 可以是**完整 LogicFlow JSON**，也可以是**精简 JSON**（只有 `id`/`type`/`properties` 与连线的 `id`/`sourceNodeId`/`targetNodeId`/`properties`），甚至是后端的 `nodes`/`lines`/`source_node_id` 结构。服务会：

1. 删除视图字段（`x`、`y`、宽高、锚点、折线点、`style`）。
2. 按 `type` 查节点模板，补 `node_template_id`、节点文本。
3. 按拓扑分层自动布局：`x = 120 + 层号 × 200`，同层 `y` 间隔 120。
4. 补连线 `type = "polyline"`，条件线橙色 `#fa8c16`/2px，普通线 `#666`/1px。
5. 用统一校验器校验并返回报告。

响应：

```json
{
  "code": 0,
  "message": "补全编排成功",
  "data": {
    "graph": { "nodes": [], "edges": [] },
    "report": { "valid": true, "errors": [], "warnings": [] }
  }
}
```

## 3. 校验编排

```text
POST /api/v1/workflows/validate
```

请求体同样为 `{ "graph": ... }`，响应 `data` 是校验报告：

```json
{
  "code": 0,
  "message": "校验编排成功",
  "data": {
    "valid": false,
    "errors": [
      {
        "code": "reference_not_upstream",
        "message": "节点 end-003 的参数引用了非上游节点 \"python_script-005\"",
        "node_id": "end-003"
      }
    ],
    "warnings": [
      {
        "code": "reference_type_mismatch",
        "message": "节点 wechat_work-007 的输入 \"body_text\" 类型为 json，但引用的 python_script-005 输出类型为 string",
        "node_id": "wechat_work-007"
      }
    ]
  }
}
```

每条结果都带 `node_id` / `edge_id`，前端可据此在画布上高亮或选中对应元素。

## 统一校验器规则

| 类别 | 规则 | 级别 |
| --- | --- | --- |
| 结构 | 恰好 1 个开始节点、至少 1 个结束节点；节点 ID 唯一；连线端点存在；无自环 | error |
| 连通性 | 所有节点可从开始节点到达；所有节点能到达结束节点 | error |
| 节点模板 | `type` 对应启用中的模板；`node_template_id` 与 `type` 一致；固定参数节点的 `key_name` 不超出模板定义 | error |
| 成对节点 | 并发/循环的开始与结束节点必须成对，且结束节点在开始节点下游 | error |
| 引用 | `ref_key_name = <节点id>.<key>`：节点存在、是当前节点的上游、`key` 在该节点的 outputs 中（开始节点取 inputs） | error |
| 插值 | 字符串与条件表达式中的 `{<节点id>.<key>}` 满足同样的上游规则 | error |
| 必填 | `required = true` 的输入必须有 `ref_key_name` 或非空 `default` | error |
| 类型 | 引用两端类型不一致（`json`/`object`/`array` 之间视为同族，不报） | warning |
| Python | 复用 `validatePythonScript` 检查运行契约；脚本中未给某个声明的输出赋值（正则检查 `^\s*key\s*=`） | error / warning |

结构类错误（缺开始节点、ID 重复、连线端点缺失、自环）会**短路**后续的引用与连通性校验，避免一次返回几十条噪音。

## 生成流程与自动修复

**循环由前端驱动，每次 HTTP 请求只调用一次大模型**。这样每轮状态都能实时显示在界面上，单次请求也不会因为多轮重试而超时。

```
开始生成
  │
  ▼
调用 ai-generate ──► 返回 status
  │
  ├─ need_clarification ──► 弹窗切为「问题」视图，用户回答后带上 clarifications 重新调用
  │
  ├─ generated 且校验通过 ──► 导入画布，结束
  │
  └─ generated 但校验有错误
        自动发起修复轮：带上上一轮 raw + 错误列表重新调用
        界面显示「第 3 轮修复中，上一轮剩余 2 个错误」
        满足任一条件时暂停自动修复：
          · 达到最大轮次（默认 5 轮）
          · 连续 2 轮错误数没有减少（无进展）
          · 用户点击「停止」
        暂停后用户可选择：
          [继续修复] [补充说明后重新生成] [导入当前结果]
```

反问的限制：每轮最多 5 个问题，每个问题都带建议默认值并支持「按默认处理」；总的反问不超过 2 次。

## 前端对接要点

1. **入口**：编排列表页「新建编排」旁的「AI 快速编排」按钮，以及编辑器工具栏的「AI 编排」按钮，打开同一个弹窗。
2. **渲染**：AI 生成结果、文件导入、后端补全结果共用同一个渲染函数，行为一致。
3. **覆盖确认**：画布上已有用户内容时先确认「将覆盖当前画布」；只有默认的开始/结束节点时直接替换。
4. **名称预填**：编排名称为空时用 `data.name` 预填。
5. **保存**：结果不自动保存，用户确认后走现有保存流程。
6. **超时**：单轮生成可能耗时较久，前端对该请求单独放宽到 180 秒（默认请求超时是 60 秒，与大模型自身超时相同，容易先超时）。
7. **不落库**：三个接口都不会修改已有编排；需要保留 JSON 文件时用编辑器现有的「导出」按钮。

## 保存与发布时的校验

- **保存草稿**：统一校验器的结果只写日志，**不阻断保存**。草稿允许是半成品（例如刚放下开始/结束节点还没连线），硬性阻断会让用户无法保存。原有的定义表校验（必须有开始/结束节点、模板存在、连线端点存在）保持不变。
- **发布**：**强门禁**。统一校验器报错即拒绝发布，接口返回 `400` 与错误明细：

```json
{
  "code": 400,
  "message": "编排校验未通过，无法发布",
  "error": "编排必须包含一个开始节点；节点 python_script-005 的必填输入 \"detections\" 既没有引用上游也没有默认值",
  "errors": [
    { "code": "missing_start", "message": "编排必须包含一个开始节点" },
    { "code": "required_input_missing", "message": "节点 python_script-005 的必填输入 \"detections\" 既没有引用上游也没有默认值", "node_id": "python_script-005" }
  ],
  "timestamp": "2026-10-14 10:00:00"
}
```

发布校验会额外要求**全图连通**：所有节点都能从开始节点到达，并且都能到达结束节点。

前端「发布」按钮会把 `errors` 逐条列在弹窗里，便于用户定位修改。开关位于 `service/workflow_validation_hook.go` 的 `blockPublishOnValidationError`。

## 端口默认值字段：统一使用 `default_value`

同一个「默认值」历史上存在两种写法，现已统一：

| 写法 | 出现位置 | 现状 |
| --- | --- | --- |
| `default_value` | 后端 `variable_definitions.default_value` 列、box-app `resolveInputs`（优先级 `value > default_value > default`）、画布导入的模板变量 | **规范字段** |
| `default` | 早期 LogicFlow 画布节点属性 | **仅作兼容读入**，写入时统一迁移为 `default_value` |

- 后端：`normalizePortDefaults` 在补全阶段把 `default` 迁移到 `default_value` 并删除旧字段；校验器用 `portDefault()` 读取（`default_value` 优先，回退 `default`）。
- 前端：`src/utils/workflow-port.ts` 提供 `readPortDefault` / `writePortDefault` / `normalizePort`，画布与转换层统一走这三个函数。
- **`false` 与 `0` 是合法默认值**，读取时用 `!== undefined` 判断，不会因「假值」被误判为空。

> box-app 的 `resolveInputs` 对 ref 分支的回退顺序是 `default` 优先于 `default_value`（与无 ref 分支相反）。仿真器已照抄这一细节，详见 `docs/编排测试-前端仿真测试说明.md`。

## 后端环境变量

沿用 Python 脚本生成已有的 LLM 配置，无需新增：

```env
LLM_API_BASE=http://gpustack.luxvisions-inno.com/v1
LLM_API_KEY=<后端配置真实密钥>
LLM_MODEL=deepseek-v4-flash
LLM_TEMPERATURE=0
LLM_REQUEST_TIMEOUT=60s
LLM_TOP_K=20
LLM_ENABLE_THINKING=true
LLM_MAX_REQUEST_BYTES=1048576
LLM_MAX_CODE_BYTES=65536
```

## 代码结构

| 文件 | 职责 |
| --- | --- |
| `service/llm_client.go` | 与 `python_script_generator_service.go` 共用的 OpenAI 兼容调用 |
| `service/workflow_graph.go` | 精简（去视图字段）与补全（模板 ID、布局、样式） |
| `service/workflow_validator.go` | 统一校验器：结构、模板、连通性、引用、必填、Python 静态检查 |
| `service/workflow_ai_prompt.go` | 编排规则常量 + 节点说明生成 + 提示词拼装 |
| `service/workflow_ai_service.go` | 单轮生成：拼提示词 → 调 LLM → 解析 → 补全 → 校验 |
| `service/workflow_graph_service.go` | `normalize` / `validate` 两个接口的实现 |
| `service/workflow_validation_hook.go` | 把统一校验器接入保存与发布流程 |
| `api/controllers/workflow_ai_controller.go` | 三个对外接口 |

完整请求/响应模型也可查看 Swagger：

```text
{BASE_CONTEXT}/swagger/index.html
```
