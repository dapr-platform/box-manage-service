/*
 * @module service/executors/executor_factory
 * @description 节点执行器工厂
 * @architecture 工厂模式 - 根据节点类型创建对应的执行器
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 执行器创建：获取节点类型 -> 创建对应执行器
 * @rules 所有执行器必须通过工厂创建
 */

package executors

import (
	"fmt"
	"sync"
)

// ExecutorFactory 执行器工厂
type ExecutorFactory struct {
	executors map[string]NodeExecutor
	mu        sync.RWMutex
}

var (
	factoryInstance *ExecutorFactory
	factoryOnce     sync.Once
)

// GetExecutorFactory 获取执行器工厂单例
func GetExecutorFactory() *ExecutorFactory {
	factoryOnce.Do(func() {
		factoryInstance = &ExecutorFactory{
			executors: make(map[string]NodeExecutor),
		}
		// 注册默认执行器
		factoryInstance.registerDefaultExecutors()
	})
	return factoryInstance
}

// registerDefaultExecutors 注册默认执行器
func (f *ExecutorFactory) registerDefaultExecutors() {
	// 控制节点
	f.Register("start", NewStartExecutor())
	f.Register("end", NewEndExecutor())
	f.Register("condition", NewConditionExecutor())

	// 脚本节点
	f.Register("python_script", NewPythonScriptExecutor())

	// AI推理节点
	f.Register("kvm", NewKVMExecutor())
	f.Register("reasoning", NewReasoningExecutor())

	// 通信节点
	f.Register("mqtt", NewMQTTExecutor())
	f.Register("http_request", NewHTTPRequestExecutor())

	// 数据处理节点
	f.Register("data_transform", NewDataTransformExecutor())

	// 工具节点
	f.Register("delay", NewDelayExecutor())

	// 成对节点 - 并发
	f.Register("concurrency_start", NewParallelGatewayStartExecutor())
	f.Register("concurrency_end", NewParallelGatewayEndExecutor())

	// 成对节点 - 循环
	f.Register("loop_start", NewLoopStartExecutor())
	f.Register("loop_end", NewLoopEndExecutor())
}

// Register 注册执行器
func (f *ExecutorFactory) Register(nodeType string, executor NodeExecutor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executors[nodeType] = executor
}

// GetExecutor 获取执行器
func (f *ExecutorFactory) GetExecutor(nodeType string) (NodeExecutor, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	executor, ok := f.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("未找到节点类型 %s 的执行器", nodeType)
	}

	return executor, nil
}

// ListExecutors 列出所有已注册的执行器
func (f *ExecutorFactory) ListExecutors() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.executors))
	for nodeType := range f.executors {
		types = append(types, nodeType)
	}
	return types
}
