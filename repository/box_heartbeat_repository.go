/*
 * @module repository/box_heartbeat_repository
 * @description 盒子心跳Repository实现，提供心跳记录相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-001: 盒子管理功能 - 状态监控
 * @stateFlow Service层 -> BoxHeartbeatRepository -> GORM -> 数据库
 * @rules 实现心跳记录的CRUD操作、历史查询、统计分析等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-001.md, context7 GORM最佳实践
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// boxHeartbeatRepository 盒子心跳Repository实现
type boxHeartbeatRepository struct {
	BaseRepository[models.BoxHeartbeat]
	db *gorm.DB
}

// NewBoxHeartbeatRepository 创建BoxHeartbeat Repository实例
func NewBoxHeartbeatRepository(db *gorm.DB) BoxHeartbeatRepository {
	return &boxHeartbeatRepository{
		BaseRepository: newBaseRepository[models.BoxHeartbeat](db),
		db:             db,
	}
}

// FindByBoxID 根据盒子ID查找心跳记录
func (r *boxHeartbeatRepository) FindByBoxID(ctx context.Context, boxID uint, limit int) ([]*models.BoxHeartbeat, error) {
	var heartbeats []*models.BoxHeartbeat

	query := r.db.WithContext(ctx).
		Where("box_id = ?", boxID).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&heartbeats).Error
	return heartbeats, err
}

// FindByBoxIDAndDateRange 根据盒子ID和日期范围查找心跳记录
func (r *boxHeartbeatRepository) FindByBoxIDAndDateRange(ctx context.Context, boxID uint, startDate, endDate string) ([]*models.BoxHeartbeat, error) {
	var heartbeats []*models.BoxHeartbeat

	err := r.db.WithContext(ctx).
		Where("box_id = ? AND timestamp >= ? AND timestamp <= ?", boxID, startDate, endDate).
		Order("timestamp ASC").
		Find(&heartbeats).Error

	return heartbeats, err
}

// GetLatestHeartbeat 获取盒子的最新心跳记录
func (r *boxHeartbeatRepository) GetLatestHeartbeat(ctx context.Context, boxID uint) (*models.BoxHeartbeat, error) {
	var heartbeat models.BoxHeartbeat

	err := r.db.WithContext(ctx).
		Where("box_id = ?", boxID).
		Order("timestamp DESC").
		First(&heartbeat).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &heartbeat, nil
}

// GetHeartbeatStatistics 获取心跳统计信息
func (r *boxHeartbeatRepository) GetHeartbeatStatistics(ctx context.Context, boxID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 基础查询
	baseQuery := r.db.WithContext(ctx).Model(&models.BoxHeartbeat{})
	if boxID > 0 {
		baseQuery = baseQuery.Where("box_id = ?", boxID)
	}

	// 总心跳记录数
	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total_heartbeats"] = totalCount

	// 最近24小时心跳数
	last24Hours := time.Now().Add(-24 * time.Hour)
	var last24HoursCount int64
	if err := baseQuery.Where("timestamp >= ?", last24Hours).Count(&last24HoursCount).Error; err != nil {
		return nil, err
	}
	stats["last_24_hours"] = last24HoursCount

	// 最近1小时心跳数
	lastHour := time.Now().Add(-1 * time.Hour)
	var lastHourCount int64
	if err := baseQuery.Where("timestamp >= ?", lastHour).Count(&lastHourCount).Error; err != nil {
		return nil, err
	}
	stats["last_hour"] = lastHourCount

	// 状态分布
	statusStats := make(map[string]int64)
	type statusCount struct {
		Status models.BoxStatus `json:"status"`
		Count  int64            `json:"count"`
	}

	var statusResults []statusCount
	err := baseQuery.
		Select("status, count(*) as count").
		Group("status").
		Find(&statusResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range statusResults {
		statusStats[string(result.Status)] = result.Count
	}
	stats["status_distribution"] = statusStats

	// 心跳间隔统计（最近100条记录）
	if boxID > 0 {
		var intervals []float64
		type intervalResult struct {
			Interval float64 `json:"interval"`
		}

		var intervalResults []intervalResult
		err = r.db.WithContext(ctx).
			Raw(`
				SELECT EXTRACT(EPOCH FROM (timestamp - LAG(timestamp) OVER (ORDER BY timestamp))) as interval
				FROM box_heartbeats 
				WHERE box_id = ? AND timestamp IS NOT NULL
				ORDER BY timestamp DESC 
				LIMIT 100
			`, boxID).
			Find(&intervalResults).Error

		if err == nil {
			for _, result := range intervalResults {
				if result.Interval > 0 {
					intervals = append(intervals, result.Interval)
				}
			}

			if len(intervals) > 0 {
				// 计算平均间隔
				var sum float64
				for _, interval := range intervals {
					sum += interval
				}
				avgInterval := sum / float64(len(intervals))
				stats["avg_interval_seconds"] = avgInterval

				// 计算最大和最小间隔
				maxInterval := intervals[0]
				minInterval := intervals[0]
				for _, interval := range intervals {
					if interval > maxInterval {
						maxInterval = interval
					}
					if interval < minInterval {
						minInterval = interval
					}
				}
				stats["max_interval_seconds"] = maxInterval
				stats["min_interval_seconds"] = minInterval
			}
		}
	}

	return stats, nil
}

// CleanupOldHeartbeats 清理旧的心跳记录
func (r *boxHeartbeatRepository) CleanupOldHeartbeats(ctx context.Context, daysToKeep int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	return r.db.WithContext(ctx).
		Where("timestamp < ?", cutoffDate).
		Delete(&models.BoxHeartbeat{}).Error
}

// GetHeartbeatTrend 获取心跳趋势数据
func (r *boxHeartbeatRepository) GetHeartbeatTrend(ctx context.Context, boxID uint, hours int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	startTime := time.Now().Add(time.Duration(-hours) * time.Hour)

	type trendData struct {
		Hour   string `json:"hour"`
		Count  int64  `json:"count"`
		Status string `json:"status"`
	}

	var trendResults []trendData
	query := `
		SELECT 
			DATE_TRUNC('hour', timestamp) as hour,
			COUNT(*) as count,
			status
		FROM box_heartbeats 
		WHERE timestamp >= ? `

	args := []interface{}{startTime}

	if boxID > 0 {
		query += " AND box_id = ?"
		args = append(args, boxID)
	}

	query += `
		GROUP BY DATE_TRUNC('hour', timestamp), status
		ORDER BY hour ASC
	`

	err := r.db.WithContext(ctx).Raw(query, args...).Find(&trendResults).Error
	if err != nil {
		return nil, err
	}

	// 转换为map格式
	for _, result := range trendResults {
		results = append(results, map[string]interface{}{
			"hour":   result.Hour,
			"count":  result.Count,
			"status": result.Status,
		})
	}

	return results, nil
}

// CreateHeartbeat 创建心跳记录
func (r *boxHeartbeatRepository) CreateHeartbeat(ctx context.Context, boxID uint, status models.BoxStatus, resources models.Resources) error {
	heartbeat := &models.BoxHeartbeat{
		BoxID:     boxID,
		Status:    status,
		Resources: resources,
		Timestamp: time.Now(),
	}

	return r.db.WithContext(ctx).Create(heartbeat).Error
}

// GetBoxesHeartbeatStatus 获取所有盒子的心跳状态
func (r *boxHeartbeatRepository) GetBoxesHeartbeatStatus(ctx context.Context) (map[uint]map[string]interface{}, error) {
	result := make(map[uint]map[string]interface{})

	// 获取每个盒子的最新心跳
	type boxHeartbeatStatus struct {
		BoxID     uint             `json:"box_id"`
		Status    models.BoxStatus `json:"status"`
		Timestamp time.Time        `json:"timestamp"`
		Resources models.Resources `json:"resources"`
	}

	var statuses []boxHeartbeatStatus
	err := r.db.WithContext(ctx).
		Table("box_heartbeats").
		Select("DISTINCT ON (box_id) box_id, status, timestamp, resources").
		Order("box_id, timestamp DESC").
		Find(&statuses).Error

	if err != nil {
		return nil, err
	}

	for _, status := range statuses {
		result[status.BoxID] = map[string]interface{}{
			"status":    status.Status,
			"timestamp": status.Timestamp,
			"resources": status.Resources,
			"is_online": time.Since(status.Timestamp) < 5*time.Minute,
		}
	}

	return result, nil
}

// GetHeartbeatHistory 获取心跳历史记录（用于图表展示）
func (r *boxHeartbeatRepository) GetHeartbeatHistory(ctx context.Context, boxID uint, startTime, endTime time.Time, interval string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// 根据间隔类型调整SQL
	var timeFormat string
	switch interval {
	case "minute":
		timeFormat = "DATE_TRUNC('minute', timestamp)"
	case "hour":
		timeFormat = "DATE_TRUNC('hour', timestamp)"
	case "day":
		timeFormat = "DATE_TRUNC('day', timestamp)"
	default:
		timeFormat = "DATE_TRUNC('hour', timestamp)"
	}

	type historyData struct {
		Time   time.Time `json:"time"`
		Count  int64     `json:"count"`
		Status string    `json:"status"`
	}

	var historyResults []historyData
	query := fmt.Sprintf(`
		SELECT 
			%s as time,
			COUNT(*) as count,
			status
		FROM box_heartbeats 
		WHERE box_id = ? AND timestamp BETWEEN ? AND ?
		GROUP BY %s, status
		ORDER BY time ASC
	`, timeFormat, timeFormat)

	err := r.db.WithContext(ctx).Raw(query, boxID, startTime, endTime).Find(&historyResults).Error
	if err != nil {
		return nil, err
	}

	// 转换为map格式
	for _, result := range historyResults {
		results = append(results, map[string]interface{}{
			"time":   result.Time,
			"count":  result.Count,
			"status": result.Status,
		})
	}

	return results, nil
}

// BulkCreateHeartbeats 批量创建心跳记录
func (r *boxHeartbeatRepository) BulkCreateHeartbeats(ctx context.Context, heartbeats []*models.BoxHeartbeat) error {
	if len(heartbeats) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).CreateInBatches(heartbeats, 100).Error
}
