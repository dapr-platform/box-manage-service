/*
 * @module api/controllers/task_controller
 * @description 任务管理控制器，提供任务调度、监控、控制等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-004: 任务调度系统
 * @stateFlow HTTP请求处理 -> 业务逻辑处理 -> 数据库操作 -> 响应返回
 * @rules 遵循PostgREST RBAC权限验证，所有操作需要相应权限
 * @dependencies box-manage-service/service
 * @refs DESIGN-000.md
 */

package controllers

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"box-manage-service/service"

	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// TaskController 任务管理控制器
type TaskController struct {
	taskRepo          repository.TaskRepository
	boxRepo           repository.BoxRepository
	schedulerService  service.TaskSchedulerService
	deploymentService service.TaskDeploymentService
}

// NewTaskController 创建任务控制器实例
func NewTaskController(taskRepo repository.TaskRepository, boxRepo repository.BoxRepository, schedulerService service.TaskSchedulerService, deploymentService service.TaskDeploymentService) *TaskController {
	return &TaskController{
		taskRepo:          taskRepo,
		boxRepo:           boxRepo,
		schedulerService:  schedulerService,
		deploymentService: deploymentService,
	}
}

// CreateTaskRequest 创建任务请求结构体
type CreateTaskRequest struct {
	TaskID                string                 `json:"taskId" validate:"required"`
	Name                  string                 `json:"name,omitempty"`                    // 任务名称
	Description           string                 `json:"description,omitempty"`             // 任务描述
	VideoSourceID         uint                   `json:"videoSourceId" validate:"required"` // 视频源ID
	SkipFrame             int                    `json:"skipFrame,omitempty"`
	AutoStart             bool                   `json:"autoStart,omitempty"`
	UseROItoInference     bool                   `json:"useROItoInference,omitempty"`
	Tags                  []string               `json:"tags,omitempty"`
	Priority              int                    `json:"priority,omitempty"`
	AutoSchedule          bool                   `json:"autoSchedule,omitempty"` // 自动调度
	AffinityTags          []string               `json:"affinityTags,omitempty"` // 亲和性标签
	OutputSettings        models.OutputSettings  `json:"outputSettings"`
	ROIs                  []models.ROIConfig     `json:"rois,omitempty"`
	InferenceTasks        []models.InferenceTask `json:"inferenceTasks" validate:"required"`
	TaskLevelForwardInfos []models.ForwardInfo   `json:"taskLevelForwardInfos,omitempty"` // 任务级别转发配置
	ScheduledAt           *time.Time             `json:"scheduledAt,omitempty"`           // 计划执行时间
	MaxRetries            *int                   `json:"maxRetries,omitempty"`            // 最大重试次数
}

// DeployTaskRequest 下发任务请求结构体
type DeployTaskRequest struct {
	BoxID uint `json:"boxId" validate:"required"`
}

// BatchDeployRequest 批量部署请求结构体
type BatchDeployRequest struct {
	TaskIDs []string `json:"task_ids" binding:"required"`
	BoxIDs  []uint   `json:"box_ids" binding:"required"`
}

