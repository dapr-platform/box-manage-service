# Python 节点脚本生成 API

## 功能说明

该接口根据 Python 节点的自然语言描述、DAG 上下游数据契约和样例数据，调用后端配置的 DeepSeek/GPUStack OpenAI 兼容接口生成 Python 脚本草稿。

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

## 请求头

```http
Content-Type: application/json
Authorization: Bearer <登录接口返回的JWT>
```

后端全局启用了认证中间件，前端应携带登录后的 JWT。LLM 的 `LLM_API_KEY` 只配置在后端，不能从前端传入。

## 请求参数

```json
{
  "description": "过滤上游订单，只保留 status 为 ok 的数据",
  "upstream": [
    {
      "id": "fetch_orders",
      "name": "获取订单",
      "schema": {
        "type": "object",
        "properties": {
          "rows": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": { "type": "integer" },
                "status": { "type": "string" }
              }
            }
          }
        }
      },
      "sample": {
        "rows": [
          { "id": 1, "status": "ok" },
          { "id": 2, "status": "fail" }
        ]
      }
    }
  ],
  "output_schema": {
    "type": "object",
    "required": ["rows"],
    "properties": {
      "rows": { "type": "array" }
    }
  },
  "mapping": {},
  "vars_schema": {},
  "vars": {},
  "config_schema": {
    "type": "object",
    "properties": {
      "status": { "type": "string" }
    }
  },
  "config": {
    "status": "ok"
  },
  "examples": []
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `description` | `string` | 是 | 节点的自然语言处理目标 |
| `upstream` | `array` | 否 | 上游节点 ID、输出 schema 和样例 |
| `upstream[].id` | `string` | 否 | 上游节点 ID，生成脚本通过 `ctx.inputs[id]` 读取 |
| `upstream[].schema` | `object` | 否 | 上游输出 JSON Schema |
| `upstream[].sample` | `any` | 否 | 脱敏后的样例数据 |
| `output_schema` | `object` | 否 | 当前节点输出 JSON Schema |
| `mapping` | `any` | 否 | 边上的字段映射规则 |
| `vars_schema` | `object` | 否 | `ctx.vars` 的 schema |
| `vars` | `object` | 否 | 当前可用的工作流变量样例或值 |
| `config_schema` | `object` | 否 | `ctx.config` 的 schema |
| `config` | `object` | 否 | 当前节点静态配置 |
| `examples` | `array` | 否 | 可选的 few-shot 示例 |

## 成功响应

HTTP 状态码为 `200`，统一响应格式如下：

```json
{
  "code": 0,
  "message": "生成Python脚本成功",
  "data": {
    "code": "def run(ctx) -> dict:\n    rows = ctx.inputs[\"fetch_orders\"][\"rows\"]\n    return {\"rows\": [row for row in rows if row.get(\"status\") == \"ok\"]}",
    "warnings": [
      "脚本仅完成生成和入口校验，未在服务端执行；应用前请在沙箱中试跑。"
    ],
    "used_inputs": ["fetch_orders"],
    "model": "deepseek-v4-flash"
  },
  "timestamp": "2026-08-23 12:00:00"
}
```

前端主要使用 `data.code`：

- 放入代码编辑器供用户预览和修改
- 用户确认后，再通过工作流/节点更新接口保存
- 不要直接在浏览器执行返回的 Python 代码

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
| `502` | 模型返回空内容、缺少 `run(ctx)`、包含禁止操作或修复后仍校验失败 |
| `503` | 后端未配置 `LLM_API_KEY`、LLM 地址不可用或请求超时 |

## 前端调用示例

```js
export async function generatePythonScript(token, context) {
  const response = await fetch(
    "/api/v1/python-script/generate",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${token}`
      },
      body: JSON.stringify(context)
    }
  );

  const result = await response.json();
  if (!response.ok || result.code !== 0) {
    throw new Error(result.error || result.message || "脚本生成失败");
  }
  return result.data;
}
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
