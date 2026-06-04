-- ============================================================
-- Node Templates Initialization Data (Idempotent)
-- 系统启动时执行，确保多次重启不会重复插入数据
-- 使用 WHERE NOT EXISTS 确保幂等性
-- ============================================================

BEGIN;

-- ============================================
-- 1. 系统预置节点模板（仅不存在时插入）
-- ============================================

-- Template: 开始节点 (start, id=1)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 1, 'start', '开始节点', 'logic', 'single', '▶', '工作流的起始节点', NULL, '{"variables":null}', '', '', '', true, true, 1, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'start' OR id = 1);

-- Template: 结束节点 (end, id=2)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 2, 'end', '结束节点', 'logic', 'single', '⏹', '工作流的结束节点', NULL, '{"variables":null}', '', '', '', true, true, 2, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'end' OR id = 2);

-- Template: 并发开始 (concurrency_start, id=3)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 3, 'concurrency_start', '并发开始', 'logic', 'paired', '⇉', '标记并发执行区域的开始', NULL, '{"variables":null}', '', 'concurrency_start', 'concurrency_end', true, true, 3, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'concurrency_start' OR id = 3);

-- Template: 并发结束 (concurrency_end, id=4)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 4, 'concurrency_end', '并发结束', 'logic', 'paired', '⇇', '标记并发执行区域的结束，等待所有并发分支完成', NULL, '{"variables":null}', '', 'concurrency_start', 'concurrency_end', true, true, 4, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'concurrency_end' OR id = 4);

-- Template: 循环开始 (loop_start, id=5)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 5, 'loop_start', '循环开始', 'logic', 'paired', '↻', '标记循环区域的开始', NULL, '{"variables":null}', '', 'loop_start', 'loop_end', true, true, 5, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'loop_start' OR id = 5);

-- Template: 循环结束 (loop_end, id=6)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 6, 'loop_end', '循环结束', 'logic', 'paired', '↺', '标记循环区域的结束，判断是否继续循环', NULL, '{"variables":null}', '', 'loop_start', 'loop_end', true, true, 6, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'loop_end' OR id = 6);

-- Template: KVM接入节点 (kvm, id=7)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 7, 'kvm', 'KVM接入节点', 'business', 'single', '🖥', '连接和控制KVM设备', NULL, '{"variables":null}', '', '', '', true, true, 10, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'kvm' OR id = 7);

-- Template: Reasoning推理节点 (reasoning, id=8)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 8, 'reasoning', 'Reasoning推理节点', 'business', 'single', '🤖', '调用AI模型进行推理计算', NULL, '{"variables":null}', '', '', '', true, true, 11, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'reasoning' OR id = 8);

-- Template: PythonScript脚本节点 (python_script, id=9)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 9, 'python_script', 'PythonScript脚本节点', 'business', 'single', '🐍', '执行自定义Python脚本', NULL, '{"variables":null}', '', '', '', true, true, 12, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'python_script' OR id = 9);

-- Template: MQTT推送节点 (mqtt, id=10)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 10, 'mqtt', 'MQTT推送节点', 'business', 'single', '📡', '向MQTT服务器推送消息', NULL, '{"variables":null}', '', '', '', true, true, 13, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'mqtt' OR id = 10);

-- Template: HTTP请求 (http_request, id=16)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 16, 'http_request', 'HTTP请求', 'business', 'single', '🌐', 'HTTP请求', NULL, '{"variables":null}', '', '', '', true, true, 14, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'http_request' OR id = 16);

-- Template: 企业微信推送 (wechat_work, id=17)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 17, 'wechat_work', '企业微信推送', 'business', 'single', '💬', '向企业微信群机器人推送消息，支持 text/markdown/news 三种类型', NULL, '{"variables":null}', '', '', '', true, true, 15, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'wechat_work' OR id = 17);

-- Template: 人脸通知 (face_notify, id=18)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 18, 'face_result_parser', '人脸识别结果处理', 'business', 'single', '👤', '处理人脸识别结果：base64图片落盘→生成URL→拼装企业微信markdown通知', NULL, '{"variables":null}', '', '', '', true, true, 16, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'face_result_parser' OR id = 18);

-- Template: 人脸比对 (face_compare, id=19)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 19, 'face_compare', '人脸比对', 'business', 'single', '🔍', '调用人脸比对接口，传入图片返回匹配结果，出参对接 face_result_parser', NULL, '{"variables":null}', '', '', '', true, true, 17, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'face_compare' OR id = 19);

-- ============================================
-- 2. 变量定义（仅不存在时插入）
-- ============================================

-- concurrency_start 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 79, 0, '', 3, 'mode', '并发模式', 'string', 'input', '"all"'::jsonb, false, '', 'all/any', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 79);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 80, 0, '', 3, 'max_concurrency', '最大并发数', 'string', 'input', '"3"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 80);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 81, 0, '', 3, 'timeout', '超时时间', 'string', 'input', '"30"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 81);

