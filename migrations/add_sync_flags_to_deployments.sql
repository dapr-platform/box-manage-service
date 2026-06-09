-- 迁移：workflow_deployments 表增加 sync_instances 和 sync_logs 字段
-- 控制盒子是否推送实例数据/日志到管理平台

BEGIN;

ALTER TABLE workflow_deployments ADD COLUMN IF NOT EXISTS sync_instances BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE workflow_deployments ADD COLUMN IF NOT EXISTS sync_logs BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN workflow_deployments.sync_instances IS '是否同步工作流实例到管理平台';
COMMENT ON COLUMN workflow_deployments.sync_logs IS '是否同步工作流日志到管理平台';

COMMIT;
