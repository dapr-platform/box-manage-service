package controllers

// PostgRESTRPCResponse PostgREST RPC 通用响应。
type PostgRESTRPCResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"操作成功"`
}

// PostgRESTGetTokenRequest 登录请求。
type PostgRESTGetTokenRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" example:"password"`
}

// PostgRESTGetTokenResponse 登录响应。
type PostgRESTGetTokenResponse struct {
	Success      bool                     `json:"success" example:"true"`
	Message      string                   `json:"message" example:"登录成功"`
	AccessToken  string                   `json:"access_token,omitempty"`
	RefreshToken string                   `json:"refresh_token,omitempty"`
	TokenType    string                   `json:"token_type,omitempty" example:"bearer"`
	ExpiresIn    int                      `json:"expires_in,omitempty" example:"900"`
	UserInfo     map[string]interface{}   `json:"user_info,omitempty"`
	Roles        []string                 `json:"roles,omitempty" example:"admin,user"`
	Permissions  []string                 `json:"permissions,omitempty" example:"system.admin,users.select"`
	MenuIDs      []map[string]string      `json:"menu_ids,omitempty"`
	Menus        []map[string]interface{} `json:"menus,omitempty"`
}

// PostgRESTRefreshTokenRequest 刷新 token 请求。
type PostgRESTRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// PostgRESTVerifyTokenRequest 校验 token 请求。
type PostgRESTVerifyTokenRequest struct {
	Token string `json:"token"`
}

// PostgRESTAddUserRequest 创建用户请求。
type PostgRESTAddUserRequest struct {
	UserName      string `json:"user_name" example:"zhangsan"`
	UserPassword  string `json:"user_password" example:"123456"`
	TargetSchemas string `json:"target_schemas" example:"postgrest,public"`
	Email         string `json:"email,omitempty" example:"zhangsan@example.com"`
	FullName      string `json:"full_name,omitempty" example:"Zhang San"`
	DisplayName   string `json:"display_name,omitempty" example:"张三"`
	DefaultRole   string `json:"default_role,omitempty" example:"user"`
}

// PostgRESTUpdateUserRequest 更新用户请求。
type PostgRESTUpdateUserRequest struct {
	UserName    string   `json:"user_name" example:"zhangsan"`
	Email       string   `json:"email,omitempty" example:"zhangsan@example.com"`
	FullName    string   `json:"full_name,omitempty" example:"Zhang San"`
	DisplayName string   `json:"display_name,omitempty" example:"张三"`
	IsActive    *bool    `json:"is_active,omitempty" example:"true"`
	NewPassword string   `json:"new_password,omitempty" example:"123456"`
	Roles       []string `json:"roles,omitempty" example:"user,readonly"`
}

// PostgRESTUpdateUserSchemasRequest 更新用户 schema 权限请求。
type PostgRESTUpdateUserSchemasRequest struct {
	UserName         string `json:"user_name" example:"zhangsan"`
	NewTargetSchemas string `json:"new_target_schemas" example:"postgrest,public"`
}

// PostgRESTChangePasswordRequest 修改密码请求。
type PostgRESTChangePasswordRequest struct {
	UserName    string `json:"user_name" example:"zhangsan"`
	NewPassword string `json:"new_password" example:"123456"`
}

// PostgRESTDeleteUserRequest 删除用户请求。
type PostgRESTDeleteUserRequest struct {
	UserName    string `json:"user_name" example:"zhangsan"`
	ForceDelete bool   `json:"force_delete,omitempty" example:"false"`
}

// PostgRESTReactivateUserRequest 重新激活用户请求。
type PostgRESTReactivateUserRequest struct {
	UserName string `json:"user_name" example:"zhangsan"`
}

// PostgRESTAssignRoleRequest 分配角色请求。
type PostgRESTAssignRoleRequest struct {
	UserName   string `json:"p_user_name" example:"zhangsan"`
	RoleName   string `json:"p_role_name" example:"user"`
	AssignedBy string `json:"p_assigned_by,omitempty" example:"admin"`
	ExpiresAt  string `json:"p_expires_at,omitempty" example:"2026-12-31T23:59:59Z"`
}

