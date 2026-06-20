-- ============================================================
-- Node Templates Initialization Data (Idempotent)
-- 系统启动时执行，确保多次重启不会重复插入数据
-- 使用 WHERE NOT EXISTS 确保幂等性
-- ============================================================

BEGIN;

--- truncate node_templates,variable_definitions

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
SELECT 18, 'face_result_parser', '人脸识别结果处理', 'business', 'single', '👤', '处理人脸识别结果：绘制人脸框+标签→输出两条企业微信消息（纯文本用户信息 + 标注图base64）', NULL, '{"variables":null}', '', '', '', true, true, 16, NOW(), NOW()
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

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2416, 0, '', 17, 'mentioned_list', '@用户列表', 'string', 'input', '""'::jsonb, false, '', '需要 @ 的企业微信用户 userid，多个用逗号分隔，如 zhangsan,lisi', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2416);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2427, 0, '', 17, 'mentioned_mobile_list', '@手机号列表', 'string', 'input', '""'::jsonb, false, '', '需要 @ 的企业微信用户手机号，多个用逗号分隔，如 138xxxx,139xxxx', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2427);

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

-- 注: template 入参已移除，节点改为固定输出纯文本 + base64 图片，不再支持自定义 markdown 模板

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

-- face_compare 的出参（供下游 face_result_parser 等节点引用）
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 289, 0, '', 19, 'face_results', '匹配人脸列表', 'array', 'output', '"[]"'::jsonb, false, '', '匹配到的人脸信息数组，无匹配或出错时为空数组', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 289);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 290, 0, '', 19, 'face_count', '匹配数量', 'number', 'output', '"0"'::jsonb, false, '', '匹配到的人脸数量', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 290);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 291, 0, '', 19, 'duration_ms', '接口耗时(ms)', 'number', 'output', '"0"'::jsonb, false, '', '人脸比对接口调用耗时', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 291);

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

-- Template: 图片标注 (image_annotator, id=24)
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 24, 'image_annotator', '图片标注', 'business', 'single', '🖼️', '将推理检测结果绘制在图片上：class_id→class_name，画绿色框+标签，可选上传获取URL', NULL, '{"variables":null}', '', '', '', true, true, 20, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'image_annotator' OR id = 24);

-- image_annotator 的入参
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2400, 0, '', 24, 'image', '推理图片', 'string', 'input', '""'::jsonb, true, '', '上游推理节点的 base64 图片', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2400);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2401, 0, '', 24, 'detections', '检测结果', 'array', 'input', '"[]"'::jsonb, true, '', '上游推理节点或 detection_filter 的 detections 输出', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2401);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2402, 0, '', 24, 'upload', '上传图片', 'boolean', 'input', '"false"'::jsonb, false, '', '是否将标注图上传服务器获取 URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2402);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2403, 0, '', 24, 'upload_url', '上传地址', 'string', 'input', '""'::jsonb, false, '', '图片上传接口 URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2403);

-- image_annotator 的出参
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2404, 0, '', 24, 'image', '标注图片', 'string', 'output', '""'::jsonb, false, '', '标注后的 base64 图片', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2404);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2405, 0, '', 24, 'image_url', '图片URL', 'string', 'output', '""'::jsonb, false, '', '上传后的图片访问 URL（upload=true 时有效）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2405);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2406, 0, '', 24, 'detections', '检测结果', 'array', 'output', '"[]"'::jsonb, false, '', '带 class_name 的检测结果（透传）', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2406);

-- face_result_parser 的出参（供下游 wechat_work 等节点引用）
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2407, 0, '', 18, 'body_text', '文本消息体', 'object', 'output', '"{\"content\":\"\"}"'::jsonb, false, '', '纯文本消息体，用于 wechat_work 节点 msgtype=text，含识别到的用户信息', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2407);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2408, 0, '', 18, 'body_image', '标注图消息体', 'object', 'output', '"{\"content\":\"\"}"'::jsonb, false, '', '标注图 base64 消息体，用于 wechat_work 节点 msgtype=text，含 data:image/jpeg;base64,... 前缀', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2408);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2409, 0, '', 18, 'face_count', '匹配数量', 'number', 'output', '"0"'::jsonb, false, '', '匹配到的人脸数量', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2409);

-- reasoning 的出参（供下游 detection_filter/image_annotator 等节点引用）
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2410, 0, '', 8, 'detections', '检测结果', 'array', 'output', '"[]"'::jsonb, false, '', '检测结果数组，含 class_id、confidence、bbox 等', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2410);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2411, 0, '', 8, 'detectObjects', '检测对象', 'array', 'output', '"[]"'::jsonb, false, '', '检测对象数组，含 leftTopX/Y、rightBtmX/Y、score 等', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2411);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2412, 0, '', 8, 'detection_count', '检测数量', 'number', 'output', '"0"'::jsonb, false, '', '检测到的目标数量', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2412);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2413, 0, '', 8, 'image_base64', '推理图片base64', 'string', 'output', '""'::jsonb, false, '', '推理输入的图片 base64 字符串', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2413);

-- face_compare 补充出参
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2414, 0, '', 19, 'http_status', 'HTTP状态码', 'number', 'output', '"0"'::jsonb, false, '', '人脸比对接口的 HTTP 响应状态码', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2414);

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


