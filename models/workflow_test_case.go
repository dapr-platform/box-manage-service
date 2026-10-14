/*
 * @module models/workflow_test_case
 * @description 编排测试用例模型。用例按 workflow_key_name 关联，
 *              编排升级版本后用例仍可复用，作为回归测试。
 */

package models

// WorkflowTestCase 编排测试用例
// @Description 一组逐帧输入与断言，用于前端仿真测试与真机测试
type WorkflowTestCase struct {
	BaseModel
	// WorkflowKeyName 编排标识（不是自增 ID），跨版本复用用例
	WorkflowKeyName string `gorm:"type:varchar(100);not null;index" json:"workflow_key_name" example:"video_analysis_workflow"`
	// Name 用例名称
	Name string `gorm:"type:varchar(200);not null" json:"name" example:"持续有人触发一次报警"`
	// Description 用例说明
	Description string `gorm:"type:text" json:"description"`
	// Content 用例 JSON：{ name, description, initial_context, frames: [...] }
	Content   string `gorm:"type:text;not null" json:"content"`
	CreatedBy uint   `gorm:"index" json:"created_by" example:"1"`
	UpdatedBy uint   `json:"updated_by" example:"1"`
}

// TableName 指定表名
func (WorkflowTestCase) TableName() string {
	return "workflow_test_cases"
}

// TestCaseFrame 单个测试帧
// @Description 一帧的输入、重复次数与断言
type TestCaseFrame struct {
	// Label 帧标签，出现在失败信息里
	Label string `json:"label,omitempty" example:"继续有人 10 帧，最后一帧首次报警"`
	// Repeat 同样输入重复的帧数，默认 1
	Repeat int `json:"repeat,omitempty" example:"10"`
	// ExpectEach 为 true 时每一帧都判断断言；默认只在最后一帧判断
	ExpectEach bool `json:"expect_each,omitempty"`
	// Inputs 开始节点的输入变量
	Inputs map[string]interface{} `json:"inputs,omitempty"`
	// Mocks 仅仿真使用：为不能仿真的节点提供本帧输出
	Mocks map[string]interface{} `json:"mocks,omitempty"`
	// Expect 断言列表
	Expect []TestCaseExpectation `json:"expect,omitempty"`
}

// TestCaseExpectation 一条断言
type TestCaseExpectation struct {
	// Path <节点id>.<输出key>，可用 . 访问嵌套字段
	Path string `json:"path,omitempty" example:"python_script-005.send_alarm"`
	// Op 比较运算：eq/ne/gt/gte/lt/lte/contains/exists/not_exists
	Op string `json:"op,omitempty" example:"eq"`
	// Value 期望值
	Value interface{} `json:"value,omitempty"`
	// Node 判断某节点是否被执行时使用
	Node string `json:"node,omitempty"`
	// Executed 与 Node 搭配，判断该节点本帧是否执行
	Executed *bool `json:"executed,omitempty"`
}

// WorkflowTestCaseContent 用例 JSON 的完整结构
type WorkflowTestCaseContent struct {
	Name           string                 `json:"name,omitempty"`
	Description    string                 `json:"description,omitempty"`
	InitialContext map[string]interface{} `json:"initial_context,omitempty"`
	Frames         []TestCaseFrame        `json:"frames"`
}

// WorkflowTestFrameResult 单帧执行结果（仿真与真机一致）
// @Description 两种测试产出的帧结果结构相同，前端断言与展示无需区分来源
type WorkflowTestFrameResult struct {
	Index         int                               `json:"index"`
	Label         string                            `json:"label,omitempty"`
	Status        string                            `json:"status" example:"success"`
	ExecutedNodes []string                          `json:"executed_nodes"`
	NodeOutputs   map[string]map[string]interface{} `json:"node_outputs"`
	NodeInputs    map[string]map[string]interface{} `json:"node_inputs"`
	Errors        []string                          `json:"errors"`
}

// WorkflowTestCaseAIRequest AI 生成用例请求
type WorkflowTestCaseAIRequest struct {
	// Description 业务需求原文（用例从需求出发，而不是从编排出发）
	Description string `json:"description"`
	// Assumptions AI 编排阶段采用的假设，可选
	Assumptions []string `json:"assumptions,omitempty"`
	// WorkflowID 编排 ID，用于读取节点与输出名称
	WorkflowID uint `json:"workflow_id"`
}

// WorkflowTestCaseAIDraft AI 生成用例的草稿结果
type WorkflowTestCaseAIDraft struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Content     WorkflowTestCaseContent `json:"content"`
	Warnings    []string                `json:"warnings,omitempty"`
	Model       string                  `json:"model,omitempty"`
}
