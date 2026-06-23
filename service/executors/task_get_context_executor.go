package executors

import (
	"box-manage-service/models"
	"context"
)

type TaskGetContextExecutor struct{ *BaseExecutor }

func NewTaskGetContextExecutor() *TaskGetContextExecutor {
	return &TaskGetContextExecutor{BaseExecutor: NewBaseExecutor("task_get_context")}
}

func (e *TaskGetContextExecutor) Validate(instance *models.NodeInstance) error { return nil }

func (e *TaskGetContextExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"从任务上下文加载持久化变量"}
	outputs := map[string]interface{}{"context": map[string]interface{}{}, "keys": 0}
	return CreateSuccessResult(outputs, logs), nil
}