-- loop_start 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 82, 0, '', 5, 'loop_type', '循环类型', 'string', 'input', '"for"'::jsonb, false, '', '- for: 固定次数循环 - while: 条件循环 - foreach: 遍历集合', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 82);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 83, 0, '', 5, 'count', '循环次数', 'string', 'input', '"3"'::jsonb, false, '', 'for 循环使用：固定执行的次数', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 83);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 285, 0, '', 5, 'condition', '循环条件', 'string', 'input', '"true"'::jsonb, false, '', 'while 循环使用：条件表达式，例如 {count} > 0', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 285);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 286, 0, '', 5, 'items', '遍历集合', 'string', 'input', '"[]"'::jsonb, false, '', 'foreach 循环使用：待遍历的数组变量引用', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 286);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 287, 0, '', 5, 'max_iterations', '最大迭代次数', 'string', 'input', '"1000"'::jsonb, false, '', '防止死循环，超出此值自动终止', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 287);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 288, 0, '', 5, 'timeout', '超时时间(秒)', 'string', 'input', '"300"'::jsonb, false, '', '循环总超时，超出自动终止', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 288);

-- mqtt 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 84, 0, '', 10, 'broker', 'MQTT服务器地址', 'string', 'input', '"tcp://localhost:1883"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 84);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 85, 0, '', 10, 'client_id', '客户端ID', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 85);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 86, 0, '', 10, 'username', '用户名', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 86);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 87, 0, '', 10, 'password', '密码', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 87);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 88, 0, '', 10, 'topic', '主题', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 88);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 89, 0, '', 10, 'payload', '消息内容', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 89);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 90, 0, '', 10, 'timeout', '超时时间（秒）', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 90);

-- python_script 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 91, 0, '', 9, 'input', '入参', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 91);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 93, 0, '', 9, 'output', '出参', 'string', 'output', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 93);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 95, 0, '', 9, 'script_content', '脚本', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 95);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 96, 0, '', 9, 'timeout_seconds', '超时时间', 'string', 'input', '"30"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 96);

-- reasoning 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 97, 0, '', 8, 'inference_type', '推理类型', 'string', 'input', '"detection"'::jsonb, true, '', '目标检测', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 97);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 98, 0, '', 8, 'model_key', '模型标识', 'string', 'input', '"detection-yolov8-bm1684x-yolov8n"'::jsonb, true, '', '用来推理的模型', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 98);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 99, 0, '', 8, 'rtsp_url', '视频流地址', 'string', 'input', '"rtsp://192.168.1.100:554/stream"'::jsonb, true, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 99);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 100, 0, '', 8, 'skip_frame', '跳帧数', 'number', 'input', '"0"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 100);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 101, 0, '', 8, 'confidence_threshold', '置信度阈值', 'number', 'input', '0'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 101);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 102, 0, '', 8, 'nms_threshold', 'NMS 阈值', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 102);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 103, 0, '', 8, 'rois', 'ROI 区域配置列表', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 103);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 104, 0, '', 8, 'output', '推理结果写入的变量名', 'string', 'output', '"reasoning_result"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 104);

-- http_request 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 105, 0, '', 16, 'method', 'Method', 'string', 'input', '"GET"'::jsonb, false, '', 'GET/POST/PUT/DELETE等', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 105);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 106, 0, '', 16, 'url', '请求URL', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 106);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 107, 0, '', 16, 'headers', '请求头', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 107);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 108, 0, '', 16, 'body', '请求体', 'string', 'input', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 108);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 109, 0, '', 16, 'timeout', '超时时间（秒）', 'string', 'input', '"30"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 109);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 272, 0, '', 16, 'output', '输出', 'string', 'output', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 272);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 273, 0, '', 16, 'status_code', '响应状态码', 'string', 'output', '""'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 273);

-- wechat_work 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 274, 0, '', 17, 'key', 'Webhook Key', 'string', 'input', '""'::jsonb, true, '', '企业微信群机器人 Webhook key（URL 自动拼接）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 274);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 275, 0, '', 17, 'msgtype', '消息类型', 'string', 'input', '"text"'::jsonb, true, '', 'text / markdown / markdown_v2 / news', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 275);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 276, 0, '', 17, 'body', '消息内容体', 'json', 'input', '{"content":""}'::jsonb, true, '', '对应 msgtype 的消息体，支持 {变量} 和 {parent.child} 模板替换', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 276);

-- face_result_parser 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 277, 0, '', 18, 'face_results', '人脸识别结果', 'json', 'input', '"[]"'::jsonb, true, '', '引用上游人脸识别节点的输出对象数组', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 277);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 278, 0, '', 18, 'image', '推理图片', 'string', 'input', '""'::jsonb, true, '', '推理原图 base64 字符串，用于绘制人脸标注框', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 278);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 279, 0, '', 18, 'score', '置信度阈值', 'string', 'input', '"0.5"'::jsonb, false, '', '阈值(0.0-1.0)，≥此值标绿框，低于此值标红框', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 279);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 280, 0, '', 18, 'template', '输出模板', 'text', 'input', '"## 📷 人脸匹配通知\n\n### 匹配人员列表\n\n{facestable}\n\n> 共匹配到 **{count}** 人\n\n### 抓拍图片\n\n![人脸抓拍图片]({image})"'::jsonb, false, '', 'markdown 输出模板，支持占位符: {facestable} {count} {image}，留空使用默认', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 280);

