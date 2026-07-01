package controllers

import (
	authMiddleware "box-manage-service/api/middleware"
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// MenuController 菜单权限控制器。
type MenuController struct {
	menuService service.MenuService
}

// NewMenuController 创建菜单权限控制器。
func NewMenuController(menuService service.MenuService) *MenuController {
	return &MenuController{menuService: menuService}
}

// GetCurrentUser 获取当前用户信息和菜单权限。
// @Summary 获取当前用户信息和菜单权限
// @Description 返回当前用户、角色、权限、menu_ids 和完整菜单树
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=service.CurrentUserWithMenus}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/me [get]
func (c *MenuController) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := authMiddleware.GetUserFromContext(r.Context())
	userInfo := &service.CurrentUserContext{}
	if user != nil {
		userInfo.ID = user.ID
		userInfo.Username = user.Username
		userInfo.Email = user.Email
		userInfo.Role = user.Role
		userInfo.Roles = user.Roles
		userInfo.Permissions = user.Permissions
	}

	data, err := c.menuService.GetCurrentUserWithMenus(r.Context(), userInfo)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取当前用户菜单权限失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取当前用户成功", data))
}

// GetMenus 获取全部启用菜单树。
// @Summary 获取全部启用菜单树
// @Description 返回系统中所有启用的菜单项
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus [get]
func (c *MenuController) GetMenus(w http.ResponseWriter, r *http.Request) {
	includeDisabled := r.URL.Query().Get("include_disabled") == "true"
	tree := r.URL.Query().Get("tree") != "false"

	menus, err := c.menuService.ListMenus(r.Context(), includeDisabled, tree)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取菜单成功", menus))
}

// ListMenuItems 获取菜单平铺列表。
// @Summary 获取菜单平铺列表
// @Description 返回菜单平铺列表，可通过 include_disabled=true 包含禁用菜单
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus/list [get]
func (c *MenuController) ListMenuItems(w http.ResponseWriter, r *http.Request) {
	includeDisabled := r.URL.Query().Get("include_disabled") == "true"

	menus, err := c.menuService.ListMenus(r.Context(), includeDisabled, false)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取菜单列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取菜单列表成功", menus))
}

// CreateMenu 创建菜单。
// @Summary 创建菜单
// @Description 创建一个菜单项
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param body body models.Menu true "菜单信息"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus [post]
func (c *MenuController) CreateMenu(w http.ResponseWriter, r *http.Request) {
	var menu models.Menu
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if strings.TrimSpace(menu.ResourceID) == "" {
		render.Render(w, r, BadRequestResponse("resource_id 不能为空", nil))
		return
	}
	if strings.TrimSpace(menu.Name) == "" || strings.TrimSpace(menu.Title) == "" {
		render.Render(w, r, BadRequestResponse("name 和 title 不能为空", nil))
		return
	}

	if err := c.menuService.CreateMenu(r.Context(), &menu); err != nil {
		render.Render(w, r, InternalErrorResponse("创建菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建菜单成功", menu))
}

// GetMenu 获取菜单详情。
// @Summary 获取菜单详情
// @Description 根据 resource_id 获取菜单详情
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param resource_id path string true "菜单资源ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus/{resource_id} [get]
func (c *MenuController) GetMenu(w http.ResponseWriter, r *http.Request) {
	resourceID := chi.URLParam(r, "resource_id")
	if resourceID == "" {
		render.Render(w, r, BadRequestResponse("resource_id 不能为空", nil))
		return
	}

	menu, err := c.menuService.GetMenu(r.Context(), resourceID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取菜单详情失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取菜单详情成功", menu))
}

// UpdateMenu 更新菜单。
// @Summary 更新菜单
// @Description 根据 resource_id 更新菜单
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param resource_id path string true "菜单资源ID"
// @Param body body models.Menu true "菜单信息"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus/{resource_id} [put]
func (c *MenuController) UpdateMenu(w http.ResponseWriter, r *http.Request) {
	resourceID := chi.URLParam(r, "resource_id")
	if resourceID == "" {
		render.Render(w, r, BadRequestResponse("resource_id 不能为空", nil))
		return
	}

	var menu models.Menu
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	menu.ResourceID = resourceID
	if strings.TrimSpace(menu.Name) == "" || strings.TrimSpace(menu.Title) == "" {
		render.Render(w, r, BadRequestResponse("name 和 title 不能为空", nil))
		return
	}

	if err := c.menuService.UpdateMenu(r.Context(), &menu); err != nil {
		render.Render(w, r, InternalErrorResponse("更新菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("更新菜单成功", menu))
}

// DeleteMenu 删除菜单。
// @Summary 删除菜单
// @Description 根据 resource_id 删除菜单
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param resource_id path string true "菜单资源ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/menus/{resource_id} [delete]
func (c *MenuController) DeleteMenu(w http.ResponseWriter, r *http.Request) {
	resourceID := chi.URLParam(r, "resource_id")
	if resourceID == "" {
		render.Render(w, r, BadRequestResponse("resource_id 不能为空", nil))
		return
	}

	if err := c.menuService.DeleteMenu(r.Context(), resourceID); err != nil {
		render.Render(w, r, InternalErrorResponse("删除菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除菜单成功", nil))
}

// GetMenusByRole 获取指定角色的菜单列表。
// @Summary 按角色查询菜单列表
// @Description 返回指定角色可访问的菜单树，admin 角色返回全部启用菜单
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param role_name path string true "角色名"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles/{role_name}/menus [get]
func (c *MenuController) GetMenusByRole(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "role_name")
	if roleName == "" {
		render.Render(w, r, BadRequestResponse("角色名不能为空", nil))
		return
	}

	menus, err := c.menuService.GetMenusForRole(r.Context(), roleName)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("按角色获取菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("按角色获取菜单成功", map[string]interface{}{
		"role_name": roleName,
		"menus":     menus,
	}))
}

// ListRoleMenus 获取角色菜单关系列表。
// @Summary 获取角色菜单关系列表
// @Description 返回角色菜单关系，可通过 role_name 过滤
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/role-menus [get]
func (c *MenuController) ListRoleMenus(w http.ResponseWriter, r *http.Request) {
	roleName := r.URL.Query().Get("role_name")

	roleMenus, err := c.menuService.GetRoleMenus(r.Context(), roleName)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取角色菜单关系列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取角色菜单关系列表成功", roleMenus))
}

// GetRoleMenus 获取指定角色的菜单资源 ID。
// @Summary 获取角色菜单关系
// @Description 返回指定角色已分配的菜单 resource_id 列表
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param role_name path string true "角色名"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/role-menus/{role_name} [get]
func (c *MenuController) GetRoleMenus(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "role_name")
	if roleName == "" {
		render.Render(w, r, BadRequestResponse("角色名不能为空", nil))
		return
	}

	resourceIDs, err := c.menuService.GetRoleMenuIDs(r.Context(), roleName)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取角色菜单权限失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取角色菜单权限成功", map[string]interface{}{
		"role_name":    roleName,
		"resource_ids": resourceIDs,
	}))
}

