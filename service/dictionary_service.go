package service

import (
	"context"
	"fmt"
	"strings"

	"box-manage-service/models"
	"box-manage-service/repository"
)

type DictionaryListResult[T any] struct {
	Items []*T  `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page,omitempty"`
	Size  int   `json:"size,omitempty"`
}

type DictionaryService interface {
	CreateField(ctx context.Context, field *models.DictionaryFieldDefinition) error
	GetField(ctx context.Context, fieldKey string) (*models.DictionaryFieldDefinition, error)
	UpdateField(ctx context.Context, fieldKey string, field *models.DictionaryFieldDefinition) error
	DeleteField(ctx context.Context, fieldKey string) error
	ListFields(ctx context.Context, query repository.DictionaryFieldQuery) (*DictionaryListResult[models.DictionaryFieldDefinition], error)

	CreateInstance(ctx context.Context, instance *models.DictionaryInstance) error
	GetInstance(ctx context.Context, id uint) (*models.DictionaryInstance, error)
	GetInstanceByKey(ctx context.Context, fieldKey, instanceKey string) (*models.DictionaryInstance, error)
	UpdateInstance(ctx context.Context, id uint, instance *models.DictionaryInstance) error
	DeleteInstance(ctx context.Context, id uint) error
	ListInstances(ctx context.Context, query repository.DictionaryInstanceQuery) (*DictionaryListResult[models.DictionaryInstance], error)
}

type dictionaryService struct {
	repo repository.DictionaryRepository
}

func NewDictionaryService(repo repository.DictionaryRepository) DictionaryService {
	return &dictionaryService{repo: repo}
}

func (s *dictionaryService) CreateField(ctx context.Context, field *models.DictionaryFieldDefinition) error {
	if err := validateDictionaryField(field); err != nil {
		return err
	}
	return s.repo.CreateField(ctx, field)
}

func (s *dictionaryService) GetField(ctx context.Context, fieldKey string) (*models.DictionaryFieldDefinition, error) {
	fieldKey = strings.TrimSpace(fieldKey)
	if fieldKey == "" {
		return nil, fmt.Errorf("field_key 不能为空")
	}
	return s.repo.GetFieldByKey(ctx, fieldKey)
}

func (s *dictionaryService) UpdateField(ctx context.Context, fieldKey string, field *models.DictionaryFieldDefinition) error {
	fieldKey = strings.TrimSpace(fieldKey)
	if fieldKey == "" {
		return fmt.Errorf("field_key 不能为空")
	}
	existing, err := s.repo.GetFieldByKey(ctx, fieldKey)
	if err != nil {
		return err
	}
	field.ID = existing.ID
	field.FieldKey = fieldKey
	field.CreatedAt = existing.CreatedAt
	if err := validateDictionaryField(field); err != nil {
		return err
	}
	return s.repo.UpdateField(ctx, field)
}

func (s *dictionaryService) DeleteField(ctx context.Context, fieldKey string) error {
	fieldKey = strings.TrimSpace(fieldKey)
	if fieldKey == "" {
		return fmt.Errorf("field_key 不能为空")
	}
	return s.repo.DeleteFieldByKey(ctx, fieldKey)
}

func (s *dictionaryService) ListFields(ctx context.Context, query repository.DictionaryFieldQuery) (*DictionaryListResult[models.DictionaryFieldDefinition], error) {
	normalizePagination(&query.Page, &query.Size)
	items, total, err := s.repo.ListFields(ctx, query)
	if err != nil {
		return nil, err
	}
	return &DictionaryListResult[models.DictionaryFieldDefinition]{
		Items: items,
		Total: total,
		Page:  query.Page,
		Size:  query.Size,
	}, nil
}

func (s *dictionaryService) CreateInstance(ctx context.Context, instance *models.DictionaryInstance) error {
	if err := validateDictionaryInstance(instance); err != nil {
		return err
	}
	if _, err := s.repo.GetFieldByKey(ctx, instance.FieldKey); err != nil {
		return fmt.Errorf("字典字段定义不存在: %s", instance.FieldKey)
	}
	return s.repo.CreateInstance(ctx, instance)
}

