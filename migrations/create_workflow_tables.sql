-- 业务编排引擎数据库迁移脚本
-- 创建工作流相关表
-- 版本: 2.0.0
-- 日期: 2025-03-15
-- 说明: 完全符合需求文档（业务编排引擎需求.md）第4章数据模型设计

-- ============================================
-- 1. workflows（工作流定义表）
-- ============================================
CREATE TABLE IF NOT EXISTS workflows (
    id SERIAL PRIMARY KEY,
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    tags VARCHAR(255),
    version INTEGER NOT NULL DEFAULT 0,
    structure_json JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_by INTEGER,
    updated_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT uk_workflow_key_version UNIQUE (key_name, version)
);

CREATE INDEX idx_workflows_key_name ON workflows(key_name);
CREATE INDEX idx_workflows_status ON workflows(status);
CREATE INDEX idx_workflows_version ON workflows(version);
CREATE INDEX idx_workflows_deleted_at ON workflows(deleted_at);

COMMENT ON TABLE workflows IS '工作流定义表';
COMMENT ON COLUMN workflows.key_name IS '工作流标识（唯一键）';
COMMENT ON COLUMN workflows.version IS '版本号（从0开始递增）';
COMMENT ON COLUMN workflows.structure_json IS '流程结构JSON（冗余字段，用于快速下发）';
COMMENT ON COLUMN workflows.status IS '状态：draft（草稿）/published（已发布）/archived（已归档）';

-- ============================================
-- 2. workflow_instances（工作流实例表）
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_instances (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id),
    instance_id VARCHAR(100) NOT NULL UNIQUE,
    box_id INTEGER,
    status VARCHAR(20) NOT NULL,
    trigger_type VARCHAR(20) NOT NULL,
    trigger_data JSONB,
    context_data JSONB,
    variables JSONB,
    current_node_id VARCHAR(100),
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    duration INTEGER,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_instances_workflow_id ON workflow_instances(workflow_id);
CREATE INDEX idx_workflow_instances_instance_id ON workflow_instances(instance_id);
CREATE INDEX idx_workflow_instances_box_id ON workflow_instances(box_id);
CREATE INDEX idx_workflow_instances_status ON workflow_instances(status);
CREATE INDEX idx_workflow_instances_created_at ON workflow_instances(created_at);
CREATE INDEX idx_workflow_instances_deleted_at ON workflow_instances(deleted_at);

COMMENT ON TABLE workflow_instances IS '工作流实例表';
COMMENT ON COLUMN workflow_instances.instance_id IS '实例唯一标识';
COMMENT ON COLUMN workflow_instances.status IS '状态：pending/running/paused/completed/failed/cancelled';
COMMENT ON COLUMN workflow_instances.trigger_type IS '触发类型：manual/event/schedule/api';
COMMENT ON COLUMN workflow_instances.duration IS '执行耗时（秒）';

