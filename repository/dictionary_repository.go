package repository

import (
	"context"
	"strings"

	"box-manage-service/models"

	"gorm.io/gorm"
)

type DictionaryFieldQuery struct {
	Keyword         string
	IncludeDisabled bool
	Page            int
	Size            int
}

type DictionaryInstanceQuery struct {
	FieldKey        string
	Keyword         string
	IncludeDisabled bool
	Page            int
	Size            int
}

type DictionaryRepository interface {
	CreateField(ctx context.Context, field *models.DictionaryFieldDefinition) error
	GetFieldByKey(ctx context.Context, fieldKey string) (*models.DictionaryFieldDefinition, error)
	UpdateField(ctx context.Context, field *models.DictionaryFieldDefinition) error
	DeleteFieldByKey(ctx context.Context, fieldKey string) error
	ListFields(ctx context.Context, query DictionaryFieldQuery) ([]*models.DictionaryFieldDefinition, int64, error)

	CreateInstance(ctx context.Context, instance *models.DictionaryInstance) error
	GetInstanceByID(ctx context.Context, id uint) (*models.DictionaryInstance, error)
	GetInstanceByKey(ctx context.Context, fieldKey, instanceKey string) (*models.DictionaryInstance, error)
	UpdateInstance(ctx context.Context, instance *models.DictionaryInstance) error
	DeleteInstance(ctx context.Context, id uint) error
	ListInstances(ctx context.Context, query DictionaryInstanceQuery) ([]*models.DictionaryInstance, int64, error)
}

type dictionaryRepository struct {
	db *gorm.DB
}

func NewDictionaryRepository(db *gorm.DB) DictionaryRepository {
	return &dictionaryRepository{db: db}
}

func (r *dictionaryRepository) CreateField(ctx context.Context, field *models.DictionaryFieldDefinition) error {
	return r.db.WithContext(ctx).Create(field).Error
}

func (r *dictionaryRepository) GetFieldByKey(ctx context.Context, fieldKey string) (*models.DictionaryFieldDefinition, error) {
	var field models.DictionaryFieldDefinition
	if err := r.db.WithContext(ctx).Where("field_key = ?", fieldKey).First(&field).Error; err != nil {
		return nil, err
	}
	return &field, nil
}

func (r *dictionaryRepository) UpdateField(ctx context.Context, field *models.DictionaryFieldDefinition) error {
	return r.db.WithContext(ctx).Save(field).Error
}

func (r *dictionaryRepository) DeleteFieldByKey(ctx context.Context, fieldKey string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("field_key = ?", fieldKey).Delete(&models.DictionaryInstance{}).Error; err != nil {
			return err
		}
		return tx.Where("field_key = ?", fieldKey).Delete(&models.DictionaryFieldDefinition{}).Error
	})
}

func (r *dictionaryRepository) ListFields(ctx context.Context, query DictionaryFieldQuery) ([]*models.DictionaryFieldDefinition, int64, error) {
	var fields []*models.DictionaryFieldDefinition
	var total int64

	dbQuery := r.db.WithContext(ctx).Model(&models.DictionaryFieldDefinition{})
	if !query.IncludeDisabled {
		dbQuery = dbQuery.Where("is_enabled = ?", true)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		dbQuery = dbQuery.Where("field_key LIKE ? OR field_name LIKE ? OR description LIKE ?", like, like, like)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	dbQuery = dbQuery.Order("sort_order ASC, field_key ASC")
	if query.Page > 0 && query.Size > 0 {
		dbQuery = dbQuery.Offset((query.Page - 1) * query.Size).Limit(query.Size)
	}
	if err := dbQuery.Find(&fields).Error; err != nil {
		return nil, 0, err
	}
	return fields, total, nil
}

func (r *dictionaryRepository) CreateInstance(ctx context.Context, instance *models.DictionaryInstance) error {
	return r.db.WithContext(ctx).Create(instance).Error
}

func (r *dictionaryRepository) GetInstanceByID(ctx context.Context, id uint) (*models.DictionaryInstance, error) {
	var instance models.DictionaryInstance
	if err := r.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

func (r *dictionaryRepository) GetInstanceByKey(ctx context.Context, fieldKey, instanceKey string) (*models.DictionaryInstance, error) {
	var instance models.DictionaryInstance
	if err := r.db.WithContext(ctx).
		Where("field_key = ? AND instance_key = ?", fieldKey, instanceKey).
		First(&instance).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

func (r *dictionaryRepository) UpdateInstance(ctx context.Context, instance *models.DictionaryInstance) error {
	return r.db.WithContext(ctx).Save(instance).Error
}

func (r *dictionaryRepository) DeleteInstance(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.DictionaryInstance{}, id).Error
}

func (r *dictionaryRepository) ListInstances(ctx context.Context, query DictionaryInstanceQuery) ([]*models.DictionaryInstance, int64, error) {
	var instances []*models.DictionaryInstance
	var total int64

	dbQuery := r.db.WithContext(ctx).Model(&models.DictionaryInstance{})
	if fieldKey := strings.TrimSpace(query.FieldKey); fieldKey != "" {
		dbQuery = dbQuery.Where("field_key = ?", fieldKey)
	}
	if !query.IncludeDisabled {
		dbQuery = dbQuery.Where("is_enabled = ?", true)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		dbQuery = dbQuery.Where("instance_key LIKE ? OR label LIKE ? OR value LIKE ? OR remark LIKE ?", like, like, like, like)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	dbQuery = dbQuery.Order("field_key ASC, sort_order ASC, instance_key ASC")
	if query.Page > 0 && query.Size > 0 {
		dbQuery = dbQuery.Offset((query.Page - 1) * query.Size).Limit(query.Size)
	}
	if err := dbQuery.Find(&instances).Error; err != nil {
		return nil, 0, err
	}
	return instances, total, nil
}
