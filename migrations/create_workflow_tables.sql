-- 业务编排引擎数据库迁移脚本
-- 创建工作流相关表
-- 版本: 1.0.0
-- 日期: 2025-01-26

-- ============================================
-- 1. 工作流定义表
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
CREATE INDEX idx_workflows_category ON workflows(category);
CREATE INDEX idx_workflows_deleted_at ON workflows(deleted_at);

COMMENT ON TABLE workflows IS '工作流定义表';
COMMENT ON COLUMN workflows.key_name IS '工作流标识';
COMMENT ON COLUMN workflows.version IS '版本号';
COMMENT ON COLUMN workflows.structure_json IS '工作流结构JSON（冗余字段）';

-- ============================================
-- 2. 节点模板表
-- ============================================
CREATE TABLE IF NOT EXISTS node_templates (
    id SERIAL PRIMARY KEY,
    key_name VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    category VARCHAR(50),
    group_type VARCHAR(20) NOT NULL DEFAULT 'single',
    description TEXT,
    icon VARCHAR(255),
    config_schema JSONB,
    input_schema JSONB,
    output_schema JSONB,
    default_variables JSONB,
    default_config JSONB,
    start_node_key VARCHAR(50),
    end_node_key VARCHAR(50),
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_node_templates_type ON node_templates(type);
CREATE INDEX idx_node_templates_category ON node_templates(category);
CREATE INDEX idx_node_templates_group_type ON node_templates(group_type);
CREATE INDEX idx_node_templates_deleted_at ON node_templates(deleted_at);

COMMENT ON TABLE node_templates IS '节点模板表';
COMMENT ON COLUMN node_templates.config_schema IS '配置参数JSON Schema';
COMMENT ON COLUMN node_templates.default_variables IS '预定义的变量配置，拖入工作流时自动复制到variable_definitions';
COMMENT ON COLUMN node_templates.group_type IS '节点组类型：single(单节点)/paired(成对节点)/container(容器节点)';
COMMENT ON COLUMN node_templates.start_node_key IS '成对节点的开始节点key（仅paired类型使用）';
COMMENT ON COLUMN node_templates.end_node_key IS '成对节点的结束节点key（仅paired类型使用）';

-- ============================================
-- 3. 节点定义表
-- ============================================
CREATE TABLE IF NOT EXISTS node_definitions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_template_id INTEGER NOT NULL REFERENCES node_templates(id),
    node_id VARCHAR(100) NOT NULL,
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    group_type VARCHAR(20) NOT NULL DEFAULT 'single',
    start_node_key VARCHAR(50),
    end_node_key VARCHAR(50),
    position_x DOUBLE PRECISION,
    position_y DOUBLE PRECISION,
    config JSONB,
    python_script TEXT,
    inputs JSONB,
    outputs JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_node_definitions_workflow_id ON node_definitions(workflow_id);
CREATE INDEX idx_node_definitions_type ON node_definitions(type);
CREATE INDEX idx_node_definitions_deleted_at ON node_definitions(deleted_at);

COMMENT ON TABLE node_definitions IS '节点定义表';
COMMENT ON COLUMN node_definitions.node_id IS '节点ID（在structure_json中的ID）';
COMMENT ON COLUMN node_definitions.group_type IS '节点组类型：single(单节点)/paired(成对节点)/container(容器节点)';
COMMENT ON COLUMN node_definitions.start_node_key IS '成对节点的开始节点key（仅paired类型使用）';
COMMENT ON COLUMN node_definitions.end_node_key IS '成对节点的结束节点key（仅paired类型使用）';

-- ============================================
-- 4. 变量定义表
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
CREATE INDEX idx_variable_definitions_node_template_id ON variable_definitions(node_template_id);
CREATE INDEX idx_variable_definitions_deleted_at ON variable_definitions(deleted_at);

COMMENT ON TABLE variable_definitions IS '变量定义表';
COMMENT ON COLUMN variable_definitions.node_id IS '节点ID，为空表示全局变量';
COMMENT ON COLUMN variable_definitions.node_template_id IS '来源节点模板ID，用于追溯参数定义来源';
COMMENT ON COLUMN variable_definitions.direction IS '变量方向：input/output';

-- ============================================
-- 5. 连接线定义表
-- ============================================
CREATE TABLE IF NOT EXISTS line_definitions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    line_id VARCHAR(100) NOT NULL,
    source_node_def_id INTEGER NOT NULL REFERENCES node_definitions(id) ON DELETE CASCADE,
    target_node_def_id INTEGER NOT NULL REFERENCES node_definitions(id) ON DELETE CASCADE,
    condition_type VARCHAR(50),
    condition_expression TEXT,
    condition_expression_view TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_line_definitions_workflow_id ON line_definitions(workflow_id);
CREATE INDEX idx_line_definitions_source ON line_definitions(source_node_def_id);
CREATE INDEX idx_line_definitions_target ON line_definitions(target_node_def_id);
CREATE INDEX idx_line_definitions_deleted_at ON line_definitions(deleted_at);

COMMENT ON TABLE line_definitions IS '连接线定义表';

-- ============================================
-- 6. 工作流实例表
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_instances (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id),
    box_id INTEGER REFERENCES boxes(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    progress DOUBLE PRECISION NOT NULL DEFAULT 0,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    error_message TEXT,
    triggered_by VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_instances_workflow_id ON workflow_instances(workflow_id);
CREATE INDEX idx_workflow_instances_box_id ON workflow_instances(box_id);
CREATE INDEX idx_workflow_instances_status ON workflow_instances(status);
CREATE INDEX idx_workflow_instances_deleted_at ON workflow_instances(deleted_at);

COMMENT ON TABLE workflow_instances IS '工作流实例表';

-- ============================================
-- 7. 节点实例表
-- ============================================
CREATE TABLE IF NOT EXISTS node_instances (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    node_def_id INTEGER NOT NULL REFERENCES node_definitions(id),
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    output JSONB,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_node_instances_workflow_instance_id ON node_instances(workflow_instance_id);
CREATE INDEX idx_node_instances_status ON node_instances(status);
CREATE INDEX idx_node_instances_deleted_at ON node_instances(deleted_at);

COMMENT ON TABLE node_instances IS '节点实例表';

-- ============================================
-- 8. 变量实例表
-- ============================================
CREATE TABLE IF NOT EXISTS variable_instances (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    variable_def_id INTEGER NOT NULL REFERENCES variable_definitions(id),
    key_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    value JSONB,
    scope VARCHAR(50) NOT NULL DEFAULT 'global',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_variable_instances_workflow_instance_id ON variable_instances(workflow_instance_id);
CREATE INDEX idx_variable_instances_deleted_at ON variable_instances(deleted_at);

COMMENT ON TABLE variable_instances IS '变量实例表';

-- ============================================
-- 9. 连接线实例表
-- ============================================
CREATE TABLE IF NOT EXISTS line_instances (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    line_def_id INTEGER NOT NULL REFERENCES line_definitions(id),
    source_node_inst_id INTEGER NOT NULL REFERENCES node_instances(id) ON DELETE CASCADE,
    target_node_inst_id INTEGER NOT NULL REFERENCES node_instances(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    condition_result BOOLEAN,
    evaluated_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_line_instances_workflow_instance_id ON line_instances(workflow_instance_id);
CREATE INDEX idx_line_instances_deleted_at ON line_instances(deleted_at);

COMMENT ON TABLE line_instances IS '连接线实例表';

-- ============================================
-- 10. 工作流日志表
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_logs (
    id SERIAL PRIMARY KEY,
    workflow_instance_id INTEGER NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    node_instance_id INTEGER REFERENCES node_instances(id) ON DELETE CASCADE,
    level VARCHAR(20) NOT NULL,
    type VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_logs_workflow_instance_id ON workflow_logs(workflow_instance_id);
CREATE INDEX idx_workflow_logs_node_instance_id ON workflow_logs(node_instance_id);
CREATE INDEX idx_workflow_logs_level ON workflow_logs(level);
CREATE INDEX idx_workflow_logs_type ON workflow_logs(type);
CREATE INDEX idx_workflow_logs_timestamp ON workflow_logs(timestamp);
CREATE INDEX idx_workflow_logs_deleted_at ON workflow_logs(deleted_at);

COMMENT ON TABLE workflow_logs IS '工作流日志表';

-- ============================================
-- 11. 工作流调度配置表
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_schedules (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    cron_expression VARCHAR(100),
    timezone VARCHAR(50) DEFAULT 'Asia/Shanghai',
    input_variables JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'enabled',
    last_executed_at TIMESTAMP,
    next_execution_at TIMESTAMP,
    execution_count INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER,
    updated_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_schedules_workflow_id ON workflow_schedules(workflow_id);
CREATE INDEX idx_workflow_schedules_type ON workflow_schedules(type);
CREATE INDEX idx_workflow_schedules_status ON workflow_schedules(status);
CREATE INDEX idx_workflow_schedules_deleted_at ON workflow_schedules(deleted_at);

COMMENT ON TABLE workflow_schedules IS '工作流调度配置表';

-- ============================================
-- 12. 工作流部署表
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_deployments (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL REFERENCES workflows(id),
    box_id INTEGER NOT NULL REFERENCES boxes(id),
    workflow_version INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    deployed_at TIMESTAMP,
    rolled_back_at TIMESTAMP,
    previous_version INTEGER,
    error_message TEXT,
    deployed_by INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workflow_deployments_workflow_id ON workflow_deployments(workflow_id);
CREATE INDEX idx_workflow_deployments_box_id ON workflow_deployments(box_id);
CREATE INDEX idx_workflow_deployments_status ON workflow_deployments(status);
CREATE INDEX idx_workflow_deployments_deleted_at ON workflow_deployments(deleted_at);
CREATE UNIQUE INDEX idx_workflow_deployments_workflow_box ON workflow_deployments(workflow_id, box_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE workflow_deployments IS '工作流部署表';

-- ============================================
-- 插入默认节点模板
-- ============================================
INSERT INTO node_templates (key_name, name, type, category, group_type, start_node_key, end_node_key, description, icon, is_enabled) VALUES
('start', '开始', 'start', 'control', 'single', NULL, NULL, '工作流开始节点', 'start-icon', true),
('end', '结束', 'end', 'control', 'single', NULL, NULL, '工作流结束节点', 'end-icon', true),
('concurrency_start', '并发开始', 'concurrency_start', 'control', 'paired', 'concurrency_start', 'concurrency_end', '并发控制开始节点', 'concurrency-icon', true),
('concurrency_end', '并发结束', 'concurrency_end', 'control', 'paired', 'concurrency_start', 'concurrency_end', '并发控制结束节点', 'concurrency-icon', true),
('loop_start', '循环开始', 'loop_start', 'control', 'paired', 'loop_start', 'loop_end', '循环控制开始节点', 'loop-icon', true),
('loop_end', '循环结束', 'loop_end', 'control', 'paired', 'loop_start', 'loop_end', '循环控制结束节点', 'loop-icon', true),
('python_script', 'Python脚本', 'python_script', 'script', 'single', NULL, NULL, 'Python脚本执行节点', 'python-icon', true),
('kvm', 'KVM推理', 'kvm', 'ai', 'single', NULL, NULL, 'KVM模型推理节点', 'kvm-icon', true),
('reasoning', 'Reasoning推理', 'reasoning', 'ai', 'single', NULL, NULL, 'Reasoning模型推理节点', 'reasoning-icon', true),
('mqtt', 'MQTT', 'mqtt', 'communication', 'single', NULL, NULL, 'MQTT消息发送节点', 'mqtt-icon', true),
('http_request', 'HTTP请求', 'http_request', 'communication', 'single', NULL, NULL, 'HTTP请求节点', 'http-icon', true),
('delay', '延时', 'delay', 'control', 'single', NULL, NULL, '延时等待节点', 'delay-icon', true),
('data_transform', '数据转换', 'data_transform', 'data', 'single', NULL, NULL, '数据转换处理节点', 'transform-icon', true),
('condition', '条件判断', 'condition', 'control', 'single', NULL, NULL, '条件判断节点', 'condition-icon', true)
ON CONFLICT (key_name) DO NOTHING;
