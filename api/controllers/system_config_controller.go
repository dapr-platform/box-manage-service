// Package controllers 系统配置控制器
package controllers

import (
	"encoding/json"
	"net/http"

	"box-manage-service/models"
	"box-manage-service/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// SystemConfigController 系统配置控制器
type SystemConfigController struct {
	configService service.SystemConfigService
}

// NewSystemConfigController 创建系统配置控制器
func NewSystemConfigController(configService service.SystemConfigService) *SystemConfigController {
	return &SystemConfigController{
		configService: configService,
	}
}

// GetAllConfigs 获取所有系统配置
// @Summary 获取所有系统配置
// @Description 获取所有系统配置的聚合视图
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=models.AllSystemConfigs}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config [get]
func (c *SystemConfigController) GetAllConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := c.configService.GetAllConfigs(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("获取配置成功", configs))
}

// GetConfigByType 根据类型获取配置
// @Summary 根据类型获取配置
// @Description 获取指定类型的所有配置项
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param type path string true "配置类型 (box/task/model/conversion/video/system)"
// @Success 200 {object} APIResponse{data=map[string]interface{}}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/type/{type} [get]
func (c *SystemConfigController) GetConfigByType(w http.ResponseWriter, r *http.Request) {
	configType := chi.URLParam(r, "type")
	if configType == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置类型不能为空", nil))
		return
	}
	
	// 验证配置类型
	validTypes := []string{
		models.ConfigTypeBox,
		models.ConfigTypeTask,
		models.ConfigTypeModel,
		models.ConfigTypeConversion,
		models.ConfigTypeVideo,
		models.ConfigTypeSystem,
	}
	isValid := false
	for _, t := range validTypes {
		if t == configType {
			isValid = true
			break
		}
	}
	if !isValid {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的配置类型", nil))
		return
	}
	
	configs, err := c.configService.GetConfigByType(r.Context(), configType)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("获取配置成功", configs))
}

// GetConfigByKey 根据键获取配置
// @Summary 根据键获取配置
// @Description 获取指定键的配置值
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Success 200 {object} APIResponse{data=interface{}}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/key/{key} [get]
func (c *SystemConfigController) GetConfigByKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置键不能为空", nil))
		return
	}
	
	value, err := c.configService.GetConfigByKey(r.Context(), key)
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, CreateErrorResponse(http.StatusNotFound, "配置不存在", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("获取配置成功", map[string]interface{}{
		"key":   key,
		"value": value,
	}))
}

// GetConfigMetadata 获取配置元数据
// @Summary 获取配置元数据
// @Description 获取所有配置项的元数据信息，包括名称、描述、类型、验证规则等
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=[]models.ConfigMetadata}
// @Router /api/v1/system-config/metadata [get]
func (c *SystemConfigController) GetConfigMetadata(w http.ResponseWriter, r *http.Request) {
	metadata, err := c.configService.GetConfigMetadata(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取配置元数据失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("获取配置元数据成功", metadata))
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Value interface{} `json:"value"` // 配置值
}

// UpdateConfig 更新单个配置
// @Summary 更新单个配置
// @Description 更新指定键的配置值
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Param body body UpdateConfigRequest true "配置值"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/key/{key} [put]
func (c *SystemConfigController) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置键不能为空", nil))
		return
	}
	
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "请求参数解析失败", err))
		return
	}
	
	if err := c.configService.UpdateConfig(r.Context(), key, req.Value); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "更新配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("更新配置成功", nil))
}

// BatchUpdateConfigRequest 批量更新配置请求
type BatchUpdateConfigRequest struct {
	Configs map[string]interface{} `json:"configs"` // 配置键值对
}

// UpdateConfigs 批量更新配置
// @Summary 批量更新配置
// @Description 批量更新多个配置项
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param body body BatchUpdateConfigRequest true "配置键值对"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/batch [put]
func (c *SystemConfigController) UpdateConfigs(w http.ResponseWriter, r *http.Request) {
	var req BatchUpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "请求参数解析失败", err))
		return
	}
	
	if len(req.Configs) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置列表不能为空", nil))
		return
	}
	
	if err := c.configService.UpdateConfigs(r.Context(), req.Configs); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "批量更新配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("批量更新配置成功", nil))
}

// UpdateConfigByTypeRequest 按类型更新配置请求
type UpdateConfigByTypeRequest struct {
	Values map[string]interface{} `json:"values"` // 配置键值对
}

// UpdateConfigByType 按类型更新配置
// @Summary 按类型更新配置
// @Description 更新指定类型的所有配置项
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param type path string true "配置类型 (box/task/model/conversion/video/system)"
// @Param body body UpdateConfigByTypeRequest true "配置值"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/type/{type} [put]
func (c *SystemConfigController) UpdateConfigByType(w http.ResponseWriter, r *http.Request) {
	configType := chi.URLParam(r, "type")
	if configType == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置类型不能为空", nil))
		return
	}
	
	var req UpdateConfigByTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "请求参数解析失败", err))
		return
	}
	
	if err := c.configService.UpdateConfigByType(r.Context(), configType, req.Values); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "更新配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("更新配置成功", nil))
}

// ResetToDefault 重置单个配置为默认值
// @Summary 重置配置为默认值
// @Description 将指定配置项重置为系统默认值
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/key/{key}/reset [post]
func (c *SystemConfigController) ResetToDefault(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "配置键不能为空", nil))
		return
	}
	
	if err := c.configService.ResetToDefault(r.Context(), key); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "重置配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("重置配置成功", nil))
}

// ResetAllToDefault 重置所有配置为默认值
// @Summary 重置所有配置为默认值
// @Description 将所有配置项重置为系统默认值
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/reset-all [post]
func (c *SystemConfigController) ResetAllToDefault(w http.ResponseWriter, r *http.Request) {
	if err := c.configService.ResetAllToDefault(r.Context()); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "重置所有配置失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("重置所有配置成功", nil))
}

// GetDefaultConfigs 获取默认配置
// @Summary 获取默认配置
// @Description 获取系统默认配置值（不影响当前配置）
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=models.AllSystemConfigs}
// @Router /api/v1/system-config/defaults [get]
func (c *SystemConfigController) GetDefaultConfigs(w http.ResponseWriter, r *http.Request) {
	defaults := models.GetDefaultAllConfigs()
	render.Render(w, r, SuccessResponse("获取默认配置成功", defaults))
}

// RefreshCache 刷新配置缓存
// @Summary 刷新配置缓存
// @Description 刷新系统配置缓存
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/system-config/refresh-cache [post]
func (c *SystemConfigController) RefreshCache(w http.ResponseWriter, r *http.Request) {
	if err := c.configService.RefreshCache(r.Context()); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "刷新缓存失败", err))
		return
	}
	
	render.Render(w, r, SuccessResponse("刷新缓存成功", nil))
}

