-- ============================================================
-- Dictionary Field & Instance Initialization Data (Idempotent)
-- 系统启动时执行，确保新环境部署时字典数据自动初始化
-- 使用 WHERE NOT EXISTS 确保多次重启不会重复插入
-- 已存在的数据不做任何修改（不覆盖人工维护值）
-- ============================================================

BEGIN;

-- ============================================
-- 1. 字典字段定义：node_group（呈现分组）
-- ============================================
INSERT INTO dictionary_field_definitions (
    created_at, updated_at, field_key, field_name, field_type,
    description, sort_order, is_enabled, is_system, remark
)
SELECT
    NOW(), NOW(), 'node_group', '呈现分组', 'node_group',
    '用于业务编排节点分组', 0, true, true, '用于业务编排节点分组'
WHERE NOT EXISTS (
    SELECT 1 FROM dictionary_field_definitions WHERE field_key = 'node_group'
);

-- ============================================
-- 2. 字典实例：node_group 分组项
--    按 (field_key, instance_key) 判重
-- ============================================

-- 实例: 逻辑节点 (logic)
INSERT INTO dictionary_instances (
    created_at, updated_at, field_key, instance_key, "label", value,
    extra, sort_order, is_enabled, is_default, remark
)
SELECT
    NOW(), NOW(), 'node_group', 'logic', '逻辑节点', 'logic',
    '', 0, true, false, ''
WHERE NOT EXISTS (
    SELECT 1 FROM dictionary_instances WHERE field_key = 'node_group' AND instance_key = 'logic'
);

-- 实例: 业务节点 (business)
INSERT INTO dictionary_instances (
    created_at, updated_at, field_key, instance_key, "label", value,
    extra, sort_order, is_enabled, is_default, remark
)
SELECT
    NOW(), NOW(), 'node_group', 'business', '业务节点', 'business',
    '', 1, true, false, ''
WHERE NOT EXISTS (
    SELECT 1 FROM dictionary_instances WHERE field_key = 'node_group' AND instance_key = 'business'
);

-- 实例: 结果节点 (result)
INSERT INTO dictionary_instances (
    created_at, updated_at, field_key, instance_key, "label", value,
    extra, sort_order, is_enabled, is_default, remark
)
SELECT
    NOW(), NOW(), 'node_group', 'result', '结果节点', 'result',
    '', 2, true, false, ''
WHERE NOT EXISTS (
    SELECT 1 FROM dictionary_instances WHERE field_key = 'node_group' AND instance_key = 'result'
);

-- 实例: 推送节点 (forward)
INSERT INTO dictionary_instances (
    created_at, updated_at, field_key, instance_key, "label", value,
    extra, sort_order, is_enabled, is_default, remark
)
SELECT
    NOW(), NOW(), 'node_group', 'forward', '推送节点', 'forward',
    '', 3, true, false, ''
WHERE NOT EXISTS (
    SELECT 1 FROM dictionary_instances WHERE field_key = 'node_group' AND instance_key = 'forward'
);

COMMIT;
