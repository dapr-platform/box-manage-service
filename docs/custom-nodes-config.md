# 自定义工作流节点配置说明

---

## 1. 企业微信推送节点 (`wechat_work`)

向企业微信群机器人推送消息。URL 自动拼接，只需提供 key。

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `key` | string | ✅ | - | 群机器人 Webhook key（URL 自动拼为 `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key={key}`） |
| `msgtype` | string | ✅ | `text` | 消息类型：`text` / `markdown` / `markdown_v2` / `news` / `image` / `file` |
| `body` | json | ✅ | `{"content":""}` | 对应 msgtype 的消息体，支持 `{变量}` 和 `{parent.child}` 模板替换 |

### 使用示例

**text 类型：**
```json
{
  "key": "693axxx-7aoc-4bc4-97a0-0ec2sifa5aaa",
  "msgtype": "text",
  "body": {
    "content": "⚠️ 告警：检测到 {detection.label}，置信度 {detection.score}",
    "mentioned_list": ["@all"]
  }
}
```

**markdown_v2 类型：**
```json
{
  "key": "693axxx-7aoc-4bc4-97a0-0ec2sifa5aaa",
  "msgtype": "markdown_v2",
  "body": {
    "markdown_v2": {
      "content": "## 📷 人脸匹配通知\n\n| 字段 | 值 |\n| :----- | :---- |\n| **用户ID** | {face_result.user_id} |\n| **姓名** | {face_result.name} |\n| **匹配得分** | {face_result.score} |\n\n![人脸图片]({face_result.image_url})"
    }
  }
}
```

**news 类型：**
```json
{
  "key": "693axxx",
  "msgtype": "news",
  "body": {
    "articles": [{
      "title": "{alarm.title}",
      "description": "{alarm.desc}",
      "url": "{alarm.url}",
      "picurl": "{alarm.pic}"
    }]
  }
}
```

**image 类型：**
```json
{
  "key": "693axxx",
  "msgtype": "image",
  "body": { "base64": "{img_data}", "md5": "{img_md5}" }
}
```

**file 类型（需先调 upload_media 获取 media_id）：**
```json
{
  "key": "693axxx",
  "msgtype": "file",
  "body": { "media_id": "{upload_result.media_id}" }
}
```

### 出参

| 字段 | 说明 |
|------|------|
| `result` | 企业微信返回的 errmsg |
| `errcode` | 企业微信返回的错误码（0=成功） |

### 模板语法

- `{变量名}` — 替换为上游节点输出变量的值
- `{parent.child}` — 替换为嵌套字段的值（如 `{face_result.user_id}`）

---

## 2. 人脸比对节点 (`face_compare`)

调用人脸比对接口，传入抓拍图片和底库图片进行比对，返回匹配结果。

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `api_url` | string | 否 | `http://10.188.96.7:5004/compareFaceImg` | 人脸比对接口地址 |
| `api_key` | string | 否 | `zlghcgePiO` | 接口认证 Key（Header `X-API-Key`） |
| `score` | string | 否 | `0.5` | 匹配置信度阈值（0.0~1.0） |
| `image` | string | ✅ | - | 待比对图片（上游推理节点输出的 base64 字符串） |

### 请求格式

以 `multipart/form-data` 发送：
```
POST /compareFaceImg
Header: X-API-Key: {api_key}

Form:
  score: "0.5"
  imgs: [binary data of image]
```

支持多张图片，变量中 `image_xxx` 格式的字段会自动作为额外图片附加。

### 响应处理

| 响应 | 处理 |
|------|------|
| `{"message":"no match"}` | 无匹配结果，face_results 返回空数组 |
| `{"error":"..."}` | 接口异常 |
| `[[{...}]]` | 二维数组，自动展平为一维 |

### 出参

| 字段 | 说明 |
|------|------|
| `face_results` | 匹配结果数组 `[{user_id, user_name, face_location, score}]` |
| `face_count` | 匹配人数 |
| `duration_ms` | 接口耗时（毫秒） |

### 数据流

```
推理节点(image) → face_compare → face_results → face_result_parser → 标注图 → wechat_work
```

---

## 3. 人脸识别结果处理节点 (`face_result_parser`)

将上游推理节点的原图和人脸比对结果合成带标注的图片，生成企业微信推送消息体。

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `face_results` | json | ✅ | `[]` | 上游人脸比对节点的输出结果数组 |
| `image` | string | ✅ | - | 推理原图 base64 字符串 |
| `score` | float | 否 | `0.5` | 置信度阈值：≥此值标 🟢绿框，低于标 🔴红框 |
| `template` | text | 否 | 内置模板 | markdown 输出模板，支持占位符，留空使用默认 |

### 处理流程

```
image(base64) → 解码 → 画框标注 → 保存本地 → 生成URL
                              ↑
face_results → 遍历每个人脸 ──┤
                              ├── score ≥ 阈值 → 🟢绿框
                              └── score < 阈值 → 🔴红框

框头部标签：名称:xxx ID:xxx score:0.xx
```

### 出参

| 字段 | 说明 |
|------|------|
| `wechat_body` | 企业微信 markdown_v2 消息体（可直接作为 wechat_work 节点的 `body` 参数） |
| `annotated_image` | 标注图访问 URL |
| `content` | markdown 文本内容 |
| `face_count` | 匹配人数 |
| `score_threshold` | 使用的阈值 |

### 模板占位符

`template` 参数自定义输出格式，支持以下占位符：

| 占位符 | 替换为 |
|--------|--------|
| `{facestable}` | 匹配人员表格 |
| `{count}` | 匹配人数 |
| `{image}` | 标注图 URL |

默认模板：
```
## 📷 人脸匹配通知

### 匹配人员列表

{facestable}

> 共匹配到 **{count}** 人

### 抓拍图片

![人脸抓拍图片]({image})
```

自定义简洁版：
```
## ⚠️ 告警：{count} 人匹配

{facestable}

![图片]({image})
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `FACE_NOTIFY_IMAGE_DIR` | `./data/notify` | 标注图落盘目录 |
| `FACE_NOTIFY_IMAGE_URL` | - | 标注图访问URL前缀（例如 `http://host:port/api/box-manage-service/notify-images`） |

### 完整数据流

```
推理节点 → image(base64)
                ↓
         face_compare → face_results [{user_id, user_name, ...}]
                              ↓
                    face_result_parser → 标注图 URL + markdown
                              ↓
                         wechat_work → 企业微信推送
```

### 节点配置示例

face_compare:
```json
{
  "api_url": "http://10.188.96.7:5004/compareFaceImg",
  "api_key": "zlghcgePiO",
  "score": "0.5",
  "image": "{{reasoning_node.output}}"
}
```

face_result_parser:
```json
{
  "face_results": "{{face_compare_node.face_results}}",
  "image": "{{reasoning_node.output}}",
  "score": "0.5"
}
```

wechat_work:
```json
{
  "key": "693axxx-7aoc-4bc4-97a0-0ec2sifa5aaa",
  "msgtype": "markdown_v2",
  "body": "{{face_result_parser_node.wechat_body}}"
}
```
