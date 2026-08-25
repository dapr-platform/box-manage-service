# Python 节点脚本生成 API

## 功能说明

该接口根据 Python 节点的自然语言描述和工作流节点上下文，调用后端配置的 DeepSeek/GPUStack OpenAI 兼容接口生成 Python 脚本草稿。

前端只需要提交工作流和节点的身份信息。当前节点的输入参数、输出参数以及节点配置由后端从数据库自动查询并注入生成上下文，前端不要重复提交或覆盖这些字段。

接口只负责生成和基础校验：

- 不执行 Python 脚本
- 不保存工作流或节点配置
- 不覆盖已有脚本
- 前端应先展示脚本，用户确认后再调用已有的工作流更新接口保存

## 接口地址

如果未配置 `BASE_CONTEXT`：

```text
POST /api/v1/python-script/generate
```

如果配置了 `BASE_CONTEXT`，需要把它加在路径前面。例如：

```text
BASE_CONTEXT=/api/box-manage-service
POST /api/box-manage-service/api/v1/python-script/generate
```

节点模板语义路径也可使用：

```text
POST /api/v1/node-templates/python-script/generate
```

两个路径由同一个控制器处理，鉴权、请求参数、响应格式和生成逻辑完全一致。`/api/v1/python-script/generate` 是主路径，`/api/v1/node-templates/python-script/generate` 是语义兼容路径，前端任选其一即可。

## 请求头

```http
Content-Type: application/json
Authorization: Bearer <登录接口返回的JWT>
```

后端全局启用了认证中间件，前端应携带登录后的 JWT。LLM 的 `LLM_API_KEY` 只配置在后端，不能从前端传入。

workflow_id + node_id
↓
查询当前节点定义
↓
读取节点 Config
↓
查询该节点的变量定义
↓
按 direction 分成输入参数和输出参数
↓
组装 Prompt 调用 DeepSeek
↓
返回 Python 脚本

## 请求参数

### 前端请求字段

前端只传以下 3 个字段：

```json
{
  "workflow_id": 69,
  "node_id": "python_script-27ac",
  "description": "过滤上游订单，只保留 status 为 ok 的数据"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `workflow_id` | `uint` | 是 | 工作流 ID，必须大于 0 |
| `node_id` | `string` | 是 | 当前节点 ID。后端同时支持节点的 `node_id` 或 `node_key_name` |
| `description` | `string` | 是 | 节点的自然语言处理目标，不能为空 |

### 后端自动填充的字段

后端根据 `workflow_id` 和 `node_id` 查询并注入以下上下文，前端不需要传，也不应传：

| 字段 | 来源 | 用途 |
| --- | --- | --- |
| `node_inputs` | 当前节点关联的输入变量定义 | 约束脚本直接使用的顶层输入变量 |
| `node_outputs` | 当前节点关联的输出变量定义 | 约束脚本直接赋值的输出变量和类型 |
| `node_config` | 当前节点定义的 `config` | 生成脚本的输入/输出参数与静态配置约束 |

旧版本支持直接传入 `upstream`、`inputs`、`outputs`、`output_schema`、`mapping`、`vars_schema`、`vars`、`config_schema`、`config`、`examples` 等完整上下文。这些字段仅用于兼容已有内部调用方，前端新接入不应再组装或传递。

## 成功响应

HTTP 状态码为 `200`，统一响应格式如下：

```json
{
  "code": 0,
  "message": "生成Python脚本成功",
  "data": {
    "code": "import json\n\nrows = [row for row in source_rows if row.get(\"status\") == status]",
    "warnings": [
      "脚本仅完成生成和入口校验，未在服务端执行；应用前请在沙箱中试跑。"
    ],
    "used_inputs": [],
    "model": "deepseek-v4-flash"
  },
  "timestamp": "2026-08-23 12:00:00"
}
```

前端主要使用 `data.code`：

- 放入代码编辑器供用户预览和修改
- 用户确认后，再通过工作流/节点更新接口保存
- 不要直接在浏览器执行返回的 Python 代码
- 保存到 Python 节点的 `config.script_content` 后，由 box-app 执行

## 错误响应

错误格式：

```json
{
  "code": 400,
  "message": "description不能为空",
  "error": "description is required",
  "timestamp": "2026-08-23 12:00:00"
}
```

| HTTP 状态码 | 场景 |
| --- | --- |
| `400` | JSON 格式错误、缺少 `description`、上下文超过大小限制 |
| `502` | 节点上下文查询失败，或模型返回空内容、包含函数入口/`ctx`/顶层 `return`、包含禁止操作、修复后仍校验失败 |
| `503` | 后端未配置 `LLM_API_KEY`、LLM 地址不可用或请求超时 |

## 前端调用示例

```js
export async function generatePythonScript(token, {
  workflowId,
  nodeId,
  description
}) {
  const response = await fetch(
    "/api/box-manage-service/api/v1/python-script/generate",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${token}`
      },
      body: JSON.stringify({
        workflow_id: workflowId,
        node_id: nodeId,
        description
      })
    }
  );

  const result = await response.json();
  if (!response.ok || result.code !== 0) {
    throw new Error(result.error || result.message || "脚本生成失败");
  }
  return result.data;
}

const draft = await generatePythonScript(token, {
  workflowId: 69,
  nodeId: "python_script-27ac",
  description: "过滤上游订单，只保留 status 为 ok 的数据"
});
console.log(draft.code);
```

如果部署时 `BASE_CONTEXT` 不是 `/api/box-manage-service`，请按实际配置替换请求路径；也可以调用兼容路径 `/api/box-manage-service/api/v1/node-templates/python-script/generate`。

### box-app 运行时约定

生成提示会把三类当前节点信息明确交给模型，但这些字段不会作为运行时对象注入 Python：

- `node_inputs`：当前节点可读取的输入参数。box-app 会按 `node_config.input_params[].name` 将值注入为同名顶层变量，脚本直接使用变量名。
- `node_outputs`：当前节点必须产出的字段。box-app 会按 `node_config.output_params[].name` 自动捕获同名顶层变量并写回节点输出，脚本直接赋值，不使用 `return`。
- `node_config`：当前节点配置。生成结果必须是顶层 Python 代码片段，并在代码开头显式声明实际使用的 `import`。

生成脚本不应包含 `def run(ctx)`、`ctx.inputs`、`ctx.config` 或 `ctx.vars`。例如：

```python
import json

rows = [row for row in source_rows if row.get("status") == status]
```

## 后端环境变量

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

完整请求/响应模型也可查看 Swagger：

```text
{BASE_CONTEXT}/swagger/index.html
```