// CreateTask 创建任务
// @Summary 创建新任务
// @Description 创建一个新的AI推理任务，包含推理配置、ROI设置、输出设置等
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param task body CreateTaskRequest true "任务配置信息"
// @Success 200 {object} APIResponse{data=models.Task} "任务创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 409 {object} APIResponse "任务ID已存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks [post]
func (c *TaskController) CreateTask(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TaskController] CreateTask request received from %s", r.RemoteAddr)

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[TaskController] Failed to decode CreateTask request - Error: %v", err)
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	log.Printf("[TaskController] Processing CreateTask request - TaskID: %s, VideoSourceID: %d, InferenceTasks: %d",
		req.TaskID, req.VideoSourceID, len(req.InferenceTasks))

	// 验证任务ID是否已存在
	exists, err := c.taskRepo.IsTaskIDExists(r.Context(), req.TaskID)
	if err != nil {
		log.Printf("[TaskController] Failed to check task ID existence - TaskID: %s, Error: %v", req.TaskID, err)
		render.Render(w, r, InternalErrorResponse("检查任务ID失败", err))
		return
	}
	if exists {
		log.Printf("[TaskController] Task ID already exists - TaskID: %s", req.TaskID)
		render.Render(w, r, ConflictResponse("任务ID已存在", nil))
		return
	}

	// 构建任务对象
	task := &models.Task{
		TaskID:            req.TaskID,
		Name:              req.Name,
		Description:       req.Description,
		VideoSourceID:     req.VideoSourceID,
		SkipFrame:         req.SkipFrame,
		AutoStart:         req.AutoStart,
		UseROItoInference: req.UseROItoInference,
		Status:            models.TaskStatusPending,
		Priority:          req.Priority,
		AutoSchedule:      req.AutoSchedule,
		MaxRetries:        3, // 默认重试3次
		OutputSettings:    req.OutputSettings,
		ROIs:              models.ROIConfigList(req.ROIs),
		InferenceTasks:    models.InferenceTaskList(req.InferenceTasks),
		LastHeartbeat:     time.Now(),
		CreatedBy:         c.getCurrentUserID(r), // 从上下文获取用户ID
	}

	// 如果没有提供名称，使用TaskID作为名称
	if task.Name == "" {
		task.Name = task.TaskID
	}

	// 设置标签
	if len(req.Tags) > 0 {
		task.SetTags(req.Tags)
	}

	// 设置亲和性标签
	if len(req.AffinityTags) > 0 {
		task.SetAffinityTags(req.AffinityTags)
	}

	// 设置任务级别转发配置
	if len(req.TaskLevelForwardInfos) > 0 {
		task.TaskLevelForwardInfos = models.ForwardInfoList(req.TaskLevelForwardInfos)
	}

	// 标准化推理任务配置
	task.NormalizeInferenceTasks()

	// 保存任务
	if err := c.taskRepo.Create(r.Context(), task); err != nil {
		render.Render(w, r, InternalErrorResponse("创建任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("任务创建成功", task))
}

// GetTasks 获取任务列表
// @Summary 获取任务列表
// @Description 获取任务列表，支持按状态、标签、模型、盒子、优先级等条件筛选和搜索
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param status query string false "任务状态筛选" Enums(pending,running,completed,failed,stopped,paused)
// @Param tags query string false "标签筛选，多个标签用逗号分隔"
// @Param modelKey query string false "模型键筛选"
// @Param boxId query string false "盒子ID筛选"
// @Param priority query string false "优先级筛选"
// @Param keyword query string false "关键词搜索（搜索任务ID、RTSP URL等）"
// @Param rtsp_url query string false "RTSP URL筛选关键词"
// @Param created_by query int false "创建者用户ID"
// @Success 200 {object} APIResponse{data=[]models.Task} "获取任务列表成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks [get]
func (c *TaskController) GetTasks(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	status := r.URL.Query().Get("status")
	tags := r.URL.Query().Get("tags")
	modelKey := r.URL.Query().Get("modelKey")
	boxID := r.URL.Query().Get("boxId")
	priority := r.URL.Query().Get("priority")
	keyword := r.URL.Query().Get("keyword")
	// TODO: 需要在service层添加对rtsp_url和created_by的支持
	// rtspURL := r.URL.Query().Get("rtsp_url")
	// createdByStr := r.URL.Query().Get("created_by")

	// var createdBy *uint
	// if createdByStr != "" {
	// 	if uid, err := strconv.ParseUint(createdByStr, 10, 32); err == nil {
	// 		userID := uint(uid)
	// 		createdBy = &userID
	// 	}
	// }

	var tasks []*models.Task
	var err error

	// 根据不同条件查询
	if keyword != "" {
		// 搜索任务
		options := &repository.QueryOptions{
			Preload: []string{"Box"},
		}
		tasks, err = c.taskRepo.SearchTasks(r.Context(), keyword, options)
	} else if tags != "" {
		// 按标签查询
		tagList := strings.Split(tags, ",")
		tasks, err = c.taskRepo.FindByTags(r.Context(), tagList)
	} else if modelKey != "" {
		// 按模型查询
		tasks, err = c.taskRepo.FindTasksByModel(r.Context(), modelKey)
	} else if boxID != "" {
		// 按盒子查询
		if boxIDInt, parseErr := strconv.ParseUint(boxID, 10, 32); parseErr == nil {
			tasks, err = c.taskRepo.FindByBoxID(r.Context(), uint(boxIDInt))
		}
	} else if priority != "" {
		// 按优先级查询
		if priorityInt, parseErr := strconv.Atoi(priority); parseErr == nil {
			tasks, err = c.taskRepo.FindTasksByPriority(r.Context(), priorityInt)
		}
	} else if status != "" {
		// 按状态查询
		tasks, err = c.taskRepo.FindByStatus(r.Context(), models.TaskStatus(status))
	} else {
		// 获取所有任务
		options := &repository.QueryOptions{
			Preload: []string{"Box"},
		}
		tasks, err = c.taskRepo.FindTasksWithBox(r.Context(), options)
	}

	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取任务列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取任务列表成功", tasks))
}

// GetTask 获取任务详情
// @Summary 获取任务详情
// @Description 根据任务ID或TaskID获取任务的详细信息
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID（数字ID或TaskID）"
// @Success 200 {object} APIResponse{data=models.Task} "获取任务详情成功"
// @Failure 400 {object} APIResponse "任务ID不能为空"
// @Failure 404 {object} APIResponse "任务不存在"
// @Router /api/v1/tasks/{id} [get]
func (c *TaskController) GetTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	if taskIDStr == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	// 尝试按ID查找
	if taskIDInt, err := strconv.ParseUint(taskIDStr, 10, 32); err == nil {
		task, err := c.taskRepo.GetByID(r.Context(), uint(taskIDInt))
		if err != nil {
			render.Render(w, r, NotFoundResponse("任务不存在", err))
			return
		}
		render.Render(w, r, SuccessResponse("获取任务详情成功", task))
		return
	}

	// 按TaskID查找
	task, err := c.taskRepo.FindByTaskID(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取任务详情成功", task))
}

// UpdateTaskRequest 更新任务请求结构体
type UpdateTaskRequest struct {
	Name                  *string                `json:"name,omitempty"`          // 任务名称
	Description           *string                `json:"description,omitempty"`   // 任务描述
	VideoSourceID         *uint                  `json:"videoSourceId,omitempty"` // 视频源ID
	SkipFrame             *int                   `json:"skipFrame,omitempty"`
	AutoStart             *bool                  `json:"autoStart,omitempty"`
	UseROItoInference     *bool                  `json:"useROItoInference,omitempty"`
	Tags                  []string               `json:"tags,omitempty"`
	Priority              *int                   `json:"priority,omitempty"`
	AutoSchedule          *bool                  `json:"autoSchedule,omitempty"` // 自动调度
	AffinityTags          []string               `json:"affinityTags,omitempty"` // 亲和性标签
	OutputSettings        *models.OutputSettings `json:"outputSettings,omitempty"`
	ROIs                  []models.ROIConfig     `json:"rois,omitempty"`
	InferenceTasks        []models.InferenceTask `json:"inferenceTasks,omitempty"`
	TaskLevelForwardInfos []models.ForwardInfo   `json:"taskLevelForwardInfos,omitempty"` // 任务级别转发配置
}

// UpdateTask 更新任务
// @Summary 更新任务配置
// @Description 更新指定任务的配置信息
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param task body UpdateTaskRequest true "任务更新信息"
// @Success 200 {object} APIResponse "任务更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id} [put]
func (c *TaskController) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	log.Printf("[TaskController] Updating task %s", task.TaskID)

	// 检查任务状态，运行中的任务不能修改关键配置
	if task.IsRunning() {
		render.Render(w, r, BadRequestResponse("运行中的任务不能修改配置", nil))
		return
	}

	// 更新字段
	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.VideoSourceID != nil {
		task.VideoSourceID = *req.VideoSourceID
	}
	if req.SkipFrame != nil {
		task.SkipFrame = *req.SkipFrame
	}
	if req.AutoStart != nil {
		task.AutoStart = *req.AutoStart
	}
	if req.UseROItoInference != nil {
		task.UseROItoInference = *req.UseROItoInference
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.AutoSchedule != nil {
		oldAutoSchedule := task.AutoSchedule
		newAutoSchedule := *req.AutoSchedule
		task.AutoSchedule = newAutoSchedule

		// 当关闭自动调度时，如果任务已分配盒子，需要从盒子上移除任务
		if oldAutoSchedule && !newAutoSchedule {
			log.Printf("[TaskController] Task %s AutoSchedule changed from true to false", task.TaskID)

			// 如果任务已分配到盒子，从盒子上移除
			if task.BoxID != nil {
				log.Printf("[TaskController] Removing task %s from box %d due to AutoSchedule disabled", task.TaskID, *task.BoxID)
				if err := c.deploymentService.UndeployTask(r.Context(), task.ID, *task.BoxID); err != nil {
					log.Printf("[TaskController] Warning: Failed to undeploy task %s from box: %v", task.TaskID, err)
					// 继续执行，不阻断更新流程
				}
			}

			// 重置任务状态为未分配（使用新方法）
			task.UnassignFromBox()
			log.Printf("[TaskController] Task %s unassigned from box", task.TaskID)
		}

		// 当启用自动调度时，记录日志
		if !oldAutoSchedule && newAutoSchedule {
			log.Printf("[TaskController] Task %s AutoSchedule enabled, will be picked up by auto-scheduler", task.TaskID)
		}
	}
	if req.OutputSettings != nil {
		task.OutputSettings = *req.OutputSettings
	}
	if len(req.ROIs) > 0 {
		task.ROIs = models.ROIConfigList(req.ROIs)
	}
	if len(req.InferenceTasks) > 0 {
		task.InferenceTasks = models.InferenceTaskList(req.InferenceTasks)
		// ModelKey现在在运行时动态生成，无需预设
	}
	if len(req.Tags) > 0 {
		task.SetTags(req.Tags)
	}
	if len(req.AffinityTags) > 0 {
		task.SetAffinityTags(req.AffinityTags)
	}
	if len(req.TaskLevelForwardInfos) > 0 {
		task.TaskLevelForwardInfos = models.ForwardInfoList(req.TaskLevelForwardInfos)
	}

	// 标准化推理任务配置
	task.NormalizeInferenceTasks()

	// 保存更新
	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		log.Printf("[TaskController] Failed to update task %s: %v", task.TaskID, err)
		render.Render(w, r, InternalErrorResponse("更新任务失败", err))
		return
	}

	log.Printf("[TaskController] Task %s updated successfully", task.TaskID)
	render.Render(w, r, SuccessResponse("任务更新成功", task))
}

// DeleteTask 删除任务
// @Summary 删除任务
// @Description 删除指定的任务，如果任务正在运行，会先停止任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse "任务删除成功"
// @Failure 400 {object} APIResponse "任务状态不允许删除"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id} [delete]
func (c *TaskController) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Deleting task %s (status: %s, boxID: %v)", task.TaskID, task.Status, task.BoxID)

	// 如果任务已部署到盒子，先从盒子上停止并删除任务
	if task.BoxID != nil {
		log.Printf("[TaskController] Task %s is deployed to box %d, removing from box first", task.TaskID, *task.BoxID)

		if err := c.deploymentService.StopTaskOnBox(r.Context(), task.ID, *task.BoxID); err != nil {
			log.Printf("[TaskController] Failed to stop task on box: %v", err)
			// 继续执行删除，可能盒子已经不可用
		} else {
			log.Printf("[TaskController] Task stopped and removed from box %d successfully", *task.BoxID)
		}

		// 如果任务正在运行，更新本地状态
		if task.Status == models.TaskStatusRunning {
			task.Stop()
			if err := c.taskRepo.Update(r.Context(), task); err != nil {
				log.Printf("[TaskController] Failed to update task status: %v", err)
				// 继续执行删除
			}
		}
	} else if task.Status == models.TaskStatusRunning {
		// 如果任务没有部署到盒子但在运行中，先停止
		log.Printf("[TaskController] Task %s is running locally, stopping first", task.TaskID)
		task.Stop()
		if err := c.taskRepo.Update(r.Context(), task); err != nil {
			log.Printf("[TaskController] Failed to update task status: %v", err)
			// 继续执行删除
		}
	}

	// 删除任务
	if err := c.taskRepo.Delete(r.Context(), task.ID); err != nil {
		log.Printf("[TaskController] Failed to delete task %s: %v", task.TaskID, err)
		render.Render(w, r, InternalErrorResponse("删除任务失败", err))
		return
	}

	log.Printf("[TaskController] Task %s deleted successfully", task.TaskID)
	render.Render(w, r, SuccessResponse("任务删除成功", nil))
}