func (s *dictionaryService) GetInstance(ctx context.Context, id uint) (*models.DictionaryInstance, error) {
	if id == 0 {
		return nil, fmt.Errorf("id 不能为空")
	}
	return s.repo.GetInstanceByID(ctx, id)
}

func (s *dictionaryService) GetInstanceByKey(ctx context.Context, fieldKey, instanceKey string) (*models.DictionaryInstance, error) {
	fieldKey = strings.TrimSpace(fieldKey)
	instanceKey = strings.TrimSpace(instanceKey)
	if fieldKey == "" || instanceKey == "" {
		return nil, fmt.Errorf("field_key 和 instance_key 不能为空")
	}
	return s.repo.GetInstanceByKey(ctx, fieldKey, instanceKey)
}

func (s *dictionaryService) UpdateInstance(ctx context.Context, id uint, instance *models.DictionaryInstance) error {
	if id == 0 {
		return fmt.Errorf("id 不能为空")
	}
	existing, err := s.repo.GetInstanceByID(ctx, id)
	if err != nil {
		return err
	}
	instance.ID = existing.ID
	instance.CreatedAt = existing.CreatedAt
	if strings.TrimSpace(instance.FieldKey) == "" {
		instance.FieldKey = existing.FieldKey
	}
	if err := validateDictionaryInstance(instance); err != nil {
		return err
	}
	if _, err := s.repo.GetFieldByKey(ctx, instance.FieldKey); err != nil {
		return fmt.Errorf("字典字段定义不存在: %s", instance.FieldKey)
	}
	return s.repo.UpdateInstance(ctx, instance)
}

func (s *dictionaryService) DeleteInstance(ctx context.Context, id uint) error {
	if id == 0 {
		return fmt.Errorf("id 不能为空")
	}
	return s.repo.DeleteInstance(ctx, id)
}

func (s *dictionaryService) ListInstances(ctx context.Context, query repository.DictionaryInstanceQuery) (*DictionaryListResult[models.DictionaryInstance], error) {
	normalizePagination(&query.Page, &query.Size)
	items, total, err := s.repo.ListInstances(ctx, query)
	if err != nil {
		return nil, err
	}
	return &DictionaryListResult[models.DictionaryInstance]{
		Items: items,
		Total: total,
		Page:  query.Page,
		Size:  query.Size,
	}, nil
}

func validateDictionaryField(field *models.DictionaryFieldDefinition) error {
	field.FieldKey = strings.TrimSpace(field.FieldKey)
	field.FieldName = strings.TrimSpace(field.FieldName)
	field.FieldType = strings.TrimSpace(field.FieldType)
	if field.FieldKey == "" {
		return fmt.Errorf("field_key 不能为空")
	}
	if field.FieldName == "" {
		return fmt.Errorf("field_name 不能为空")
	}
	if field.FieldType == "" {
		field.FieldType = "string"
	}
	return nil
}

func validateDictionaryInstance(instance *models.DictionaryInstance) error {
	instance.FieldKey = strings.TrimSpace(instance.FieldKey)
	instance.InstanceKey = strings.TrimSpace(instance.InstanceKey)
	instance.Label = strings.TrimSpace(instance.Label)
	if instance.FieldKey == "" {
		return fmt.Errorf("field_key 不能为空")
	}
	if instance.InstanceKey == "" {
		return fmt.Errorf("instance_key 不能为空")
	}
	if instance.Label == "" {
		return fmt.Errorf("label 不能为空")
	}
	if strings.TrimSpace(instance.Value) == "" {
		instance.Value = instance.InstanceKey
	}
	return nil
}

func normalizePagination(page *int, size *int) {
	if *page < 0 {
		*page = 0
	}
	if *size < 0 {
		*size = 0
	}
	if *page == 0 && *size > 0 {
		*page = 1
	}
	if *page > 0 && *size == 0 {
		*size = 20
	}
	if *size > 200 {
		*size = 200
	}
}