-- ============================================
-- 3. workflow_logs（工作流日志表）
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_logs (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    log_type VARCHAR(20) NOT NULL,
    operation_instance_id VARCHAR(100) NOT NULL,
    operation_instance_name VARCHAR(200),
    operation_instance_input JSONB,
    operation_instance_output JSONB,
    operation_instance_status VARCHAR(50),
    message TEXT NOT NULL,
    details JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_logs_workflow_instance ON workflow_logs(workflow_instance_id);
CREATE INDEX idx_workflow_logs_log_type ON workflow_logs(log_type);
CREATE INDEX idx_workflow_logs_operation_instance ON workflow_logs(operation_instance_id);
CREATE INDEX idx_workflow_logs_created_at ON workflow_logs(created_at);
CREATE INDEX idx_workflow_logs_deleted_at ON workflow_logs(deleted_at);

COMMENT ON TABLE workflow_logs IS '工作流日志表，记录节点和连接线的执行日志';
COMMENT ON COLUMN workflow_logs.log_type IS '日志类型：node（节点日志）/line（连接线日志）';
COMMENT ON COLUMN workflow_logs.operation_instance_id IS '操作实例ID：node实例id或line实例id';
COMMENT ON COLUMN workflow_logs.operation_instance_name IS '操作名称：node的node_name或line的condition_expression';
COMMENT ON COLUMN workflow_logs.operation_instance_input IS '操作实例输入：node的input_data或line的condition_context';
COMMENT ON COLUMN workflow_logs.operation_instance_output IS '操作实例输出：node的output_data或line的condition_result';
COMMENT ON COLUMN workflow_logs.operation_instance_status IS '操作实例状态：node的status或line的executed';

-- ============================================
-- 4. workflow_schedules（工作流调度配置表）
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_schedules (
    id SERIAL PRIMARY KEY,
    deployment_id INTEGER NOT NULL,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id),
    name VARCHAR(100) NOT NULL,
    schedule_type VARCHAR(20) NOT NULL,
    cron_expression VARCHAR(100),
    input_variables JSONB,
    event_type VARCHAR(50),
    event_filter JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 0,
    max_concurrent INTEGER NOT NULL DEFAULT 1,
    timeout INTEGER NOT NULL DEFAULT 3600,
    retry_policy JSONB,
    next_run_time TIMESTAMP,
    last_run_time TIMESTAMP,
    run_count INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_schedules_deployment_id ON workflow_schedules(deployment_id);
CREATE INDEX idx_workflow_schedules_workflow_id ON workflow_schedules(workflow_id);
CREATE INDEX idx_workflow_schedules_schedule_type ON workflow_schedules(schedule_type);
CREATE INDEX idx_workflow_schedules_is_enabled ON workflow_schedules(is_enabled);
CREATE INDEX idx_workflow_schedules_next_run_time ON workflow_schedules(next_run_time);
CREATE INDEX idx_workflow_schedules_deleted_at ON workflow_schedules(deleted_at);

COMMENT ON TABLE workflow_schedules IS '工作流调度配置表';
COMMENT ON COLUMN workflow_schedules.deployment_id IS '部署ID（调度针对部署对象）';
COMMENT ON COLUMN workflow_schedules.schedule_type IS '调度类型：manual（手动触发）/cron（定时触发）';
COMMENT ON COLUMN workflow_schedules.timeout IS '超时时间（秒）';

-- ============================================
-- 5. workflow_deployments（工作流部署表）
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_deployments (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    key VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id),
    box_id INTEGER NOT NULL,
    workflow_version INTEGER NOT NULL,
    deployment_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    workflow_json JSONB NOT NULL,
    deployed_at TIMESTAMP,
    rolled_back_at TIMESTAMP,
    previous_version INTEGER,
    error_message TEXT,
    deployed_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_deployments_key ON workflow_deployments(key);
CREATE INDEX idx_workflow_deployments_workflow_id ON workflow_deployments(workflow_id);
CREATE INDEX idx_workflow_deployments_box_id ON workflow_deployments(box_id);
CREATE INDEX idx_workflow_deployments_status ON workflow_deployments(deployment_status);
CREATE INDEX idx_workflow_deployments_deleted_at ON workflow_deployments(deleted_at);

COMMENT ON TABLE workflow_deployments IS '工作流部署表';
COMMENT ON COLUMN workflow_deployments.key IS '部署标识（唯一键）';
COMMENT ON COLUMN workflow_deployments.deployment_status IS '部署状态：pending/deploying/deployed/failed/rolled_back';
COMMENT ON COLUMN workflow_deployments.workflow_json IS '工作流JSON（下发到盒子的完整配置）';

-- ============================================
-- 6. node_templates（节点模板表）
-- ============================================
CREATE TABLE IF NOT EXISTS node_templates (
    id SERIAL PRIMARY KEY,
    type_key VARCHAR(50) NOT NULL UNIQUE,
    type_name VARCHAR(100) NOT NULL,
    category VARCHAR(20) NOT NULL,
    group_type VARCHAR(20) NOT NULL DEFAULT 'single',
    icon VARCHAR(255),
    description TEXT,
    config_schema JSONB,
    input_schema JSONB,
    output_schema JSONB,
    default_variables JSONB,
    script_template TEXT,
    start_node_key VARCHAR(50),
    end_node_key VARCHAR(50),
    is_system BOOLEAN NOT NULL DEFAULT false,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_node_templates_type_key ON node_templates(type_key);
CREATE INDEX idx_node_templates_category ON node_templates(category);
CREATE INDEX idx_node_templates_group_type ON node_templates(group_type);
CREATE INDEX idx_node_templates_is_enabled ON node_templates(is_enabled);
CREATE INDEX idx_node_templates_sort_order ON node_templates(sort_order);
CREATE INDEX idx_node_templates_deleted_at ON node_templates(deleted_at);

COMMENT ON TABLE node_templates IS '节点模板表';
COMMENT ON COLUMN node_templates.category IS '节点分类：logic（逻辑控制）/business（业务执行）';
COMMENT ON COLUMN node_templates.group_type IS '节点分组类型：single（单节点）/paired（成对节点）/container（容器节点）';
COMMENT ON COLUMN node_templates.default_variables IS '预定义的变量配置，拖入工作流时自动复制到variable_definitions';
COMMENT ON COLUMN node_templates.start_node_key IS '成对节点的开始节点key（仅paired类型使用）';
COMMENT ON COLUMN node_templates.end_node_key IS '成对节点的结束节点key（仅paired类型使用）';

-- ============================================
-- 7. node_definitions（节点定义表）
-- ============================================
CREATE TABLE IF NOT EXISTS node_definitions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_id VARCHAR(100) NOT NULL,
    node_template_id INTEGER NOT NULL REFERENCES node_templates(id),
    type_key VARCHAR(50) NOT NULL,
    type_name VARCHAR(100) NOT NULL,
    node_name VARCHAR(100) NOT NULL,
    node_key_name VARCHAR(100) NOT NULL,
    group_type VARCHAR(20) NOT NULL DEFAULT 'single',
    start_node_key VARCHAR(50),
    end_node_key VARCHAR(50),
    config JSONB,
    python_script TEXT,
    inputs JSONB,
    outputs JSONB,
    position JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT uk_node_workflow_node UNIQUE (workflow_id, node_id),
    CONSTRAINT uk_node_workflow_key_name UNIQUE (workflow_id, node_key_name)
);

CREATE INDEX idx_node_definitions_workflow_id ON node_definitions(workflow_id);
CREATE INDEX idx_node_definitions_node_id ON node_definitions(node_id);
CREATE INDEX idx_node_definitions_node_template_id ON node_definitions(node_template_id);
CREATE INDEX idx_node_definitions_node_key_name ON node_definitions(node_key_name);
CREATE INDEX idx_node_definitions_deleted_at ON node_definitions(deleted_at);

COMMENT ON TABLE node_definitions IS '节点定义表';
COMMENT ON COLUMN node_definitions.node_id IS '节点ID（对应structure_json中的node.id）';
COMMENT ON COLUMN node_definitions.node_key_name IS '节点英文标识（当前流程唯一）';
COMMENT ON COLUMN node_definitions.group_type IS '节点分组类型（从模板继承）';
COMMENT ON COLUMN node_definitions.start_node_key IS '成对节点的开始节点key（从模板继承）';
COMMENT ON COLUMN node_definitions.end_node_key IS '成对节点的结束节点key（从模板继承）';

-- ============================================
-- 8. node_instances（节点实例表）
-- ============================================
CREATE TABLE IF NOT EXISTS node_instances (
    id SERIAL PRIMARY KEY,
    instance_id VARCHAR(100) NOT NULL UNIQUE,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    node_def_id INTEGER NOT NULL REFERENCES node_definitions(id),
    node_id VARCHAR(100) NOT NULL,
    node_type VARCHAR(50) NOT NULL,
    node_name VARCHAR(100) NOT NULL,
    node_key_name VARCHAR(100) NOT NULL,
    config JSONB,
    status VARCHAR(20) NOT NULL,
    input_data JSONB,
    output_data JSONB,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    duration INTEGER,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_node_instances_instance_id ON node_instances(instance_id);
CREATE INDEX idx_node_instances_workflow_instance_id ON node_instances(workflow_instance_id);
CREATE INDEX idx_node_instances_node_def_id ON node_instances(node_def_id);
CREATE INDEX idx_node_instances_node_id ON node_instances(node_id);
CREATE INDEX idx_node_instances_status ON node_instances(status);
CREATE INDEX idx_node_instances_node_key_name ON node_instances(node_key_name);
CREATE INDEX idx_node_instances_deleted_at ON node_instances(deleted_at);

COMMENT ON TABLE node_instances IS '节点实例表';
COMMENT ON COLUMN node_instances.instance_id IS '节点实例唯一标识';
COMMENT ON COLUMN node_instances.node_def_id IS '关联节点定义，用于追溯节点配置来源';
COMMENT ON COLUMN node_instances.config IS '节点配置（从NodeDefinition复制，用于记录执行时的配置快照）';
COMMENT ON COLUMN node_instances.status IS '状态：pending/running/completed/failed/skipped';
COMMENT ON COLUMN node_instances.duration IS '执行耗时（毫秒）';

-- ============================================
-- 9. variable_definitions（变量定义表）
-- ============================================
CREATE TABLE IF NOT EXISTS variable_definitions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_id VARCHAR(100),
    node_template_id INTEGER REFERENCES node_templates(id),
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    direction VARCHAR(10) NOT NULL,
    default_value JSONB,
    required BOOLEAN NOT NULL DEFAULT false,
    ref_key_name VARCHAR(200),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT uk_variable_workflow_node_key UNIQUE (workflow_id, node_id, key_name)
);

CREATE INDEX idx_variable_definitions_workflow_id ON variable_definitions(workflow_id);
CREATE INDEX idx_variable_definitions_node_id ON variable_definitions(node_id);
CREATE INDEX idx_variable_definitions_node_template_id ON variable_definitions(node_template_id);
CREATE INDEX idx_variable_definitions_key_name ON variable_definitions(key_name);
CREATE INDEX idx_variable_definitions_deleted_at ON variable_definitions(deleted_at);

COMMENT ON TABLE variable_definitions IS '变量定义表';
COMMENT ON COLUMN variable_definitions.node_id IS '节点ID，为空表示全局变量';
COMMENT ON COLUMN variable_definitions.node_template_id IS '来源节点模板ID，用于追溯参数定义来源';
COMMENT ON COLUMN variable_definitions.type IS '变量类型：string/number/boolean/object/array/reference';
COMMENT ON COLUMN variable_definitions.direction IS '方向：input（输入）/output（输出）';
COMMENT ON COLUMN variable_definitions.ref_key_name IS '引用参数标识（格式：节点key_name.参数key_name）';

-- ============================================
-- 10. variable_instances（变量实例表）
-- ============================================
CREATE TABLE IF NOT EXISTS variable_instances (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    deployment_id INTEGER,
    variable_def_id INTEGER NOT NULL REFERENCES variable_definitions(id),
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    value JSONB,
    ref_key_name VARCHAR(200),
    scope VARCHAR(50) NOT NULL DEFAULT 'global',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_variable_instances_workflow_instance ON variable_instances(workflow_instance_id);
CREATE INDEX idx_variable_instances_deployment_id ON variable_instances(deployment_id);
CREATE INDEX idx_variable_instances_variable_def_id ON variable_instances(variable_def_id);
CREATE INDEX idx_variable_instances_key_name ON variable_instances(key_name);
CREATE INDEX idx_variable_instances_deleted_at ON variable_instances(deleted_at);

COMMENT ON TABLE variable_instances IS '变量实例表';
COMMENT ON COLUMN variable_instances.deployment_id IS '部署ID（可选），用于记录给一个部署配置的实际参数';
COMMENT ON COLUMN variable_instances.ref_key_name IS '引用参数标识（格式：节点key_name.参数key_name）';
COMMENT ON COLUMN variable_instances.scope IS '变量作用域：global（全局）/node（节点）';

-- ============================================
-- 11. line_definitions（连接线定义表）
-- ============================================
CREATE TABLE IF NOT EXISTS line_definitions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    line_id VARCHAR(100) NOT NULL,
    source_node_id VARCHAR(100) NOT NULL,
    target_node_id VARCHAR(100) NOT NULL,
    condition_type VARCHAR(20) DEFAULT 'none',
    logic_type VARCHAR(10) DEFAULT 'and',
    condition_expression TEXT,
    condition_expression_view TEXT,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT uk_line_workflow_line UNIQUE (workflow_id, line_id)
);

CREATE INDEX idx_line_definitions_workflow_id ON line_definitions(workflow_id);
CREATE INDEX idx_line_definitions_line_id ON line_definitions(line_id);
CREATE INDEX idx_line_definitions_source_node_id ON line_definitions(source_node_id);
CREATE INDEX idx_line_definitions_target_node_id ON line_definitions(target_node_id);
CREATE INDEX idx_line_definitions_deleted_at ON line_definitions(deleted_at);

COMMENT ON TABLE line_definitions IS '连接线定义表';
COMMENT ON COLUMN line_definitions.line_id IS '连接线ID（对应前端生成的line.id）';
COMMENT ON COLUMN line_definitions.condition_type IS '条件类型：none/simple/complex/expression';
COMMENT ON COLUMN line_definitions.logic_type IS '逻辑类型：and（与逻辑）/or（或逻辑），用于complex条件';
COMMENT ON COLUMN line_definitions.condition_expression IS '条件表达式（用于执行）';
COMMENT ON COLUMN line_definitions.condition_expression_view IS '条件表达式呈现（用于前端显示）';

-- ============================================
-- 12. line_instances（连接线实例表）
-- ============================================
CREATE TABLE IF NOT EXISTS line_instances (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    line_id VARCHAR(100) NOT NULL,
    source_node_id VARCHAR(100) NOT NULL,
    target_node_id VARCHAR(100) NOT NULL,
    condition_type VARCHAR(20),
    logic_type VARCHAR(10) DEFAULT 'and',
    condition_expression TEXT,
    condition_context JSONB,
    condition_result BOOLEAN,
    executed BOOLEAN NOT NULL DEFAULT false,
    evaluated_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_line_instances_workflow_instance ON line_instances(workflow_instance_id);
CREATE INDEX idx_line_instances_line_id ON line_instances(line_id);
CREATE INDEX idx_line_instances_deleted_at ON line_instances(deleted_at);

COMMENT ON TABLE line_instances IS '连接线实例表';
COMMENT ON COLUMN line_instances.logic_type IS '逻辑类型：and（与逻辑）/or（或逻辑）';
COMMENT ON COLUMN line_instances.condition_context IS '条件评估上下文（变量值）';
COMMENT ON COLUMN line_instances.condition_result IS '条件判断结果';
COMMENT ON COLUMN line_instances.executed IS '是否已执行';

-- ============================================
-- 插入系统预置节点模板
-- ============================================
INSERT INTO node_templates (type_key, type_name, category, group_type, start_node_key, end_node_key, description, icon, is_system, is_enabled, sort_order) VALUES
-- 逻辑控制类节点
('start', '开始节点', 'logic', 'single', NULL, NULL, '工作流的起始节点', 'icon-start', true, true, 1),
('end', '结束节点', 'logic', 'single', NULL, NULL, '工作流的结束节点', 'icon-end', true, true, 2),
('concurrency_start', '并发开始', 'logic', 'paired', 'concurrency_start', 'concurrency_end', '标记并发执行区域的开始', 'icon-concurrency', true, true, 3),
('concurrency_end', '并发结束', 'logic', 'paired', 'concurrency_start', 'concurrency_end', '标记并发执行区域的结束，等待所有并发分支完成', 'icon-concurrency', true, true, 4),
('loop_start', '循环开始', 'logic', 'paired', 'loop_start', 'loop_end', '标记循环区域的开始', 'icon-loop', true, true, 5),
('loop_end', '循环结束', 'logic', 'paired', 'loop_start', 'loop_end', '标记循环区域的结束，判断是否继续循环', 'icon-loop', true, true, 6),

-- 业务执行类节点
('kvm', 'KVM接入节点', 'business', 'single', NULL, NULL, '连接和控制KVM设备', 'icon-kvm', true, true, 10),
('reasoning', 'Reasoning推理节点', 'business', 'single', NULL, NULL, '调用AI模型进行推理计算', 'icon-ai', true, true, 11),
('python_script', 'PythonScript脚本节点', 'business', 'single', NULL, NULL, '执行自定义Python脚本', 'icon-python', true, true, 12),
('mqtt', 'MQTT推送节点', 'business', 'single', NULL, NULL, '向MQTT服务器推送消息', 'icon-mqtt', true, true, 13)
ON CONFLICT (type_key) DO NOTHING;

-- ============================================
-- 创建触发器：自动更新 updated_at
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为所有表创建触发器
CREATE TRIGGER update_workflows_updated_at BEFORE UPDATE ON workflows FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_workflow_instances_updated_at BEFORE UPDATE ON workflow_instances FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_workflow_logs_updated_at BEFORE UPDATE ON workflow_logs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_workflow_schedules_updated_at BEFORE UPDATE ON workflow_schedules FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_workflow_deployments_updated_at BEFORE UPDATE ON workflow_deployments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_node_templates_updated_at BEFORE UPDATE ON node_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_node_definitions_updated_at BEFORE UPDATE ON node_definitions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_node_instances_updated_at BEFORE UPDATE ON node_instances FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_variable_definitions_updated_at BEFORE UPDATE ON variable_definitions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_variable_instances_updated_at BEFORE UPDATE ON variable_instances FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_line_definitions_updated_at BEFORE UPDATE ON line_definitions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_line_instances_updated_at BEFORE UPDATE ON line_instances FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- 完成提示
-- ============================================
DO $$
BEGIN
    RAISE NOTICE '业务编排引擎数据表创建完成！';
    RAISE NOTICE '共创建 12 个数据表：';
    RAISE NOTICE '  1. workflows - 工作流定义表';
    RAISE NOTICE '  2. workflow_instances - 工作流实例表';
    RAISE NOTICE '  3. workflow_logs - 工作流日志表';
    RAISE NOTICE '  4. workflow_schedules - 工作流调度配置表';
    RAISE NOTICE '  5. workflow_deployments - 工作流部署表';
    RAISE NOTICE '  6. node_templates - 节点模板表';
    RAISE NOTICE '  7. node_definitions - 节点定义表';
    RAISE NOTICE '  8. node_instances - 节点实例表';
    RAISE NOTICE '  9. variable_definitions - 变量定义表';
    RAISE NOTICE ' 10. variable_instances - 变量实例表';
    RAISE NOTICE ' 11. line_definitions - 连接线定义表';
    RAISE NOTICE ' 12. line_instances - 连接线实例表';
    RAISE NOTICE '';
    RAISE NOTICE '已插入 10 个系统预置节点模板';
    RAISE NOTICE '已创建所有必要的索引和约束';
    RAISE NOTICE '已创建 updated_at 自动更新触发器';
END $$;
