package repository

import (
	"box-manage-service/models"
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MenuRepository 菜单权限数据访问接口。
type MenuRepository interface {
	BaseRepository[models.Menu]
	FindAll(ctx context.Context, includeDisabled bool) ([]*models.Menu, error)
	FindEnabled(ctx context.Context) ([]*models.Menu, error)
	GetByResourceID(ctx context.Context, resourceID string) (*models.Menu, error)
	DeleteByResourceID(ctx context.Context, resourceID string) error
	FindByRoleNames(ctx context.Context, roleNames []string) ([]*models.Menu, error)
	FindRoleMenus(ctx context.Context, roleName string) ([]*models.RoleMenu, error)
	FindRoleMenuIDs(ctx context.Context, roleName string) ([]string, error)
	ReplaceRoleMenus(ctx context.Context, roleName string, resourceIDs []string, grantedBy string) error
	DeleteRoleMenus(ctx context.Context, roleName string) error
	GrantAllMenusToRole(ctx context.Context, roleName, grantedBy string) error
}

type menuRepository struct {
	BaseRepository[models.Menu]
	db *gorm.DB
}

// NewMenuRepository 创建菜单权限 Repository。
func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{
		BaseRepository: newBaseRepository[models.Menu](db),
		db:             db,
	}
}

func (r *menuRepository) FindAll(ctx context.Context, includeDisabled bool) ([]*models.Menu, error) {
	var menus []*models.Menu
	query := r.db.WithContext(ctx).Model(&models.Menu{})
	if !includeDisabled {
		query = query.Where("is_enabled = ?", true)
	}
	err := query.Order("sort_order ASC, resource_id ASC").Find(&menus).Error
	return menus, err
}

func (r *menuRepository) FindEnabled(ctx context.Context) ([]*models.Menu, error) {
	var menus []*models.Menu
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Order("sort_order ASC, resource_id ASC").
		Find(&menus).Error
	return menus, err
}

func (r *menuRepository) GetByResourceID(ctx context.Context, resourceID string) (*models.Menu, error) {
	var menu models.Menu
	if err := r.db.WithContext(ctx).
		Where("resource_id = ?", resourceID).
		First(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) DeleteByResourceID(ctx context.Context, resourceID string) error {
	return r.db.WithContext(ctx).
		Where("resource_id = ?", resourceID).
		Delete(&models.Menu{}).Error
}

func (r *menuRepository) FindByRoleNames(ctx context.Context, roleNames []string) ([]*models.Menu, error) {
	if len(roleNames) == 0 {
		return []*models.Menu{}, nil
	}

	var menus []*models.Menu
	err := r.db.WithContext(ctx).
		Model(&models.Menu{}).
		Joins("JOIN postgrest.role_menus ON postgrest.role_menus.resource_id = postgrest.menus.resource_id").
		Where("postgrest.role_menus.role_name IN ?", roleNames).
		Where("postgrest.menus.is_enabled = ?", true).
		Group("postgrest.menus.id").
		Order("postgrest.menus.sort_order ASC, postgrest.menus.resource_id ASC").
		Find(&menus).Error
	return menus, err
}

func (r *menuRepository) FindRoleMenus(ctx context.Context, roleName string) ([]*models.RoleMenu, error) {
	var roleMenus []*models.RoleMenu
	query := r.db.WithContext(ctx).Model(&models.RoleMenu{})
	if roleName != "" {
		query = query.Where("role_name = ?", roleName)
	}
	err := query.Order("role_name ASC, resource_id ASC").Find(&roleMenus).Error
	return roleMenus, err
}

func (r *menuRepository) FindRoleMenuIDs(ctx context.Context, roleName string) ([]string, error) {
	var resourceIDs []string
	err := r.db.WithContext(ctx).
		Model(&models.RoleMenu{}).
		Where("role_name = ?", roleName).
		Order("resource_id ASC").
		Pluck("resource_id", &resourceIDs).Error
	return resourceIDs, err
}

func (r *menuRepository) ReplaceRoleMenus(ctx context.Context, roleName string, resourceIDs []string, grantedBy string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_name = ?", roleName).Delete(&models.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(resourceIDs) == 0 {
			return nil
		}

		roleMenus := make([]models.RoleMenu, 0, len(resourceIDs))
		seen := make(map[string]struct{}, len(resourceIDs))
		for _, resourceID := range resourceIDs {
			if resourceID == "" {
				continue
			}
			if _, ok := seen[resourceID]; ok {
				continue
			}
			seen[resourceID] = struct{}{}
			roleMenus = append(roleMenus, models.RoleMenu{
				RoleName:   roleName,
				ResourceID: resourceID,
				GrantedBy:  grantedBy,
			})
		}
		if len(roleMenus) == 0 {
			return nil
		}
		return tx.Create(&roleMenus).Error
	})
}

func (r *menuRepository) DeleteRoleMenus(ctx context.Context, roleName string) error {
	return r.db.WithContext(ctx).
		Where("role_name = ?", roleName).
		Delete(&models.RoleMenu{}).Error
}

func (r *menuRepository) GrantAllMenusToRole(ctx context.Context, roleName, grantedBy string) error {
	var resourceIDs []string
	if err := r.db.WithContext(ctx).
		Model(&models.Menu{}).
		Where("is_enabled = ?", true).
		Pluck("resource_id", &resourceIDs).Error; err != nil {
		return err
	}

	if len(resourceIDs) == 0 {
		return nil
	}

	roleMenus := make([]models.RoleMenu, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		roleMenus = append(roleMenus, models.RoleMenu{
			RoleName:   roleName,
			ResourceID: resourceID,
			GrantedBy:  grantedBy,
		})
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&roleMenus).Error
}
