package controllers

import (
	authMiddleware "box-manage-service/api/middleware"
	"box-manage-service/service"
	"encoding/json"
	"net/http"

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
	menus, err := c.menuService.GetAllMenus(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取菜单失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取菜单成功", menus))
}

// GetRoleMenus 获取指定角色的菜单资源 ID。
// @Summary 获取角色菜单权限
// @Description 返回指定角色已分配的菜单 resource_id 列表
// @Tags 菜单权限
// @Accept json
// @Produce json
// @Param role_name path string true "角色名"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles/{role_name}/menus [get]
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
// @Router /api/v1/roles/{role_name}/menus [put]
func (c *MenuController) UpdateRoleMenus(w http.ResponseWriter, r *http.Request) {
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

	render.Render(w, r, SuccessResponse("更新角色菜单权限成功", nil))
}