-- rpa_wechat_proxy node template
INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at)
SELECT 25, 'rpa_wechat_proxy', 'RPA企微推送', 'business', 'single', '📨', '通过RPA代理发送文本(@人)和图片到企业微信群', NULL, '{"variables":null}', '', '', '', true, true, 17, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'rpa_wechat_proxy' OR id = 25);

-- inputs
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2417, 0, '', 25, 'webhook_url', '企业微信Webhook URL', 'string', 'input', '""'::jsonb, true, '', '企业微信群机器人完整 Webhook URL', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2417);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2418, 0, '', 25, 'rpa_proxy_url', 'RPA代理地址', 'string', 'input', '"http://10.188.32.17:8080/RPA"'::jsonb, false, '', 'RPA 代理服务地址', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2418);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2419, 0, '', 25, 'body_text', '文本消息体', 'json', 'input', '{"content":""}'::jsonb, false, '', '文本消息体，引用 face_result_parser-xxx.body_text', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2419);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2420, 0, '', 25, 'body_image', '图片消息体', 'json', 'input', '{"content":""}'::jsonb, false, '', '图片消息体，引用 face_result_parser-xxx.body_image', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2420);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2421, 0, '', 25, 'mentioned_list', '@用户列表', 'string', 'input', '""'::jsonb, false, '', '需要 @ 的 userid，逗号分隔', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2421);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2422, 0, '', 25, 'mentioned_mobile_list', '@手机号列表', 'string', 'input', '""'::jsonb, false, '', '需要 @ 的手机号，逗号分隔', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2422);

-- outputs
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2423, 0, '', 25, 'text_sent', '文本发送成功', 'boolean', 'output', '"false"'::jsonb, false, '', '文本消息发送结果', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2423);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2424, 0, '', 25, 'image_sent', '图片发送成功', 'boolean', 'output', '"false"'::jsonb, false, '', '图片消息发送结果', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2424);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2425, 0, '', 25, 'error_message', '错误信息', 'string', 'output', '""'::jsonb, false, '', '失败时的错误信息', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2425);


-- reasoning_loop 循环推理匹配节点
INSERT INTO node_templates (id, type_key, name, category, layout, icon, description, default_config, metadata, inputs, outputs, data, version, is_active, is_system, sort_order, created_at, updated_at)
SELECT 26, 'reasoning_loop', '循环推理匹配', 'business', 'single', '🔄', '按指定次数循环推理，累计匹配目标 class_id 次数，达到阈值后输出成功', NULL, '{"variables":null}', '', '', '', true, true, 18, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM node_templates WHERE type_key = 'reasoning_loop' OR id = 26);

-- inputs
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2430, 0, '', 26, 'rtsp_url', 'RTSP地址', 'string', 'input', '""'::jsonb, true, '', 'RTSP 流地址', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2430);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2431, 0, '', 26, 'model_key', '模型Key', 'string', 'input', '""'::jsonb, true, '', '推理模型 key', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2431);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2432, 0, '', 26, 'class_id', '目标类别ID', 'number', 'input', '"0"'::jsonb, true, '', '要匹配的目标 class_id', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2432);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2433, 0, '', 26, 'total_count', '推理次数', 'number', 'input', '"10"'::jsonb, false, '', '执行推理的总次数', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2433);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2434, 0, '', 26, 'match_threshold', '匹配阈值', 'number', 'input', '"1"'::jsonb, false, '', '判定成功所需的最小匹配次数', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2434);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2435, 0, '', 26, 'score', '匹配置信度', 'number', 'input', '"0.5"'::jsonb, false, '', '匹配时的最低置信度阈值', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2435);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2436, 0, '', 26, 'interval_ms', '推理间隔(ms)', 'number', 'input', '"500"'::jsonb, false, '', '两次推理之间的间隔毫秒', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2436);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2437, 0, '', 26, 'confidence_threshold', '置信度阈值', 'number', 'input', '"0.0"'::jsonb, false, '', '推理置信度阈值', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2437);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2438, 0, '', 26, 'nms_threshold', 'NMS阈值', 'number', 'input', '"0.0"'::jsonb, false, '', 'NMS 阈值', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2438);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2439, 0, '', 26, 'inference_type', '推理类型', 'string', 'input', '"detection"'::jsonb, false, '', 'detection / segmentation / obb', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2439);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2440, 0, '', 26, 'roi', 'ROI区域', 'string', 'input', '""'::jsonb, false, '', '感兴趣区域', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2440);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2441, 0, '', 26, 'skip_frame', '跳帧数', 'number', 'input', '"0"'::jsonb, false, '', '跳帧间隔', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2441);

-- outputs
INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2445, 0, '', 26, 'matched', '匹配成功', 'boolean', 'output', '"false"'::jsonb, false, '', '是否达到匹配阈值', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2445);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2446, 0, '', 26, 'match_count', '匹配次数', 'number', 'output', '"0"'::jsonb, false, '', '实际匹配次数', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2446);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2447, 0, '', 26, 'detections', '检测结果', 'array', 'output', '"[]"'::jsonb, false, '', '最后一次推理的检测结果', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2447);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2448, 0, '', 26, 'detection_count', '检测数量', 'number', 'output', '"0"'::jsonb, false, '', '最后一次推理的检测数量', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2448);

INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at)
SELECT 2449, 0, '', 26, 'image_base64', '图片base64', 'string', 'output', '""'::jsonb, false, '', '最后一次抓帧的 base64', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM variable_definitions WHERE id = 2449);
