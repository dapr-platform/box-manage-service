-- rpa_wechat_proxy node template
INSERT INTO node_templates (id, type_key, name, category, layout, icon, description, default_config, metadata, inputs, outputs, data, version, is_active, is_system, sort_order, created_at, updated_at)
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