// StartTask 启动任务
// @Summary 启动任务
// @Description 启动指定的任务，任务状态必须为pending或paused，如果任务已部署到盒子，会同时启动盒子上的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.Task} "任务启动成功"
// @Failure 400 {object} APIResponse "任务状态不允许启动"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/start [post]
func (c *TaskController) StartTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Starting task %s (status: %s, boxID: %v)", task.TaskID, task.Status, task.BoxID)

	// 检查任务状态是否可以启动
	if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusPaused && task.Status != models.TaskStatusScheduled {
		render.Render(w, r, BadRequestResponse("任务当前状态不允许启动", nil))
		return
	}

	// 如果任务已部署到盒子，先启动盒子上的任务
	if task.BoxID != nil {
		log.Printf("[TaskController] Task is deployed to box %d, starting task on box first", *task.BoxID)

		// 获取盒子信息
		box, err := c.boxRepo.GetByID(r.Context(), *task.BoxID)
		if err != nil {
			log.Printf("[TaskController] Failed to get box info: %v", err)
			render.Render(w, r, InternalErrorResponse("获取盒子信息失败", err))
			return
		}

		// 创建盒子客户端并启动任务
		boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
		if err := boxClient.StartTask(r.Context(), task.TaskID); err != nil {
			log.Printf("[TaskController] Failed to start task on box: %v", err)
			render.Render(w, r, InternalErrorResponse("启动盒子上的任务失败", err))
			return
		}
		log.Printf("[TaskController] Task started successfully on box %d", *task.BoxID)
	}

	// 更新任务状态
	task.Start()
	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		log.Printf("[TaskController] Failed to update task status: %v", err)
		render.Render(w, r, InternalErrorResponse("启动任务失败", err))
		return
	}

	log.Printf("[TaskController] Task %s started successfully", task.TaskID)
	render.Render(w, r, SuccessResponse("任务启动成功", task))
}

// StopTask 停止任务
// @Summary 停止任务
// @Description 停止指定的任务，任务状态必须为running或paused，如果任务已部署到盒子，会先停止盒子上的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.Task} "任务停止成功"
// @Failure 400 {object} APIResponse "任务状态不允许停止"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/stop [post]
func (c *TaskController) StopTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Stopping task %s (status: %s, boxID: %v)", task.TaskID, task.Status, task.BoxID)

	// 检查任务状态是否可以停止
	if task.Status != models.TaskStatusRunning && task.Status != models.TaskStatusPaused {
		render.Render(w, r, BadRequestResponse("任务当前状态不允许停止", nil))
		return
	}

	// 如果任务已部署到盒子，先停止盒子上的任务
	if task.BoxID != nil {
		log.Printf("[TaskController] Task is deployed to box %d, stopping task on box first", *task.BoxID)

		if err := c.deploymentService.StopTaskOnBox(r.Context(), task.ID, *task.BoxID); err != nil {
			log.Printf("[TaskController] Failed to stop task on box: %v", err)
			// 不返回错误，继续更新本地状态，因为盒子可能已经不可用
		} else {
			log.Printf("[TaskController] Task stopped successfully on box %d", *task.BoxID)
		}
	}

	// 更新任务状态
	task.Stop()
	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		log.Printf("[TaskController] Failed to update task status: %v", err)
		render.Render(w, r, InternalErrorResponse("停止任务失败", err))
		return
	}

	log.Printf("[TaskController] Task %s stopped successfully", task.TaskID)
	render.Render(w, r, SuccessResponse("任务停止成功", task))
}

