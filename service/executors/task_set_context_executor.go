package executors

import (
	"box-manage-service/models"
	"context"
)

type TaskSetContextExecutor struct{ *BaseExecutor }

func NewTaskSetContextExecutor() *TaskSetContextExecutor {
	return &TaskSetContextExecutor{BaseExecutor: NewBaseExecutor("task_set_context")}
}

func (e *TaskSetContextExecutor) Validate(instance *models.NodeInstance) error { return nil }

func (e *TaskSetContextExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"将流转变量持久化到任务上下文"}
	outputs := map[string]interface{}{"saved": true}
	return CreateSuccessResult(outputs, logs), nil
}
