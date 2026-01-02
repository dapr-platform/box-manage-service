/*
 * @module models/heartbeat
 * @description 盒子心跳数据模型定义，用于接收盒子端POST的心跳数据
 * @architecture 数据模型层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 心跳接收 -> 数据解析 -> 状态更新 -> 任务同步
 * @rules 定义心跳数据结构，支持设备信息、任务状态、系统资源等数据
 * @dependencies 无
 * @refs req_1222.md
 */

package models

// BoxHeartbeatRequest 盒子心跳请求数据结构
// 对应盒子端每10秒POST到 /api/v1/box-client/heartbeat 的数据
type BoxHeartbeatRequest struct {
	Timestamp int64               `json:"timestamp"`        // 时间戳（毫秒）
	Version   string              `json:"version"`          // 盒子软件版本
	Service   string              `json:"service"`          // 服务名称
	Device    HeartbeatDeviceInfo `json:"device"`           // 设备信息
	ApiKey    string              `json:"api_key"`          // API密钥
	Port      int                 `json:"port"`             // 服务端口
	Tasks     HeartbeatTasksInfo  `json:"tasks"`            // 任务信息
	System    HeartbeatSystemInfo `json:"system"`           // 系统信息
	IPAddress string              `json:"ip_address,omitempty"` // IP地址（由服务端填充）
}

// HeartbeatDeviceInfo 心跳中的设备信息
type HeartbeatDeviceInfo struct {
	DeviceFingerprint string `json:"device_fingerprint"` // 设备指纹
	LicenseID         string `json:"license_id"`         // 许可证ID
	Edition           string `json:"edition"`            // 版本类型（如commercial）
	IsValid           bool   `json:"is_valid"`           // 许可证是否有效
}

// HeartbeatTasksInfo 心跳中的任务信息
type HeartbeatTasksInfo struct {
	TotalTasks   int                     `json:"total_tasks"`   // 任务总数
	RunningTasks int                     `json:"running_tasks"` // 运行中任务数
	Tasks        []HeartbeatTaskStatInfo `json:"tasks"`         // 任务统计列表
}

// HeartbeatTaskStatInfo 心跳中的单个任务统计信息
type HeartbeatTaskStatInfo struct {
	TaskID string                `json:"task_id"` // 任务ID
	Stats  HeartbeatTaskStatsData `json:"stats"`   // 任务统计数据
}

// HeartbeatTaskStatsData 心跳中的任务统计数据
type HeartbeatTaskStatsData struct {
	Status              string  `json:"status"`                // 任务状态：RUNNING, STOPPED, ERROR等
	StartTime           int64   `json:"start_time"`            // 启动时间（毫秒时间戳）
	StopTime            int64   `json:"stop_time"`             // 停止时间（毫秒时间戳，0表示未停止）
	TotalFrames         int64   `json:"total_frames"`          // 处理的总帧数
	InferenceCount      int64   `json:"inference_count"`       // 推理次数
	ForwardSuccess      int64   `json:"forward_success"`       // 转发成功次数
	ForwardFailed       int64   `json:"forward_failed"`        // 转发失败次数
	TotalInferenceTimeMs int64   `json:"total_inference_time_ms"` // 总推理时间（毫秒）
	AvgInferenceTimeMs  float64 `json:"avg_inference_time_ms"`   // 平均推理时间（毫秒）
	AvgFPS              float64 `json:"avg_fps"`                 // 平均帧率
	LastError           string  `json:"last_error"`              // 最后错误信息
}

// HeartbeatSystemInfo 心跳中的系统信息
type HeartbeatSystemInfo struct {
	Uptime                int64                  `json:"uptime"`                   // 运行时间（秒）
	TimeSinceUptime       string                 `json:"time_since_uptime"`        // 启动时间
	Procs                 int64                  `json:"procs"`                    // 进程数
	CPUTotal              int                    `json:"cpu_total"`                // CPU核心数
	CPUUsedPercent        float64                `json:"cpu_used_percent"`         // CPU使用率
	CPUUsed               float64                `json:"cpu_used"`                 // 使用的CPU
	CPUPercent            []float64              `json:"cpu_percent"`              // 各核心使用率
	Load1                 float64                `json:"load1"`                    // 1分钟负载
	Load5                 float64                `json:"load5"`                    // 5分钟负载
	Load15                float64                `json:"load15"`                   // 15分钟负载
	LoadUsagePercent      float64                `json:"load_usage_percent"`       // 负载使用率
	MemoryTotal           int64                  `json:"memory_total"`             // 总内存（字节）
	MemoryAvailable       int64                  `json:"memory_available"`         // 可用内存（字节）
	MemoryUsed            int64                  `json:"memory_used"`              // 已用内存（字节）
	MemoryUsedPercent     float64                `json:"memory_used_percent"`      // 内存使用率
	SwapMemoryTotal       int64                  `json:"swap_memory_total"`        // 交换内存总量
	SwapMemoryAvailable   int64                  `json:"swap_memory_available"`    // 交换内存可用量
	SwapMemoryUsed        int64                  `json:"swap_memory_used"`         // 交换内存已用量
	SwapMemoryUsedPercent float64                `json:"swap_memory_used_percent"` // 交换内存使用率
	NPUMemoryTotal        float64                `json:"npu_memory_total"`         // NPU内存总量
	NPUMemoryUsed         float64                `json:"npu_memory_used"`          // NPU内存已用量
	VPPMemoryTotal        float64                `json:"vpp_memory_total"`         // VPP内存总量
	VPPMemoryUsed         float64                `json:"vpp_memory_used"`          // VPP内存已用量
	VPUMemoryTotal        float64                `json:"vpu_memory_total"`         // VPU内存总量
	VPUMemoryUsed         float64                `json:"vpu_memory_used"`          // VPU内存已用量
	TPUUsed               float64                `json:"tpu_used"`                 // TPU使用率
	IOReadBytes           int64                  `json:"io_read_bytes"`            // IO读取字节数
	IOWriteBytes          int64                  `json:"io_write_bytes"`           // IO写入字节数
	IOCount               int64                  `json:"io_count"`                 // IO操作次数
	IOReadTime            int64                  `json:"io_read_time"`             // IO读取时间
	IOWriteTime           int64                  `json:"io_write_time"`            // IO写入时间
	NetBytesSent          int64                  `json:"net_bytes_sent"`           // 网络发送字节数
	NetBytesRecv          int64                  `json:"net_bytes_recv"`           // 网络接收字节数
	BoardTemperature      float64                `json:"board_temperature"`        // 板载温度
	CoreTemperature       float64                `json:"core_temperature"`         // 核心温度
	DiskData              []HeartbeatDiskInfo    `json:"disk_data"`                // 磁盘信息
	ShotTime              string                 `json:"shot_time"`                // 快照时间
}