// PostgRESTRevokeRoleRequest 撤销角色请求。
type PostgRESTRevokeRoleRequest struct {
	UserName string `json:"user_name" example:"zhangsan"`
	RoleName string `json:"role_name" example:"user"`
}

// PostgRESTCreateRoleRequest 创建角色请求。
type PostgRESTCreateRoleRequest struct {
	RoleName     string `json:"role_name" example:"operator"`
	Description  string `json:"description,omitempty" example:"运维人员"`
	DisplayName  string `json:"display_name,omitempty" example:"运维人员"`
	IsSystemRole bool   `json:"is_system_role,omitempty" example:"false"`
}

// PostgRESTUpdateRoleRequest 更新角色请求。
type PostgRESTUpdateRoleRequest struct {
	RoleName        string `json:"role_name" example:"operator"`
	NewDescription  string `json:"new_description,omitempty" example:"运维人员"`
	NewDisplayName  string `json:"new_display_name,omitempty" example:"运维人员"`
	NewIsSystemRole *bool  `json:"new_is_system_role,omitempty" example:"false"`
}

// PostgRESTDeleteRoleRequest 删除角色请求。
type PostgRESTDeleteRoleRequest struct {
	RoleName    string `json:"role_name" example:"operator"`
	ForceDelete bool   `json:"force_delete,omitempty" example:"false"`
}

// PostgRESTRoleResponse 角色列表响应项。
type PostgRESTRoleResponse struct {
	RoleName        string                   `json:"role_name" example:"admin"`
	Description     string                   `json:"description,omitempty"`
	DisplayName     string                   `json:"display_name,omitempty" example:"系统管理员"`
	IsSystemRole    bool                     `json:"is_system_role" example:"true"`
	CreatedAt       string                   `json:"created_at,omitempty"`
	UpdatedAt       string                   `json:"updated_at,omitempty"`
	UserCount       int64                    `json:"user_count" example:"1"`
	PermissionCount int64                    `json:"permission_count" example:"12"`
	MenuIDs         []map[string]string      `json:"menu_ids,omitempty"`
	Menus           []map[string]interface{} `json:"menus,omitempty"`
}

// PostgRESTCreatePermissionRequest 创建权限请求。
type PostgRESTCreatePermissionRequest struct {
	PermissionName string `json:"permission_name" example:"system.admin"`
	Description    string `json:"description,omitempty" example:"系统管理权限"`
	DisplayName    string `json:"display_name,omitempty" example:"系统管理"`
	ResourceType   string `json:"resource_type,omitempty" example:"system"`
	ActionType     string `json:"action_type,omitempty" example:"admin"`
}

// PostgRESTUpdatePermissionRequest 更新权限请求。
type PostgRESTUpdatePermissionRequest struct {
	PermissionName  string `json:"permission_name" example:"system.admin"`
	NewDescription  string `json:"new_description,omitempty" example:"系统管理权限"`
	NewDisplayName  string `json:"new_display_name,omitempty" example:"系统管理"`
	NewResourceType string `json:"new_resource_type,omitempty" example:"system"`
	NewActionType   string `json:"new_action_type,omitempty" example:"admin"`
}

// PostgRESTDeletePermissionRequest 删除权限请求。
type PostgRESTDeletePermissionRequest struct {
	PermissionName string `json:"permission_name" example:"system.admin"`
	ForceDelete    bool   `json:"force_delete,omitempty" example:"false"`
}

// PostgRESTPermissionResponse 权限列表响应项。
type PostgRESTPermissionResponse struct {
	PermissionName string `json:"permission_name" example:"system.admin"`
	Description    string `json:"description,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	ResourceType   string `json:"resource_type,omitempty"`
	ActionType     string `json:"action_type,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	RoleCount      int64  `json:"role_count" example:"1"`
}

// PostgRESTGrantPermissionToRoleRequest 为角色分配权限请求。
type PostgRESTGrantPermissionToRoleRequest struct {
	RoleName       string `json:"p_role_name" example:"operator"`
	PermissionName string `json:"p_permission_name" example:"system.admin"`
	GrantedBy      string `json:"p_granted_by,omitempty" example:"admin"`
}

