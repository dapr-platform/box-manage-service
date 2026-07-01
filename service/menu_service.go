package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"sort"
)

// MenuIDResource 兼容前端现有 menu_ids 数据结构。
type MenuIDResource struct {
	ResourceID string `json:"resource_id"`
}

// CurrentUserContext 当前登录用户上下文。
type CurrentUserContext struct {
	ID          uint
	Username    string
	Email       string
	Role        string
	Roles       []string
	Permissions []string
}

// CurrentUserWithMenus 当前用户和菜单权限聚合信息。
type CurrentUserWithMenus struct {
	ID          uint             `json:"id"`
	Username    string           `json:"username"`
	Name        string           `json:"name"`
	Email       string           `json:"email,omitempty"`
	Role        string           `json:"role,omitempty"`
	Roles       []string         `json:"roles"`
	Permissions []string         `json:"permissions"`
	MenuIDs     []MenuIDResource `json:"menu_ids"`
	Menus       []*models.Menu   `json:"menus"`
}

// MenuService 菜单权限业务接口。
type MenuService interface {
	CreateMenu(ctx context.Context, menu *models.Menu) error
	GetMenu(ctx context.Context, resourceID string) (*models.Menu, error)
	UpdateMenu(ctx context.Context, menu *models.Menu) error
	DeleteMenu(ctx context.Context, resourceID string) error
	ListMenus(ctx context.Context, includeDisabled bool, tree bool) ([]*models.Menu, error)
	GetAllMenus(ctx context.Context) ([]*models.Menu, error)
	GetMenusForRole(ctx context.Context, roleName string) ([]*models.Menu, error)
	GetMenusForRoles(ctx context.Context, roles []string) ([]*models.Menu, error)
	GetRoleMenus(ctx context.Context, roleName string) ([]*models.RoleMenu, error)
	GetRoleMenuIDs(ctx context.Context, roleName string) ([]string, error)
	ReplaceRoleMenus(ctx context.Context, roleName string, resourceIDs []string, grantedBy string) error
	DeleteRoleMenus(ctx context.Context, roleName string) error
	GetCurrentUserWithMenus(ctx context.Context, user *CurrentUserContext) (*CurrentUserWithMenus, error)
}

type menuService struct {
	menuRepo repository.MenuRepository
}

// NewMenuService 创建菜单权限服务。
func NewMenuService(menuRepo repository.MenuRepository) MenuService {
	return &menuService{menuRepo: menuRepo}
}

func (s *menuService) CreateMenu(ctx context.Context, menu *models.Menu) error {
	return s.menuRepo.Create(ctx, menu)
}

func (s *menuService) GetMenu(ctx context.Context, resourceID string) (*models.Menu, error) {
	return s.menuRepo.GetByResourceID(ctx, resourceID)
}

func (s *menuService) UpdateMenu(ctx context.Context, menu *models.Menu) error {
	existing, err := s.menuRepo.GetByResourceID(ctx, menu.ResourceID)
	if err != nil {
		return err
	}
	menu.ID = existing.ID
	menu.CreatedAt = existing.CreatedAt
	return s.menuRepo.Update(ctx, menu)
}

func (s *menuService) DeleteMenu(ctx context.Context, resourceID string) error {
	return s.menuRepo.DeleteByResourceID(ctx, resourceID)
}