// HeartbeatDiskInfo 心跳中的磁盘信息
type HeartbeatDiskInfo struct {
	Path              string  `json:"path"`                // 挂载路径
	Type              string  `json:"type"`                // 文件系统类型
	Device            string  `json:"device"`              // 设备名
	Total             int64   `json:"total"`               // 总容量（字节）
	Free              int64   `json:"free"`                // 空闲容量（字节）
	Used              int64   `json:"used"`                // 已用容量（字节）
	UsedPercent       float64 `json:"used_percent"`        // 使用率
	InodesTotal       int64   `json:"inodes_total"`        // inode总数
	InodesUsed        int64   `json:"inodes_used"`         // inode已用数
	InodesFree        int64   `json:"inodes_free"`         // inode空闲数
	InodesUsedPercent float64 `json:"inodes_used_percent"` // inode使用率
}

// BoxHeartbeatResponse 心跳响应数据结构
type BoxHeartbeatResponse struct {
	Success       bool   `json:"success"`        // 是否成功
	BoxID         uint   `json:"box_id"`         // 盒子ID
	Timestamp     int64  `json:"timestamp"`      // 响应时间戳
	Message       string `json:"message"`        // 消息
	TasksToSync   int    `json:"tasks_to_sync"`  // 需要同步的任务数
	SyncTriggered bool   `json:"sync_triggered"` // 是否触发了任务同步
}

// ConvertToResources 将心跳系统信息转换为Resources结构
func (h *HeartbeatSystemInfo) ConvertToResources() Resources {
	// 转换磁盘信息
	diskData := make([]DiskUsage, len(h.DiskData))
	for i, d := range h.DiskData {
		diskData[i] = DiskUsage{
			Path:              d.Path,
			Type:              d.Type,
			Device:            d.Device,
			Total:             d.Total,
			Free:              d.Free,
			Used:              d.Used,
			UsedPercent:       d.UsedPercent,
			INodesTotal:       d.InodesTotal,
			INodesUsed:        d.InodesUsed,
			INodesFree:        d.InodesFree,
			INodesUsedPercent: d.InodesUsedPercent,
		}
	}

	return Resources{
		BoardTemperature:      h.BoardTemperature,
		CoreTemperature:       h.CoreTemperature,
		CPUPercent:            h.CPUPercent,
		CPUTotal:              h.CPUTotal,
		CPUUsed:               h.CPUUsed,
		CPUUsedPercent:        h.CPUUsedPercent,
		DiskData:              diskData,
		IOCount:               h.IOCount,
		IOReadBytes:           h.IOReadBytes,
		IOReadTime:            h.IOReadTime,
		IOWriteBytes:          h.IOWriteBytes,
		IOWriteTime:           h.IOWriteTime,
		Load1:                 h.Load1,
		Load5:                 h.Load5,
		Load15:                h.Load15,
		LoadUsagePercent:      h.LoadUsagePercent,
		MemoryAvailable:       h.MemoryAvailable,
		MemoryTotal:           h.MemoryTotal,
		MemoryUsed:            h.MemoryUsed,
		MemoryUsedPercent:     h.MemoryUsedPercent,
		NetBytesRecv:          h.NetBytesRecv,
		NetBytesSent:          h.NetBytesSent,
		NPUMemoryTotal:        int(h.NPUMemoryTotal),
		NPUMemoryUsed:         int(h.NPUMemoryUsed),
		Procs:                 h.Procs,
		ShotTime:              h.ShotTime,
		SwapMemoryAvailable:   h.SwapMemoryAvailable,
		SwapMemoryTotal:       h.SwapMemoryTotal,
		SwapMemoryUsed:        h.SwapMemoryUsed,
		SwapMemoryUsedPercent: h.SwapMemoryUsedPercent,
		TimeSinceUptime:       h.TimeSinceUptime,
		TPUUsed:               int(h.TPUUsed),
		Uptime:                h.Uptime,
		VPPMemoryTotal:        int(h.VPPMemoryTotal),
		VPPMemoryUsed:         int(h.VPPMemoryUsed),
		VPUMemoryTotal:        int(h.VPUMemoryTotal),
		VPUMemoryUsed:         int(h.VPUMemoryUsed),
	}
}