// UpdateRoleMenusRequest 更新角色菜单权限请求。
type UpdateRoleMenusRequest struct {
	ResourceIDs []string `json:"resource_ids"`
}

// CreateRoleMenus 创建角色菜单关系。
// @Summary 创建角色菜单关系
// @Description 为一个角色分配多个菜单项，语义为覆盖保存
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param body body UpdateRoleMenusRequest true "菜单资源ID列表"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/role-menus/{role_name} [post]
func (c *MenuController) CreateRoleMenus(w http.ResponseWriter, r *http.Request) {
	c.saveRoleMenus(w, r, "创建角色菜单关系成功")
}

// UpdateRoleMenus 更新指定角色的菜单权限。
// @Summary 更新角色菜单权限
// @Description 使用传入 resource_ids 覆盖指定角色的菜单权限
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param role_name path string true "角色名"
// @Param body body UpdateRoleMenusRequest true "菜单资源ID列表"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/role-menus/{role_name} [put]
func (c *MenuController) UpdateRoleMenus(w http.ResponseWriter, r *http.Request) {
	c.saveRoleMenus(w, r, "更新角色菜单权限成功")
}

func (c *MenuController) saveRoleMenus(w http.ResponseWriter, r *http.Request, successMessage string) {
	roleName := chi.URLParam(r, "role_name")
	if roleName == "" {
		render.Render(w, r, BadRequestResponse("角色名不能为空", nil))
		return
	}

	var req UpdateRoleMenusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}

	grantedBy := "system"
	if user := authMiddleware.GetUserFromContext(r.Context()); user != nil && user.Username != "" {
		grantedBy = user.Username
	}

	if err := c.menuService.ReplaceRoleMenus(r.Context(), roleName, req.ResourceIDs, grantedBy); err != nil {
		render.Render(w, r, InternalErrorResponse("更新角色菜单权限失败", err))
		return
	}

	render.Render(w, r, SuccessResponse(successMessage, nil))
}

// DeleteRoleMenus 删除角色菜单关系。
// @Summary 删除角色菜单关系
// @Description 删除指定角色的全部菜单关系
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param role_name path string true "角色名"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/role-menus/{role_name} [delete]
func (c *MenuController) DeleteRoleMenus(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "role_name")
	if roleName == "" {
		render.Render(w, r, BadRequestResponse("角色名不能为空", nil))
		return
	}

	if err := c.menuService.DeleteRoleMenus(r.Context(), roleName); err != nil {
		render.Render(w, r, InternalErrorResponse("删除角色菜单关系失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除角色菜单关系成功", nil))
}
