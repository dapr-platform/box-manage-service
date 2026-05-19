/*
 * @module service/node_template_sync_service
 * @description 节点模板下发版本管理服务（单例）。任何节点模板增删改后调用 Bump()
 * 自增版本号并持久化；盒子心跳时通过 Current() 与上报版本比对决定是否触发下发。
 * @architecture 服务层
 * @stateFlow Init(读 DB 现有 version) -> NodeTemplateService 增删改 -> Bump(原子+1+持久化)
 *            -> ProcessHeartbeat 比对 Current() 与 LocalTemplatesVersion -> 触发下发
 * @rules 版本号单调递增；持久化失败不阻塞业务（仅日志）；并发安全。
 * @dependencies gorm.io/gorm, models.NodeTemplateMeta
 */

package service

import (
	"box-manage-service/models"
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

const nodeTemplateMetaRowID uint = 1

// NodeTemplateSyncService 节点模板下发版本管理（单例）
type NodeTemplateSyncService struct {
	version atomic.Int64
	db      *gorm.DB
}

// NewNodeTemplateSyncService 创建实例
func NewNodeTemplateSyncService(db *gorm.DB) *NodeTemplateSyncService {
	return &NodeTemplateSyncService{db: db}
}

// Init 启动时从 DB 加载版本号；不存在则插入 id=1 的初始行
// 初始 version 优先取 max(node_templates.updated_at).Unix() 以便首次启动时
// 与已有数据的"自然版本"对齐，避免历史盒子无意义重新下发
func (s *NodeTemplateSyncService) Init(ctx context.Context) error {
	if s.db == nil {
		return errors.New("NodeTemplateSyncService: db is nil")
	}

	var meta models.NodeTemplateMeta
	err := s.db.WithContext(ctx).First(&meta, nodeTemplateMetaRowID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 计算初始版本：以现有节点模板最大 updated_at 的 unix 秒为种子
		var seed int64
		var maxUpdated *time.Time
		if e := s.db.WithContext(ctx).
			Model(&models.NodeTemplate{}).
			Select("MAX(updated_at)").
			Scan(&maxUpdated).Error; e == nil && maxUpdated != nil {
			seed = maxUpdated.Unix()
		}
		meta = models.NodeTemplateMeta{
			ID:        nodeTemplateMetaRowID,
			Version:   seed,
			UpdatedAt: time.Now(),
		}
		if e := s.db.WithContext(ctx).Create(&meta).Error; e != nil {
			return e
		}
		log.Printf("[NodeTemplateSync] meta 初始化完成, version=%d", seed)
	}

	s.version.Store(meta.Version)
	log.Printf("[NodeTemplateSync] 启动版本号: %d", meta.Version)
	return nil
}

// Bump 自增版本号并持久化，返回新版本号
// 持久化失败仅记录日志，不影响业务流程
func (s *NodeTemplateSyncService) Bump() int64 {
	newVer := s.version.Add(1)
	if s.db == nil {
		return newVer
	}
	if err := s.db.Model(&models.NodeTemplateMeta{}).
		Where("id = ?", nodeTemplateMetaRowID).
		Updates(map[string]interface{}{
			"version":    newVer,
			"updated_at": time.Now(),
		}).Error; err != nil {
		log.Printf("[NodeTemplateSync] 持久化版本号失败 version=%d err=%v", newVer, err)
	}
	return newVer
}

// Current 返回当前版本号（线程安全）
func (s *NodeTemplateSyncService) Current() int64 {
	return s.version.Load()
}
