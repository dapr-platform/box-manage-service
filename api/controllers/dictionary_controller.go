package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"box-manage-service/models"
	"box-manage-service/repository"
	"box-manage-service/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// DictionaryController 字典管理控制器。
type DictionaryController struct {
	dictionaryService service.DictionaryService
}

func NewDictionaryController(dictionaryService service.DictionaryService) *DictionaryController {
	return &DictionaryController{dictionaryService: dictionaryService}
}

// ListFields 获取字典字段定义列表。
// @Summary 获取字典字段定义列表
// @Description 获取字典字段定义列表，支持 keyword/include_disabled/page/size 查询
// @Tags 字典管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields [get]
func (c *DictionaryController) ListFields(w http.ResponseWriter, r *http.Request) {
	query := repository.DictionaryFieldQuery{
		Keyword:         r.URL.Query().Get("keyword"),
		IncludeDisabled: r.URL.Query().Get("include_disabled") == "true",
		Page:            queryInt(r, "page", 0),
		Size:            queryInt(r, "size", 0),
	}

	fields, err := c.dictionaryService.ListFields(r.Context(), query)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字典字段定义列表失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字典字段定义列表成功", fields))
}

// CreateField 创建字典字段定义。
// @Summary 创建字典字段定义
// @Description 创建字典字段定义
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param body body models.DictionaryFieldDefinition true "字典字段定义"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields [post]
func (c *DictionaryController) CreateField(w http.ResponseWriter, r *http.Request) {
	var field models.DictionaryFieldDefinition
	if err := json.NewDecoder(r.Body).Decode(&field); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if err := c.dictionaryService.CreateField(r.Context(), &field); err != nil {
		render.Render(w, r, BadRequestResponse("创建字典字段定义失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("创建字典字段定义成功", field))
}

// GetField 获取字典字段定义详情。
// @Summary 获取字典字段定义详情
// @Description 根据 field_key 获取字典字段定义详情
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields/{field_key} [get]
func (c *DictionaryController) GetField(w http.ResponseWriter, r *http.Request) {
	fieldKey := chi.URLParam(r, "field_key")
	field, err := c.dictionaryService.GetField(r.Context(), fieldKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字典字段定义失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字典字段定义成功", field))
}

// UpdateField 更新字典字段定义。
// @Summary 更新字典字段定义
// @Description 根据 field_key 更新字典字段定义
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Param body body models.DictionaryFieldDefinition true "字典字段定义"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields/{field_key} [put]
func (c *DictionaryController) UpdateField(w http.ResponseWriter, r *http.Request) {
	fieldKey := chi.URLParam(r, "field_key")
	var field models.DictionaryFieldDefinition
	if err := json.NewDecoder(r.Body).Decode(&field); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if err := c.dictionaryService.UpdateField(r.Context(), fieldKey, &field); err != nil {
		render.Render(w, r, BadRequestResponse("更新字典字段定义失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("更新字典字段定义成功", field))
}

// DeleteField 删除字典字段定义。
// @Summary 删除字典字段定义
// @Description 根据 field_key 删除字典字段定义，同时删除该字段下字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields/{field_key} [delete]
func (c *DictionaryController) DeleteField(w http.ResponseWriter, r *http.Request) {
	fieldKey := chi.URLParam(r, "field_key")
	if err := c.dictionaryService.DeleteField(r.Context(), fieldKey); err != nil {
		render.Render(w, r, BadRequestResponse("删除字典字段定义失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("删除字典字段定义成功", nil))
}

// ListInstances 获取字典实例列表。
// @Summary 获取字典实例列表
// @Description 获取字典实例列表，支持 field_key/keyword/include_disabled/page/size 查询
// @Tags 字典管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances [get]
func (c *DictionaryController) ListInstances(w http.ResponseWriter, r *http.Request) {
	query := repository.DictionaryInstanceQuery{
		FieldKey:        r.URL.Query().Get("field_key"),
		Keyword:         r.URL.Query().Get("keyword"),
		IncludeDisabled: r.URL.Query().Get("include_disabled") == "true",
		Page:            queryInt(r, "page", 0),
		Size:            queryInt(r, "size", 0),
	}
	instances, err := c.dictionaryService.ListInstances(r.Context(), query)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字典实例列表失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字典实例列表成功", instances))
}

// ListFieldInstances 获取指定字段下字典实例列表。
// @Summary 获取指定字段下字典实例列表
// @Description 根据 field_key 获取字典实例列表
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields/{field_key}/instances [get]
func (c *DictionaryController) ListFieldInstances(w http.ResponseWriter, r *http.Request) {
	query := repository.DictionaryInstanceQuery{
		FieldKey:        chi.URLParam(r, "field_key"),
		Keyword:         r.URL.Query().Get("keyword"),
		IncludeDisabled: r.URL.Query().Get("include_disabled") == "true",
		Page:            queryInt(r, "page", 0),
		Size:            queryInt(r, "size", 0),
	}
	instances, err := c.dictionaryService.ListInstances(r.Context(), query)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字段字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字段字典实例成功", instances))
}

// CreateInstance 创建字典实例。
// @Summary 创建字典实例
// @Description 创建字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param body body models.DictionaryInstance true "字典实例"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances [post]
func (c *DictionaryController) CreateInstance(w http.ResponseWriter, r *http.Request) {
	var instance models.DictionaryInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if err := c.dictionaryService.CreateInstance(r.Context(), &instance); err != nil {
		render.Render(w, r, BadRequestResponse("创建字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("创建字典实例成功", instance))
}

// CreateFieldInstance 在指定字段下创建字典实例。
// @Summary 在指定字段下创建字典实例
// @Description 根据 field_key 创建字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Param body body models.DictionaryInstance true "字典实例"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/fields/{field_key}/instances [post]
func (c *DictionaryController) CreateFieldInstance(w http.ResponseWriter, r *http.Request) {
	var instance models.DictionaryInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	instance.FieldKey = chi.URLParam(r, "field_key")
	if err := c.dictionaryService.CreateInstance(r.Context(), &instance); err != nil {
		render.Render(w, r, BadRequestResponse("创建字段字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("创建字段字典实例成功", instance))
}

// GetInstance 获取字典实例详情。
// @Summary 获取字典实例详情
// @Description 根据 ID 获取字典实例详情
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param id path int true "字典实例ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances/{id} [get]
func (c *DictionaryController) GetInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUint(w, r, "id")
	if !ok {
		return
	}
	instance, err := c.dictionaryService.GetInstance(r.Context(), id)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字典实例成功", instance))
}

// GetInstanceByKey 根据 field_key 和 instance_key 获取字典实例。
// @Summary 根据 key 获取字典实例
// @Description 根据 field_key 和 instance_key 获取字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param field_key path string true "字段标识"
// @Param instance_key path string true "实例标识"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances/by-key/{field_key}/{instance_key} [get]
func (c *DictionaryController) GetInstanceByKey(w http.ResponseWriter, r *http.Request) {
	fieldKey := chi.URLParam(r, "field_key")
	instanceKey := chi.URLParam(r, "instance_key")
	instance, err := c.dictionaryService.GetInstanceByKey(r.Context(), fieldKey, instanceKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("获取字典实例成功", instance))
}

// UpdateInstance 更新字典实例。
// @Summary 更新字典实例
// @Description 根据 ID 更新字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param id path int true "字典实例ID"
// @Param body body models.DictionaryInstance true "字典实例"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances/{id} [put]
func (c *DictionaryController) UpdateInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUint(w, r, "id")
	if !ok {
		return
	}
	var instance models.DictionaryInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if err := c.dictionaryService.UpdateInstance(r.Context(), id, &instance); err != nil {
		render.Render(w, r, BadRequestResponse("更新字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("更新字典实例成功", instance))
}

// DeleteInstance 删除字典实例。
// @Summary 删除字典实例
// @Description 根据 ID 删除字典实例
// @Tags 字典管理
// @Accept json
// @Produce json
// @Param id path int true "字典实例ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dictionaries/instances/{id} [delete]
func (c *DictionaryController) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUint(w, r, "id")
	if !ok {
		return
	}
	if err := c.dictionaryService.DeleteInstance(r.Context(), id); err != nil {
		render.Render(w, r, BadRequestResponse("删除字典实例失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("删除字典实例成功", nil))
}

func queryInt(r *http.Request, key string, defaultValue int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func pathUint(w http.ResponseWriter, r *http.Request, key string) (uint, bool) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		render.Render(w, r, BadRequestResponse(key+" 必须是正整数", err))
		return 0, false
	}
	return uint(id), true
}