// PostgRESTRevokePermissionFromRoleRequest 从角色撤销权限请求。
type PostgRESTRevokePermissionFromRoleRequest struct {
	RoleName       string `json:"p_role_name" example:"operator"`
	PermissionName string `json:"p_permission_name" example:"system.admin"`
}

// PostgRESTGetRolePermissionsRequest 获取角色权限请求。
type PostgRESTGetRolePermissionsRequest struct {
	RoleName string `json:"p_role_name" example:"operator"`
}

// PostgRESTRolePermissionResponse 角色权限响应项。
type PostgRESTRolePermissionResponse struct {
	PermissionName string `json:"permission_name" example:"system.admin"`
	Description    string `json:"description,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	ResourceType   string `json:"resource_type,omitempty"`
	ActionType     string `json:"action_type,omitempty"`
	GrantedAt      string `json:"granted_at,omitempty"`
	GrantedBy      string `json:"granted_by,omitempty"`
}

// postgRESTGetToken godoc
// @Summary PostgREST 登录获取 Token
// @Description 调用 postgrest.get_token，成功后返回 access_token、refresh_token、角色、权限和菜单权限数据。
// @Tags PostgREST RPC - 认证
// @Accept json
// @Produce json
// @Param body body PostgRESTGetTokenRequest true "登录参数"
// @Success 200 {object} PostgRESTGetTokenResponse
// @Router /api/postgrest/rpc/get_token [post]
func postgRESTGetToken() {}

// postgRESTRefreshToken godoc
// @Summary PostgREST 刷新 Token
// @Description 调用 postgrest.refresh_token，使用 refresh_token 换取新的 access_token。
// @Tags PostgREST RPC - 认证
// @Accept json
// @Produce json
// @Param body body PostgRESTRefreshTokenRequest true "刷新 Token 参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/refresh_token [post]
func postgRESTRefreshToken() {}

// postgRESTVerifyToken godoc
// @Summary PostgREST 校验 Token
// @Description 调用 postgrest.verify_token，校验 JWT 是否有效。
// @Tags PostgREST RPC - 认证
// @Accept json
// @Produce json
// @Param body body PostgRESTVerifyTokenRequest true "Token 参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/verify_token [post]
func postgRESTVerifyToken() {}

// postgRESTAddUser godoc
// @Summary PostgREST 创建用户
// @Description 调用 postgrest.add_user，创建数据库用户并分配默认角色。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTAddUserRequest true "创建用户参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/add_user [post]
func postgRESTAddUser() {}

// postgRESTUpdateUser godoc
// @Summary PostgREST 更新用户
// @Description 调用 postgrest.update_user，更新用户资料、启停状态、密码和角色。roles 不传表示不改角色，传空数组表示清空角色。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTUpdateUserRequest true "更新用户参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/update_user [post]
func postgRESTUpdateUser() {}

// postgRESTUpdateUserSchemas godoc
// @Summary PostgREST 更新用户 Schema 权限
// @Description 调用 postgrest.update_user_schemas，更新用户可访问的数据库 schema。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTUpdateUserSchemasRequest true "更新 Schema 参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/update_user_schemas [post]
func postgRESTUpdateUserSchemas() {}

// postgRESTChangePassword godoc
// @Summary PostgREST 修改用户密码
// @Description 调用 postgrest.change_password，管理员可修改任意用户密码，普通用户只能修改自己的密码。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTChangePasswordRequest true "修改密码参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/change_password [post]
func postgRESTChangePassword() {}

// postgRESTDeleteUser godoc
// @Summary PostgREST 删除或禁用用户
// @Description 调用 postgrest.delete_user，force_delete=false 时软删除禁用用户，true 时强制删除。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTDeleteUserRequest true "删除用户参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/delete_user [post]
func postgRESTDeleteUser() {}

// postgRESTListUsers godoc
// @Summary PostgREST 用户列表
// @Description 调用 postgrest.list_users，返回所有用户及角色信息。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {array} object
// @Router /api/postgrest/rpc/list_users [post]
func postgRESTListUsers() {}

// postgRESTReactivateUser godoc
// @Summary PostgREST 重新激活用户
// @Description 调用 postgrest.reactivate_user，重新启用被禁用的用户。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTReactivateUserRequest true "激活用户参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/reactivate_user [post]
func postgRESTReactivateUser() {}

// postgRESTAssignRole godoc
// @Summary PostgREST 为用户分配角色
// @Description 调用 postgrest.assign_role，为用户分配角色。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTAssignRoleRequest true "分配角色参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/assign_role [post]
func postgRESTAssignRole() {}

// postgRESTRevokeRole godoc
// @Summary PostgREST 撤销用户角色
// @Description 调用 postgrest.revoke_role，撤销用户角色。
// @Tags PostgREST RPC - 用户
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTRevokeRoleRequest true "撤销角色参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/revoke_role [post]
func postgRESTRevokeRole() {}

// postgRESTCreateRole godoc
// @Summary PostgREST 创建角色
// @Description 调用 postgrest.create_role，创建角色。
// @Tags PostgREST RPC - 角色
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTCreateRoleRequest true "创建角色参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/create_role [post]
func postgRESTCreateRole() {}

// postgRESTUpdateRole godoc
// @Summary PostgREST 更新角色
// @Description 调用 postgrest.update_role，更新角色信息。
// @Tags PostgREST RPC - 角色
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTUpdateRoleRequest true "更新角色参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/update_role [post]
func postgRESTUpdateRole() {}

// postgRESTDeleteRole godoc
// @Summary PostgREST 删除角色
// @Description 调用 postgrest.delete_role，删除角色。系统角色或仍有用户使用的角色需要 force_delete=true。
// @Tags PostgREST RPC - 角色
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTDeleteRoleRequest true "删除角色参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/delete_role [post]
func postgRESTDeleteRole() {}

// postgRESTListRoles godoc
// @Summary PostgREST 角色列表
// @Description 调用 postgrest.list_roles，返回角色、用户数量、菜单权限数量以及已配置菜单项。
// @Tags PostgREST RPC - 角色
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {array} PostgRESTRoleResponse
// @Router /api/postgrest/rpc/list_roles [post]
func postgRESTListRoles() {}

// postgRESTCreatePermission godoc
// @Summary PostgREST 创建权限
// @Description 调用 postgrest.create_permission，创建权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTCreatePermissionRequest true "创建权限参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/create_permission [post]
func postgRESTCreatePermission() {}

// postgRESTUpdatePermission godoc
// @Summary PostgREST 更新权限
// @Description 调用 postgrest.update_permission，更新权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTUpdatePermissionRequest true "更新权限参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/update_permission [post]
func postgRESTUpdatePermission() {}

// postgRESTDeletePermission godoc
// @Summary PostgREST 删除权限
// @Description 调用 postgrest.delete_permission，删除权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTDeletePermissionRequest true "删除权限参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/delete_permission [post]
func postgRESTDeletePermission() {}

// postgRESTListPermissions godoc
// @Summary PostgREST 权限列表
// @Description 调用 postgrest.list_permissions，返回权限点列表。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {array} PostgRESTPermissionResponse
// @Router /api/postgrest/rpc/list_permissions [post]
func postgRESTListPermissions() {}

// postgRESTGrantPermissionToRole godoc
// @Summary PostgREST 为角色分配权限
// @Description 调用 postgrest.grant_permission_to_role，为角色分配权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTGrantPermissionToRoleRequest true "分配权限参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/grant_permission_to_role [post]
func postgRESTGrantPermissionToRole() {}

// postgRESTRevokePermissionFromRole godoc
// @Summary PostgREST 从角色撤销权限
// @Description 调用 postgrest.revoke_permission_from_role，从角色撤销权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTRevokePermissionFromRoleRequest true "撤销权限参数"
// @Success 200 {object} PostgRESTRPCResponse
// @Router /api/postgrest/rpc/revoke_permission_from_role [post]
func postgRESTRevokePermissionFromRole() {}

// postgRESTGetRolePermissions godoc
// @Summary PostgREST 获取角色权限
// @Description 调用 postgrest.get_role_permissions，返回角色已配置的权限点。
// @Tags PostgREST RPC - 权限
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostgRESTGetRolePermissionsRequest true "查询角色权限参数"
// @Success 200 {array} PostgRESTRolePermissionResponse
// @Router /api/postgrest/rpc/get_role_permissions [post]
func postgRESTGetRolePermissions() {}
