/*
 * @module models/workflow_ai_session
 * @description AI 编排会话：记录一次「与 AI 对话式编排」的完整过程，
 *              使关闭弹窗/刷新页面后仍能继续在同一份编排上迭代。
 *
 * 设计要点：
 * - 按 workflow_key_name 关联（不是自增 ID），编排升级版本后会话仍然有效。
 * - 会话只在**用户保存**时落库，AI 生成过程本身不产生垃圾数据。
 * - messages 存完整对话轮次；latest_graph 存最近一次产出的编排，
 *   便于「下次打开直接接着改」而不必重跑大模型。
 */

package models

// WorkflowAISessionMessage 单条对话消息
type WorkflowAISessionMessage struct {
	// Role user / assistant
	Role string `json:"role" example:"user"`
	// Content 消息正文。user 消息是用户的需求或修改意见；
	// assistant 消息是模型返回的摘要/说明（不存完整图，图在 LatestGraph）。
	Content string `json:"content"`
	// Round 该消息所属的轮次（从 1 开始）
	Round int `json:"round,omitempty" example:"1"`
	// CreatedAt 消息时间
	CreatedAt string `json:"created_at,omitempty"`
}

// WorkflowAISession AI 编排会话
type WorkflowAISession struct {
	BaseModel
	// WorkflowKeyName 编排标识（不是自增 ID），跨版本复用会话
	WorkflowKeyName string `gorm:"type:varchar(100);not null;index" json:"workflow_key_name" example:"2669"`
	// WorkflowID 最近一次关联的编排主键，便于前端直接跳转（可空）
	WorkflowID uint `gorm:"index" json:"workflow_id,omitempty"`
	// Title 会话标题，取首次需求的前若干字
	Title string `gorm:"type:varchar(200)" json:"title" example:"检测车辆在禁停区停留超过30帧"`
	// Description 最初的业务需求原文
	Description string `gorm:"type:text" json:"description"`
	// TemplateWorkflowID 使用的参考模板
	TemplateWorkflowID uint `gorm:"index" json:"template_workflow_id,omitempty"`
	// Messages 对话轮次（JSON 数组）
	Messages JSONArr `gorm:"type:jsonb" json:"messages"`
	// LatestGraph 最近一次产出的编排（LogicFlow JSON），用于「接着改」
	LatestGraph JSONMap `gorm:"type:jsonb" json:"latest_graph,omitempty"`
	// LatestReport 最近一次的校验报告摘要
	LatestReport JSONMap `gorm:"type:jsonb" json:"latest_report,omitempty"`
	// Round 已进行的轮次数
	Round int `gorm:"default:0" json:"round"`
	// Status open / imported / closed
	Status string `gorm:"type:varchar(20);default:'open'" json:"status" example:"open"`
	// CreatedBy 创建人
	CreatedBy uint `gorm:"index" json:"created_by"`
	// UpdatedBy 最后修改人
	UpdatedBy uint `json:"updated_by"`
}

// TableName 指定表名
func (WorkflowAISession) TableName() string {
	return "workflow_ai_sessions"
}
