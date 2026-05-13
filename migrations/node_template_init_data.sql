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
SELECT 83, 0, '', 5, 'count', '循环次数', 'string', 'input', '"3"'::jsonb, false, '', '', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 83);

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