// PauseTask 暂停任务
// @Summary 暂停任务
// @Description 暂停正在运行的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.Task} "任务暂停成功"
// @Failure 400 {object} APIResponse "任务状态不允许暂停"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/pause [post]
func (c *TaskController) PauseTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	// 检查任务状态是否可以暂停
	if task.Status != models.TaskStatusRunning {
		render.Render(w, r, BadRequestResponse("只有运行中的任务可以暂停", nil))
		return
	}

	// 更新任务状态
	task.Pause()
	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		render.Render(w, r, InternalErrorResponse("暂停任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("任务暂停成功", task))
}

// ResumeTask 恢复任务
// @Summary 恢复任务
// @Description 恢复已暂停的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.Task} "任务恢复成功"
// @Failure 400 {object} APIResponse "任务状态不允许恢复"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/resume [post]
func (c *TaskController) ResumeTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	// 检查任务状态是否可以恢复
	if task.Status != models.TaskStatusPaused {
		render.Render(w, r, BadRequestResponse("只有暂停的任务可以恢复", nil))
		return
	}

	// 更新任务状态
	task.Resume()
	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		render.Render(w, r, InternalErrorResponse("恢复任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("任务恢复成功", task))
}

// RetryTask 重试任务
// @Summary 重试任务
// @Description 重新执行失败的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.Task} "任务重试成功"
// @Failure 400 {object} APIResponse "任务不可重试"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/retry [post]
func (c *TaskController) RetryTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	// 检查任务是否可以重试
	if !task.CanRetry() {
		render.Render(w, r, BadRequestResponse("任务不可重试", nil))
		return
	}

	// 重试任务
	if !task.Retry() {
		render.Render(w, r, BadRequestResponse("重试任务失败", nil))
		return
	}

	if err := c.taskRepo.Update(r.Context(), task); err != nil {
		render.Render(w, r, InternalErrorResponse("重试任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("任务重试成功", task))
}

// TaskStatusResponse 任务状态响应结构体
type TaskStatusResponse struct {
	TaskID        string  `json:"task_id"`
	Status        string  `json:"status"`
	Progress      float64 `json:"progress"`
	BoxID         *uint   `json:"box_id,omitempty"`
	BoxStatus     string  `json:"box_status,omitempty"`
	StartTime     *string `json:"start_time,omitempty"`
	StopTime      *string `json:"stop_time,omitempty"`
	LastHeartbeat string  `json:"last_heartbeat"`
	ErrorMessage  string  `json:"error_message,omitempty"`
	RetryCount    int     `json:"retry_count"`
	MaxRetries    int     `json:"max_retries"`
}

// GetTaskStatus 获取任务状态
// @Summary 获取任务状态
// @Description 获取指定任务的当前状态信息，包括在盒子上的运行状态
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=TaskStatusResponse} "获取任务状态成功"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/status [get]
func (c *TaskController) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Getting status for task %s", task.TaskID)

	// 构建基础状态响应
	response := &TaskStatusResponse{
		TaskID:        task.TaskID,
		Status:        string(task.Status),
		Progress:      task.Progress,
		BoxID:         task.BoxID,
		LastHeartbeat: task.LastHeartbeat.Format("2006-01-02 15:04:05"),
		ErrorMessage:  task.LastError,
		RetryCount:    task.RetryCount,
		MaxRetries:    task.MaxRetries,
	}

	if task.StartTime != nil {
		startTime := task.StartTime.Format("2006-01-02 15:04:05")
		response.StartTime = &startTime
	}
	if task.StopTime != nil {
		stopTime := task.StopTime.Format("2006-01-02 15:04:05")
		response.StopTime = &stopTime
	}

	// 如果任务已部署到盒子，获取盒子上的实时状态
	if task.BoxID != nil {
		log.Printf("[TaskController] Task is deployed to box %d, getting box status", *task.BoxID)

		boxTaskStatus, err := c.deploymentService.GetTaskStatusFromBox(r.Context(), task.ID, *task.BoxID)
		if err != nil {
			log.Printf("[TaskController] Failed to get task status from box: %v", err)
			response.BoxStatus = "error"
		} else if boxTaskStatus != nil {
			response.BoxStatus = boxTaskStatus.Status
			// 更新进度信息
			if boxTaskStatus.Progress > 0 {
				response.Progress = boxTaskStatus.Progress
			}
		}
	}

	log.Printf("[TaskController] Task %s status retrieved: %s", task.TaskID, response.Status)
	render.Render(w, r, SuccessResponse("获取任务状态成功", response))
}

// TaskResultsResponse 任务结果响应结构体
type TaskResultsResponse struct {
	TaskID         string                 `json:"task_id"`
	Status         string                 `json:"status"`
	TotalFrames    int64                  `json:"total_frames"`
	InferenceCount int64                  `json:"inference_count"`
	ForwardSuccess int64                  `json:"forward_success"`
	ForwardFailed  int64                  `json:"forward_failed"`
	StartTime      *string                `json:"start_time,omitempty"`
	StopTime       *string                `json:"stop_time,omitempty"`
	Duration       string                 `json:"duration,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	Statistics     map[string]interface{} `json:"statistics,omitempty"`
}

// GetTaskResults 获取任务结果
// @Summary 获取任务结果
// @Description 获取指定任务的执行结果统计信息
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=TaskResultsResponse} "获取任务结果成功"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/results [get]
func (c *TaskController) GetTaskResults(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Getting results for task %s", task.TaskID)

	// 构建结果响应
	response := &TaskResultsResponse{
		TaskID:         task.TaskID,
		Status:         string(task.Status),
		TotalFrames:    task.TotalFrames,
		InferenceCount: task.InferenceCount,
		ForwardSuccess: task.ForwardSuccess,
		ForwardFailed:  task.ForwardFailed,
		ErrorMessage:   task.LastError,
	}

	if task.StartTime != nil {
		startTime := task.StartTime.Format("2006-01-02 15:04:05")
		response.StartTime = &startTime
	}
	if task.StopTime != nil {
		stopTime := task.StopTime.Format("2006-01-02 15:04:05")
		response.StopTime = &stopTime
	}

	// 计算执行时长
	if task.StartTime != nil {
		endTime := time.Now()
		if task.StopTime != nil {
			endTime = *task.StopTime
		}
		duration := endTime.Sub(*task.StartTime)
		response.Duration = duration.String()
	}

	// 构建统计信息
	response.Statistics = map[string]interface{}{
		"success_rate": func() float64 {
			if task.InferenceCount > 0 {
				return float64(task.ForwardSuccess) / float64(task.InferenceCount) * 100
			}
			return 0
		}(),
		"failure_rate": func() float64 {
			if task.InferenceCount > 0 {
				return float64(task.ForwardFailed) / float64(task.InferenceCount) * 100
			}
			return 0
		}(),
		"avg_fps": func() float64 {
			if task.StartTime != nil && task.TotalFrames > 0 {
				endTime := time.Now()
				if task.StopTime != nil {
					endTime = *task.StopTime
				}
				duration := endTime.Sub(*task.StartTime).Seconds()
				if duration > 0 {
					return float64(task.TotalFrames) / duration
				}
			}
			return 0
		}(),
	}

	// 如果任务在盒子上运行，尝试获取更详细的统计信息
	if task.BoxID != nil {
		boxTaskStatus, err := c.deploymentService.GetTaskStatusFromBox(r.Context(), task.ID, *task.BoxID)
		if err == nil && boxTaskStatus != nil && boxTaskStatus.Statistics != nil {
			response.Statistics["box_fps"] = boxTaskStatus.Statistics.FPS
			response.Statistics["box_latency"] = boxTaskStatus.Statistics.AverageLatency
			response.Statistics["box_processed_frames"] = boxTaskStatus.Statistics.ProcessedFrames
			response.Statistics["box_inference_count"] = boxTaskStatus.Statistics.InferenceCount
		}
	}

	log.Printf("[TaskController] Task %s results retrieved", task.TaskID)
	render.Render(w, r, SuccessResponse("获取任务结果成功", response))
}

// GetTaskLogs 获取任务日志
// @Summary 获取任务日志
// @Description 获取指定任务的执行日志
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} APIResponse{data=[]string} "获取任务日志成功"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/logs [get]
func (c *TaskController) GetTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	// TODO: 实现真实的日志获取逻辑
	logs := []string{
		fmt.Sprintf("[%s] 任务 %s 已创建", time.Now().Format("2006-01-02 15:04:05"), task.TaskID),
		fmt.Sprintf("[%s] 任务状态: %s", time.Now().Format("2006-01-02 15:04:05"), task.Status),
	}

	render.Render(w, r, SuccessResponse("获取任务日志成功", logs))
}

// BatchOperationRequest 批量操作请求结构体
type BatchOperationRequest struct {
	TaskIDs []string `json:"task_ids" validate:"required,min=1"`
}

// BatchOperationResult 批量操作结果
type BatchOperationResult struct {
	TaskID    string `json:"task_id"`
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	NewStatus string `json:"new_status,omitempty"`
}

// BatchOperationResponse 批量操作响应
type BatchOperationResponse struct {
	TotalTasks   int                    `json:"total_tasks"`
	SuccessCount int                    `json:"success_count"`
	FailureCount int                    `json:"failure_count"`
	Results      []BatchOperationResult `json:"results"`
}

// BatchStart 批量启动任务
// @Summary 批量启动任务
// @Description 批量启动多个任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param batch body BatchOperationRequest true "批量操作请求"
// @Success 200 {object} APIResponse{data=BatchOperationResponse} "批量启动完成"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/batch/start [post]
func (c *TaskController) BatchStart(w http.ResponseWriter, r *http.Request) {
	var req BatchOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	if len(req.TaskIDs) == 0 {
		render.Render(w, r, BadRequestResponse("任务ID列表不能为空", nil))
		return
	}

	log.Printf("[TaskController] BatchStart request for %d tasks", len(req.TaskIDs))

	var results []BatchOperationResult
	var successCount, failureCount int

	for _, taskIDStr := range req.TaskIDs {
		result := BatchOperationResult{TaskID: taskIDStr}

		task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
		if err != nil {
			result.Success = false
			result.Message = "任务不存在"
			failureCount++
		} else {
			// 检查任务状态是否可以启动
			if task.Status != models.TaskStatusPending && task.Status != models.TaskStatusPaused {
				result.Success = false
				result.Message = fmt.Sprintf("任务当前状态(%s)不允许启动", task.Status)
				failureCount++
			} else {
				// 启动任务
				task.Start()
				if err := c.taskRepo.Update(r.Context(), task); err != nil {
					result.Success = false
					result.Message = fmt.Sprintf("启动任务失败: %v", err)
					failureCount++
				} else {
					result.Success = true
					result.Message = "任务启动成功"
					result.NewStatus = string(task.Status)
					successCount++
				}
			}
		}

		results = append(results, result)
	}

	response := &BatchOperationResponse{
		TotalTasks:   len(req.TaskIDs),
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
	}

	log.Printf("[TaskController] BatchStart completed - Total: %d, Success: %d, Failed: %d",
		response.TotalTasks, response.SuccessCount, response.FailureCount)
	render.Render(w, r, SuccessResponse("批量启动任务完成", response))
}

// BatchStop 批量停止任务
// @Summary 批量停止任务
// @Description 批量停止多个任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param batch body BatchOperationRequest true "批量操作请求"
// @Success 200 {object} APIResponse{data=BatchOperationResponse} "批量停止完成"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/batch/stop [post]
func (c *TaskController) BatchStop(w http.ResponseWriter, r *http.Request) {
	var req BatchOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	if len(req.TaskIDs) == 0 {
		render.Render(w, r, BadRequestResponse("任务ID列表不能为空", nil))
		return
	}

	log.Printf("[TaskController] BatchStop request for %d tasks", len(req.TaskIDs))

	var results []BatchOperationResult
	var successCount, failureCount int

	for _, taskIDStr := range req.TaskIDs {
		result := BatchOperationResult{TaskID: taskIDStr}

		task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
		if err != nil {
			result.Success = false
			result.Message = "任务不存在"
			failureCount++
		} else {
			// 检查任务状态是否可以停止
			if task.Status != models.TaskStatusRunning && task.Status != models.TaskStatusPaused {
				result.Success = false
				result.Message = fmt.Sprintf("任务当前状态(%s)不允许停止", task.Status)
				failureCount++
			} else {
				// 如果任务已部署到盒子，先从盒子上停止
				if task.BoxID != nil {
					if err := c.deploymentService.StopTaskOnBox(r.Context(), task.ID, *task.BoxID); err != nil {
						log.Printf("[TaskController] Failed to stop task %s on box: %v", task.TaskID, err)
						// 继续执行，更新本地状态
					}
				}

				// 停止任务
				task.Stop()
				if err := c.taskRepo.Update(r.Context(), task); err != nil {
					result.Success = false
					result.Message = fmt.Sprintf("停止任务失败: %v", err)
					failureCount++
				} else {
					result.Success = true
					result.Message = "任务停止成功"
					result.NewStatus = string(task.Status)
					successCount++
				}
			}
		}

		results = append(results, result)
	}

	response := &BatchOperationResponse{
		TotalTasks:   len(req.TaskIDs),
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
	}

	log.Printf("[TaskController] BatchStop completed - Total: %d, Success: %d, Failed: %d",
		response.TotalTasks, response.SuccessCount, response.FailureCount)
	render.Render(w, r, SuccessResponse("批量停止任务完成", response))
}

// BatchDelete 批量删除任务
// @Summary 批量删除任务
// @Description 批量删除多个任务，如果任务正在运行会先停止
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param batch body BatchOperationRequest true "批量操作请求"
// @Success 200 {object} APIResponse{data=BatchOperationResponse} "批量删除完成"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/batch/delete [post]
func (c *TaskController) BatchDelete(w http.ResponseWriter, r *http.Request) {
	var req BatchOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	if len(req.TaskIDs) == 0 {
		render.Render(w, r, BadRequestResponse("任务ID列表不能为空", nil))
		return
	}

	log.Printf("[TaskController] BatchDelete request for %d tasks", len(req.TaskIDs))

	var results []BatchOperationResult
	var successCount, failureCount int

	for _, taskIDStr := range req.TaskIDs {
		result := BatchOperationResult{TaskID: taskIDStr}

		task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
		if err != nil {
			result.Success = false
			result.Message = "任务不存在"
			failureCount++
		} else {
			// 如果任务正在运行，先停止
			if task.Status == models.TaskStatusRunning {
				// 如果任务已部署到盒子，先从盒子上停止
				if task.BoxID != nil {
					if err := c.deploymentService.StopTaskOnBox(r.Context(), task.ID, *task.BoxID); err != nil {
						log.Printf("[TaskController] Failed to stop task %s on box before deletion: %v", task.TaskID, err)
						// 继续执行删除
					}
				}
				// 更新任务状态为停止
				task.Stop()
				if err := c.taskRepo.Update(r.Context(), task); err != nil {
					log.Printf("[TaskController] Failed to update task status before deletion: %v", err)
					// 继续执行删除
				}
			}

			// 删除任务
			if err := c.taskRepo.Delete(r.Context(), task.ID); err != nil {
				result.Success = false
				result.Message = fmt.Sprintf("删除任务失败: %v", err)
				failureCount++
			} else {
				result.Success = true
				result.Message = "任务删除成功"
				result.NewStatus = "deleted"
				successCount++
			}
		}

		results = append(results, result)
	}

	response := &BatchOperationResponse{
		TotalTasks:   len(req.TaskIDs),
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
	}

	log.Printf("[TaskController] BatchDelete completed - Total: %d, Success: %d, Failed: %d",
		response.TotalTasks, response.SuccessCount, response.FailureCount)
	render.Render(w, r, SuccessResponse("批量删除任务完成", response))
}

// GetTaskStatistics 获取任务统计
// @Summary 获取任务统计信息
// @Description 获取各种状态的任务数量统计信息
// @Tags 任务管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse "获取任务统计成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/statistics [get]
func (c *TaskController) GetTaskStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := c.taskRepo.GetTaskStatistics(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取任务统计失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取任务统计成功", stats))
}

// GetTaskTags 获取所有任务标签
// @Summary 获取所有任务标签
// @Description 获取系统中所有已使用的任务标签列表
// @Tags 任务管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=[]string} "获取任务标签成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/tags [get]
func (c *TaskController) GetTaskTags(w http.ResponseWriter, r *http.Request) {
	tags, err := c.taskRepo.GetAllTags(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取任务标签失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取任务标签成功", tags))
}

// DeployTask 下发任务到指定盒子
// @Summary 下发任务到指定盒子
// @Description 将任务部署到指定的AI盒子上执行，创建部署任务进行管理
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param deploy body DeployTaskRequest true "部署配置"
// @Success 200 {object} APIResponse "任务下发成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/deploy [post]
func (c *TaskController) DeployTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	var req DeployTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	if req.BoxID == 0 {
		render.Render(w, r, BadRequestResponse("盒子ID不能为空", nil))
		return
	}

	log.Printf("[TaskController] DeployTask request - TaskID: %s, BoxID: %d", task.TaskID, req.BoxID)

	// 直接使用同步部署（保持向后兼容）
	resp, err := c.deploymentService.DeployTask(r.Context(), task.ID, req.BoxID)
	if err != nil || !resp.Success {
		log.Printf("[TaskController] Failed to deploy task %s to box %d: %v", task.TaskID, req.BoxID, err)
		if resp != nil {
			render.Render(w, r, InternalErrorResponse(resp.Message, err))
		} else {
			render.Render(w, r, InternalErrorResponse("任务下发失败", err))
		}
		return
	}

	log.Printf("[TaskController] Task %s successfully deployed to box %d", task.TaskID, req.BoxID)
	render.Render(w, r, SuccessResponse("任务下发成功", resp))
}

// BatchDeploy 批量下发任务（异步）
// @Summary 批量下发任务（异步）
// @Description 创建批量部署任务，异步将多个任务部署到指定的AI盒子上执行
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param batch body BatchDeployRequest true "批量部署配置"
// @Success 200 {object} APIResponse "批量部署任务创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/batch/deploy [post]
func (c *TaskController) BatchDeploy(w http.ResponseWriter, r *http.Request) {
	var req BatchDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	if len(req.TaskIDs) == 0 || len(req.BoxIDs) == 0 {
		render.Render(w, r, BadRequestResponse("任务ID和盒子ID不能为空", nil))
		return
	}

	log.Printf("[TaskController] BatchDeploy request - Tasks: %d, Boxes: %d", len(req.TaskIDs), len(req.BoxIDs))

	// 转换任务ID字符串为uint
	var taskIDs []uint
	for _, taskIDStr := range req.TaskIDs {
		if taskIDInt, err := strconv.ParseUint(taskIDStr, 10, 32); err == nil {
			taskIDs = append(taskIDs, uint(taskIDInt))
		} else {
			// 尝试按TaskID查找
			if task, err := c.taskRepo.FindByTaskID(r.Context(), taskIDStr); err == nil {
				taskIDs = append(taskIDs, task.ID)
			}
		}
	}

	if len(taskIDs) == 0 {
		render.Render(w, r, BadRequestResponse("没有有效的任务ID", nil))
		return
	}

	// 提示：建议使用新的部署任务API进行异步批量部署
	render.Render(w, r, SuccessResponse("建议使用 /api/v1/deployments/batch 接口进行异步批量部署", map[string]interface{}{
		"message":             "当前接口仅支持简单部署，大量任务建议使用部署任务管理系统",
		"recommended_api":     "/api/v1/deployments/batch",
		"deployment_api_docs": "/api/v1/deployments",
		"task_count":          len(taskIDs),
		"box_count":           len(req.BoxIDs),
	}))
}

// 辅助方法

// getCurrentUserID 从请求上下文获取当前用户ID
func (c *TaskController) getCurrentUserID(r *http.Request) uint {
	return getUserIDFromRequest(r)
}

// getTaskByIDStr 根据字符串ID获取任务（支持数字ID和TaskID）
func (c *TaskController) getTaskByIDStr(ctx context.Context, taskIDStr string) (*models.Task, error) {
	// 尝试按数字ID查找
	if taskIDInt, err := strconv.ParseUint(taskIDStr, 10, 32); err == nil {
		return c.taskRepo.GetByID(ctx, uint(taskIDInt))
	}

	// 按TaskID查找
	return c.taskRepo.FindByTaskID(ctx, taskIDStr)
}

// GetAutoScheduleTasks 获取启用自动调度的任务
// @Summary 获取启用自动调度的任务
// @Description 获取所有启用了自动调度的待执行任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param limit query int false "限制返回数量" default(10)
// @Success 200 {object} APIResponse{data=[]models.Task} "获取自动调度任务成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/auto-schedule [get]
func (c *TaskController) GetAutoScheduleTasks(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TaskController] GetAutoScheduleTasks request received")

	// 解析limit参数
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // 默认限制
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// 获取自动调度任务
	tasks, err := c.taskRepo.FindAutoScheduleTasks(r.Context(), limit)
	if err != nil {
		log.Printf("[TaskController] Failed to get auto schedule tasks - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("获取自动调度任务失败", err))
		return
	}

	log.Printf("[TaskController] Found %d auto schedule tasks", len(tasks))
	render.Render(w, r, SuccessResponse("获取自动调度任务成功", tasks))
}

// GetTasksByBox 获取指定盒子兼容的任务
// @Summary 获取指定盒子兼容的任务
// @Description 根据盒子ID获取与该盒子兼容的任务（基于亲和性标签）
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param boxId path uint true "盒子ID"
// @Success 200 {object} APIResponse{data=[]models.Task} "获取兼容任务成功"
// @Failure 400 {object} APIResponse "盒子ID格式错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/box/{boxId}/compatible [get]
func (c *TaskController) GetTasksByBox(w http.ResponseWriter, r *http.Request) {
	boxIDStr := chi.URLParam(r, "boxId")
	if boxIDStr == "" {
		render.Render(w, r, BadRequestResponse("盒子ID不能为空", nil))
		return
	}

	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("盒子ID格式错误", err))
		return
	}

	log.Printf("[TaskController] Getting compatible tasks for box: %d", boxID)

	// 获取兼容的任务
	tasks, err := c.taskRepo.FindTasksCompatibleWithBox(r.Context(), uint(boxID))
	if err != nil {
		log.Printf("[TaskController] Failed to get compatible tasks for box %d - Error: %v", boxID, err)
		render.Render(w, r, InternalErrorResponse("获取兼容任务失败", err))
		return
	}

	log.Printf("[TaskController] Found %d compatible tasks for box %d", len(tasks), boxID)
	render.Render(w, r, SuccessResponse("获取兼容任务成功", tasks))
}

// GetTasksByAffinityTags 根据亲和性标签查找任务
// @Summary 根据亲和性标签查找任务
// @Description 根据提供的亲和性标签查找匹配的任务
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param tags query string true "亲和性标签，多个标签用逗号分隔"
// @Success 200 {object} APIResponse{data=[]models.Task} "查找任务成功"
// @Failure 400 {object} APIResponse "标签参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/affinity [get]
func (c *TaskController) GetTasksByAffinityTags(w http.ResponseWriter, r *http.Request) {
	tagsStr := r.URL.Query().Get("tags")
	if tagsStr == "" {
		render.Render(w, r, BadRequestResponse("亲和性标签不能为空", nil))
		return
	}

	tags := strings.Split(tagsStr, ",")
	for i, tag := range tags {
		tags[i] = strings.TrimSpace(tag)
	}

	log.Printf("[TaskController] Searching tasks by affinity tags: %v", tags)

	// 根据亲和性标签查找任务
	tasks, err := c.taskRepo.FindTasksWithAffinityTags(r.Context(), tags)
	if err != nil {
		log.Printf("[TaskController] Failed to find tasks by affinity tags %v - Error: %v", tags, err)
		render.Render(w, r, InternalErrorResponse("查找任务失败", err))
		return
	}

	log.Printf("[TaskController] Found %d tasks matching affinity tags %v", len(tasks), tags)
	render.Render(w, r, SuccessResponse("查找任务成功", tasks))
}

// UpdateTaskProgress 更新任务进度
// @Summary 更新任务进度
// @Description 更新指定任务的执行进度
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param progress body map[string]float64 true "进度信息" example({"progress": 50.5})
// @Success 200 {object} APIResponse "任务进度更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/progress [put]
func (c *TaskController) UpdateTaskProgress(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	var req struct {
		Progress float64 `json:"progress" validate:"min=0,max=100"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	log.Printf("[TaskController] Updating task %s progress to %.2f%%", task.TaskID, req.Progress)

	// 更新任务进度
	if err := c.taskRepo.UpdateTaskProgress(r.Context(), task.ID, req.Progress); err != nil {
		log.Printf("[TaskController] Failed to update task progress - TaskID: %s, Error: %v", task.TaskID, err)
		render.Render(w, r, InternalErrorResponse("更新任务进度失败", err))
		return
	}

	log.Printf("[TaskController] Successfully updated task %s progress", task.TaskID)
	render.Render(w, r, SuccessResponse("任务进度更新成功", nil))
}

// TriggerAutoSchedule 触发自动调度
// @Summary 触发自动调度
// @Description 触发一次自动调度，将启用自动调度的任务分配到合适的盒子
// @Tags 任务管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=service.ScheduleResult} "自动调度完成"
// @Failure 500 {object} APIResponse "调度失败"
// @Router /api/v1/tasks/schedule/auto [post]
func (c *TaskController) TriggerAutoSchedule(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TaskController] TriggerAutoSchedule request received from %s", r.RemoteAddr)

	// 执行自动调度
	result, err := c.schedulerService.ScheduleAutoTasks(r.Context())
	if err != nil {
		log.Printf("[TaskController] Auto schedule failed - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("自动调度失败", err))
		return
	}

	log.Printf("[TaskController] Auto schedule completed - Total: %d, Scheduled: %d, Failed: %d",
		result.TotalTasks, result.ScheduledTasks, result.FailedTasks)
	render.Render(w, r, SuccessResponse("自动调度完成", result))
}

// ScheduleTask 调度指定任务
// @Summary 调度指定任务
// @Description 将指定任务调度到最合适的盒子
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse{data=service.TaskScheduleResult} "任务调度成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "调度失败"
// @Router /api/v1/tasks/{id}/schedule [post]
func (c *TaskController) ScheduleTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		log.Printf("[TaskController] Invalid task ID parameter - TaskID: %s, Error: %v", taskIDStr, err)
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	log.Printf("[TaskController] ScheduleTask request received - TaskID: %d", taskID)

	// 执行任务调度
	result, err := c.schedulerService.ScheduleTask(r.Context(), uint(taskID))
	if err != nil {
		log.Printf("[TaskController] Schedule task failed - TaskID: %d, Error: %v", taskID, err)
		render.Render(w, r, InternalErrorResponse("任务调度失败", err))
		return
	}

	if result.Success {
		log.Printf("[TaskController] Task scheduled successfully - TaskID: %d, BoxID: %d", taskID, *result.BoxID)
	} else {
		log.Printf("[TaskController] Task schedule failed - TaskID: %d, Reason: %s", taskID, result.Reason)
	}

	render.Render(w, r, SuccessResponse("任务调度处理完成", result))
}

// GetCompatibleBoxes 获取与任务兼容的盒子
// @Summary 获取与任务兼容的盒子
// @Description 查找与指定任务兼容的盒子列表，按适配分数排序
// @Tags 任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse{data=[]service.BoxScore} "获取兼容盒子成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "查询失败"
// @Router /api/v1/tasks/{id}/compatible-boxes [get]
func (c *TaskController) GetCompatibleBoxes(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		log.Printf("[TaskController] Invalid task ID parameter - TaskID: %s, Error: %v", taskIDStr, err)
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	log.Printf("[TaskController] GetCompatibleBoxes request received - TaskID: %d", taskID)

	// 查找兼容的盒子
	boxes, err := c.schedulerService.FindCompatibleBoxes(r.Context(), uint(taskID))
	if err != nil {
		log.Printf("[TaskController] Find compatible boxes failed - TaskID: %d, Error: %v", taskID, err)
		render.Render(w, r, InternalErrorResponse("查找兼容盒子失败", err))
		return
	}

	log.Printf("[TaskController] Found %d compatible boxes for task %d", len(boxes), taskID)
	render.Render(w, r, SuccessResponse("获取兼容盒子成功", boxes))
}

// GetTaskSSEStream 获取任务的SSE图像流代理
// @Summary 获取任务SSE图像流
// @Description 代理访问盒子上任务的SSE图像流，如果任务未部署到盒子则返回错误
// @Tags 任务管理
// @Accept text/event-stream
// @Produce text/event-stream
// @Param id path string true "任务ID"
// @Success 200 {string} string "SSE流数据"
// @Failure 400 {object} APIResponse "任务未部署到盒子"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tasks/{id}/sse [get]
func (c *TaskController) GetTaskSSEStream(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	task, err := c.getTaskByIDStr(r.Context(), taskIDStr)
	if err != nil {
		render.Render(w, r, NotFoundResponse("任务不存在", err))
		return
	}

	log.Printf("[TaskController] Getting SSE stream for task %s", task.TaskID)

	// 检查任务是否已部署到盒子
	if task.BoxID == nil {
		log.Printf("[TaskController] Task %s is not deployed to any box", task.TaskID)
		render.Render(w, r, BadRequestResponse("任务未部署到盒子", nil))
		return
	}

	// 获取盒子信息
	box, err := c.boxRepo.GetByID(r.Context(), *task.BoxID)
	if err != nil {
		log.Printf("[TaskController] Failed to get box info: %v", err)
		render.Render(w, r, InternalErrorResponse("获取盒子信息失败", err))
		return
	}

	log.Printf("[TaskController] Proxying SSE stream from box %d (%s:%d) for task %s",
		*task.BoxID, box.IPAddress, box.Port, task.TaskID)

	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// 创建到盒子的SSE连接URL
	sseURL := fmt.Sprintf("http://%s:%d/api/v1/sse/%s", box.IPAddress, box.Port, task.TaskID)

	log.Printf("[TaskController] Connecting to box SSE URL: %s", sseURL)

	// 创建到盒子的HTTP请求
	req, err := http.NewRequestWithContext(r.Context(), "GET", sseURL, nil)
	if err != nil {
		log.Printf("[TaskController] Failed to create request to box: %v", err)
		render.Render(w, r, InternalErrorResponse("创建盒子请求失败", err))
		return
	}

	// 设置请求头
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// 发送请求到盒子
	client := &http.Client{
		Timeout: 0, // 无超时，因为这是流式连接
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[TaskController] Failed to connect to box SSE: %v", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"Failed to connect to box\", \"message\": \"%v\"}\n\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[TaskController] Box SSE returned non-200 status: %d", resp.StatusCode)
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"Box SSE error\", \"status\": %d}\n\n", resp.StatusCode)
		return
	}

	log.Printf("[TaskController] Successfully connected to box SSE, starting proxy")

	// 创建一个flusher来实时推送数据
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("[TaskController] Response writer does not support flushing")
		render.Render(w, r, InternalErrorResponse("不支持流式响应", nil))
		return
	}

	// 代理SSE数据流
	buffer := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			log.Printf("[TaskController] Client disconnected from SSE stream")
			return
		default:
			n, err := resp.Body.Read(buffer)
			if err != nil {
				if err != io.EOF {
					log.Printf("[TaskController] Error reading from box SSE: %v", err)
					fmt.Fprintf(w, "event: error\ndata: {\"error\": \"Stream read error\", \"message\": \"%v\"}\n\n", err)
					flusher.Flush()
				}
				return
			}

			if n > 0 {
				w.Write(buffer[:n])
				flusher.Flush()
			}
		}
	}
}