-- face_compare 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 281, 0, '', 19, 'api_url', '接口地址', 'string', 'input', '"http://10.188.96.7:5004/compareFaceImg"'::jsonb, false, '', '人脸比对接口URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 281);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 282, 0, '', 19, 'api_key', 'API Key', 'string', 'input', '"zlghcgePiO"'::jsonb, false, '', '人脸比对接口认证 Key', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 282);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 283, 0, '', 19, 'score', '置信度阈值', 'string', 'input', '"0.5"'::jsonb, false, '', '匹配置信度阈值(0.0-1.0)', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 283);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 284, 0, '', 19, 'image', '待比对图片', 'string', 'input', '""'::jsonb, true, '', '上游推理节点的抓拍图 base64', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 284);

-- Template: 检测结果过滤 (detection_filter, id=20)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 20, 'detection_filter', '检测结果过滤', 'business', 'single', '🎯', '按类别和置信度过滤推理检测结果，仅保留符合条件的 detections，输出匹配数量和透传推理图片', NULL, '{"variables":null}', '', '', '', true, true, 18, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'detection_filter' OR id = 20);

-- detection_filter 的变量
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 293, 0, '', 20, 'detections', '检测结果', 'array', 'input', '"[]"'::jsonb, true, '', '上游推理节点的 detections 输出', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 293);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 294, 0, '', 20, 'class_id', '目标类别ID', 'string', 'input', '""'::jsonb, false, '', '需要匹配的类别ID（留空=所有类别）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 294);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 295, 0, '', 20, 'score', '置信度阈值', 'string', 'input', '"0.5"'::jsonb, false, '', '最低置信度(0.0-1.0)', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 295);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 296, 0, '', 20, 'image', '推理图片', 'string', 'input', '""'::jsonb, false, '', '上游推理节点的 base64 图片（透传输出）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 296);

-- detection_filter 的出参（供下游节点引用）
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 297, 0, '', 20, 'matched_detections', '匹配结果', 'array', 'output', '"[]"'::jsonb, false, '', '过滤后的检测结果数组', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 297);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 298, 0, '', 20, 'match_count', '匹配数量', 'number', 'output', '"0"'::jsonb, false, '', '符合条件的检测目标数量', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 298);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 299, 0, '', 20, 'has_match', '有匹配', 'boolean', 'output', '"false"'::jsonb, false, '', '是否存在符合条件的检测目标', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 299);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 300, 0, '', 20, 'image', '推理图片', 'string', 'output', '""'::jsonb, false, '', '上游推理节点的 base64 图片（透传）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 300);

-- Template: 图片标注 (image_annotator, id=21)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 21, 'image_annotator', '图片标注', 'business', 'single', '🖼️', '将推理检测结果绘制在图片上：class_id→class_name，画绿色框+标签，可选上传获取URL', NULL, '{"variables":null}', '', '', '', true, true, 19, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'image_annotator' OR id = 21);

-- image_annotator 的入参
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 310, 0, '', 21, 'image', '推理图片', 'string', 'input', '""'::jsonb, true, '', '上游推理节点的 base64 图片', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 310);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 311, 0, '', 21, 'detections', '检测结果', 'array', 'input', '"[]"'::jsonb, true, '', '上游推理节点或 detection_filter 的 detections 输出', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 311);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 312, 0, '', 21, 'upload', '上传图片', 'boolean', 'input', '"false"'::jsonb, false, '', '是否将标注图上传服务器获取 URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 312);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 313, 0, '', 21, 'upload_url', '上传地址', 'string', 'input', '""'::jsonb, false, '', '图片上传接口 URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 313);

-- image_annotator 的出参
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 314, 0, '', 21, 'image', '标注图片', 'string', 'output', '""'::jsonb, false, '', '标注后的 base64 图片', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 314);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 315, 0, '', 21, 'image_url', '图片URL', 'string', 'output', '""'::jsonb, false, '', '上传后的图片访问 URL（upload=true 时有效）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 315);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 316, 0, '', 21, 'detections', '检测结果', 'array', 'output', '"[]"'::jsonb, false, '', '带 class_name 的检测结果（透传）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 316);

-- ============================================
-- 3. 重置序列（确保后续业务插入的自增ID不冲突）
-- 使用 last_value 避免序列回退，仅向前推进
-- ============================================
SELECT setval('node_templates_id_seq', GREATEST(
    (SELECT COALESCE(MAX(id), 0) FROM node_templates),
    (SELECT last_value FROM node_templates_id_seq)
));
SELECT setval('variable_definitions_id_seq', GREATEST(
    (SELECT COALESCE(MAX(id), 0) FROM variable_definitions),
    (SELECT last_value FROM variable_definitions_id_seq)
));

COMMIT;