func (s *menuService) ListMenus(ctx context.Context, includeDisabled bool, tree bool) ([]*models.Menu, error) {
	menus, err := s.menuRepo.FindAll(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	if tree {
		return BuildVisibleMenuTree(menus), nil
	}
	return menus, nil
}

func (s *menuService) GetAllMenus(ctx context.Context) ([]*models.Menu, error) {
	menus, err := s.menuRepo.FindEnabled(ctx)
	if err != nil {
		return nil, err
	}
	return BuildVisibleMenuTree(menus), nil
}

func (s *menuService) GetMenusForRole(ctx context.Context, roleName string) ([]*models.Menu, error) {
	return s.GetMenusForRoles(ctx, []string{roleName})
}

func (s *menuService) GetMenusForRoles(ctx context.Context, roles []string) ([]*models.Menu, error) {
	roles = normalizeRoles(roles)
	if hasRole(roles, "admin") {
		return s.GetAllMenus(ctx)
	}

	menus, err := s.menuRepo.FindByRoleNames(ctx, roles)
	if err != nil {
		return nil, err
	}
	return BuildVisibleMenuTree(menus), nil
}

func (s *menuService) GetRoleMenus(ctx context.Context, roleName string) ([]*models.RoleMenu, error) {
	return s.menuRepo.FindRoleMenus(ctx, roleName)
}

func (s *menuService) GetRoleMenuIDs(ctx context.Context, roleName string) ([]string, error) {
	return s.menuRepo.FindRoleMenuIDs(ctx, roleName)
}

func (s *menuService) ReplaceRoleMenus(ctx context.Context, roleName string, resourceIDs []string, grantedBy string) error {
	return s.menuRepo.ReplaceRoleMenus(ctx, roleName, resourceIDs, grantedBy)
}

func (s *menuService) DeleteRoleMenus(ctx context.Context, roleName string) error {
	return s.menuRepo.DeleteRoleMenus(ctx, roleName)
}

func (s *menuService) GetCurrentUserWithMenus(ctx context.Context, user *CurrentUserContext) (*CurrentUserWithMenus, error) {
	if user == nil {
		user = &CurrentUserContext{}
	}

	roles := normalizeRoles(user.Roles)
	if len(roles) == 0 && user.Role != "" {
		roles = []string{user.Role}
	}
	if len(roles) == 0 {
		roles = []string{"guest"}
	}

	menus, err := s.GetMenusForRoles(ctx, roles)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.getMenuIDResourcesForRoles(ctx, roles)
	if err != nil {
		return nil, err
	}

	name := user.Username
	if name == "" {
		name = "system"
	}

	return &CurrentUserWithMenus{
		ID:          user.ID,
		Username:    user.Username,
		Name:        name,
		Email:       user.Email,
		Role:        user.Role,
		Roles:       roles,
		Permissions: uniqueStrings(user.Permissions),
		MenuIDs:     menuIDs,
		Menus:       menus,
	}, nil
}

// BuildMenuTree 将平铺菜单构造成树。
func BuildMenuTree(menus []*models.Menu) []*models.Menu {
	byID := make(map[string]*models.Menu, len(menus))
	roots := make([]*models.Menu, 0)

	for _, menu := range menus {
		copied := *menu
		copied.Children = nil
		byID[copied.ResourceID] = &copied
	}

	for _, menu := range byID {
		if menu.ParentID == "" {
			roots = append(roots, menu)
			continue
		}
		parent, ok := byID[menu.ParentID]
		if !ok {
			roots = append(roots, menu)
			continue
		}
		parent.Children = append(parent.Children, menu)
	}

	sortMenus(roots)
	return roots
}

// BuildVisibleMenuTree 只构造可见菜单树，隐藏路由仅作为权限资源保留。
func BuildVisibleMenuTree(menus []*models.Menu) []*models.Menu {
	visibleMenus := make([]*models.Menu, 0, len(menus))
	for _, menu := range menus {
		if menu.IsVisible {
			visibleMenus = append(visibleMenus, menu)
		}
	}
	return BuildMenuTree(visibleMenus)
}

func sortMenus(menus []*models.Menu) {
	sort.SliceStable(menus, func(i, j int) bool {
		if menus[i].SortOrder == menus[j].SortOrder {
			return menus[i].ResourceID < menus[j].ResourceID
		}
		return menus[i].SortOrder < menus[j].SortOrder
	})
	for _, menu := range menus {
		sortMenus(menu.Children)
	}
}

func collectMenuIDs(menus []*models.Menu, out *[]MenuIDResource) {
	for _, menu := range menus {
		*out = append(*out, MenuIDResource{ResourceID: menu.ResourceID})
		collectMenuIDs(menu.Children, out)
	}
}

func (s *menuService) getMenuIDResourcesForRoles(ctx context.Context, roles []string) ([]MenuIDResource, error) {
	var ids []string

	if hasRole(roles, "admin") {
		menus, menuErr := s.menuRepo.FindEnabled(ctx)
		if menuErr != nil {
			return nil, menuErr
		}
		ids = make([]string, 0, len(menus))
		for _, menu := range menus {
			ids = append(ids, menu.ResourceID)
		}
	} else {
		menus, menuErr := s.menuRepo.FindByRoleNames(ctx, roles)
		if menuErr != nil {
			return nil, menuErr
		}
		ids = make([]string, 0, len(menus))
		for _, menu := range menus {
			ids = append(ids, menu.ResourceID)
		}
	}

	ids = uniqueStrings(ids)
	menuIDs := make([]MenuIDResource, 0, len(ids))
	for _, id := range ids {
		menuIDs = append(menuIDs, MenuIDResource{ResourceID: id})
	}
	sort.SliceStable(menuIDs, func(i, j int) bool {
		return menuIDs[i].ResourceID < menuIDs[j].ResourceID
	})

	return menuIDs, nil
}

func normalizeRoles(roles []string) []string {
	return uniqueStrings(roles)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func hasRole(roles []string, target string) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}
