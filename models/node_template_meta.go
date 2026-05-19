/*
 * @module models/node_template_meta
 * @description 节点模板同步元信息（单行表，用于持久化下发版本号）
 * @architecture 数据模型层
 * @stateFlow Service 增删改 -> 版本号自增 -> 心跳响应携带新版本与列表 -> 盒子端落库
 * @rules 仅一行（id=1），版本号单调递增；用于盒子端通过版本号比对决定是否拉取
 */

package models

import "time"

// NodeTemplateMeta 节点模板同步元信息（单行表）
// 仅维护一行 id=1，记录全局节点模板版本号。任何 NodeTemplate / VariableDefinition
// 增删改之后需自增 Version，盒子端心跳时上报本地版本号，与之不一致即触发下发。
type NodeTemplateMeta struct {
	ID        uint      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Version   int64     `gorm:"not null;default:0" json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (NodeTemplateMeta) TableName() string {
	return "node_template_meta"
}
