/*
 * @module service/executors/data_transform_executor
 * @description 数据转换节点执行器
 * @architecture 策略模式 - 数据转换执行器实现
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 读取输入数据 -> 应用转换规则 -> 输出转换结果
 * @rules 支持JSON路径提取、字段映射、类型转换、数据过滤等操作
 * @dependencies models, encoding/json
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// DataTransformExecutor 数据转换执行器
type DataTransformExecutor struct {
	*BaseExecutor
}

// NewDataTransformExecutor 创建数据转换执行器
func NewDataTransformExecutor() *DataTransformExecutor {
	return &DataTransformExecutor{
		BaseExecutor: NewBaseExecutor("data_transform"),
	}
}

// Execute 执行数据转换
func (e *DataTransformExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行数据转换节点"}

	// 解析配置
	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 获取输入数据
	inputData, ok := execCtx.Inputs["input_data"]
	if !ok {
		err := fmt.Errorf("缺少输入数据: input_data")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("输入数据类型: %T", inputData))

	// 执行转换
	result, err := e.transform(inputData, config, logs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("数据转换失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, "数据转换成功")

	outputs := map[string]interface{}{
		"output_data":    result,
		"transform_type": config.TransformType,
	}

	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *DataTransformExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// DataTransformConfig 数据转换配置
type DataTransformConfig struct {
	TransformType string            `json:"transform_type"` // map/filter/extract/convert
	Mappings      map[string]string `json:"mappings"`       // 字段映射规则
	FilterRules   []FilterRule      `json:"filter_rules"`   // 过滤规则
	ExtractPath   string            `json:"extract_path"`   // JSON路径提取
	ConvertType   string            `json:"convert_type"`   // 类型转换目标
	CustomScript  string            `json:"custom_script"`  // 自定义转换脚本（预留）
}

// FilterRule 过滤规则
type FilterRule struct {
	Field    string      `json:"field"`    // 字段名
	Operator string      `json:"operator"` // 操作符: eq/ne/gt/lt/contains
	Value    interface{} `json:"value"`    // 比较值
}

// parseConfig 解析配置
func (e *DataTransformExecutor) parseConfig(inputs map[string]interface{}) (*DataTransformConfig, error) {
	config := &DataTransformConfig{
		TransformType: "map",
		Mappings:      make(map[string]string),
	}

	if transformType, ok := inputs["transform_type"].(string); ok {
		config.TransformType = transformType
	}

	if mappings, ok := inputs["mappings"].(map[string]interface{}); ok {
		for k, v := range mappings {
			if strVal, ok := v.(string); ok {
				config.Mappings[k] = strVal
			}
		}
	}

	if extractPath, ok := inputs["extract_path"].(string); ok {
		config.ExtractPath = extractPath
	}

	if convertType, ok := inputs["convert_type"].(string); ok {
		config.ConvertType = convertType
	}

	return config, nil
}

// transform 执行转换
func (e *DataTransformExecutor) transform(data interface{}, config *DataTransformConfig, logs []string) (interface{}, error) {
	switch config.TransformType {
	case "map":
		return e.mapTransform(data, config.Mappings, logs)
	case "extract":
		return e.extractTransform(data, config.ExtractPath, logs)
	case "convert":
		return e.convertTransform(data, config.ConvertType, logs)
	case "filter":
		return e.filterTransform(data, config.FilterRules, logs)
	default:
		return nil, fmt.Errorf("不支持的转换类型: %s", config.TransformType)
	}
}

// mapTransform 字段映射转换
func (e *DataTransformExecutor) mapTransform(data interface{}, mappings map[string]string, logs []string) (interface{}, error) {
	result := make(map[string]interface{})

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("map转换要求输入数据为对象类型")
	}

	for targetField, sourceField := range mappings {
		if value, exists := dataMap[sourceField]; exists {
			result[targetField] = value
		}
	}

	logs = append(logs, fmt.Sprintf("映射了 %d 个字段", len(result)))
	return result, nil
}

// extractTransform JSON路径提取
func (e *DataTransformExecutor) extractTransform(data interface{}, path string, logs []string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	// 简单的点号路径解析（如 "user.name"）
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if value, exists := currentMap[part]; exists {
				current = value
			} else {
				return nil, fmt.Errorf("路径不存在: %s", part)
			}
		} else {
			return nil, fmt.Errorf("无法从非对象类型提取字段: %s", part)
		}
	}

	logs = append(logs, fmt.Sprintf("提取路径: %s", path))
	return current, nil
}

// convertTransform 类型转换
func (e *DataTransformExecutor) convertTransform(data interface{}, targetType string, logs []string) (interface{}, error) {
	switch targetType {
	case "string":
		return fmt.Sprintf("%v", data), nil
	case "number":
		return e.toNumber(data)
	case "boolean":
		return e.toBoolean(data)
	case "array":
		return e.toArray(data)
	case "json":
		return json.Marshal(data)
	default:
		return nil, fmt.Errorf("不支持的目标类型: %s", targetType)
	}
}

// filterTransform 数据过滤
func (e *DataTransformExecutor) filterTransform(data interface{}, rules []FilterRule, logs []string) (interface{}, error) {
	// 如果是数组，过滤数组元素
	if arr, ok := data.([]interface{}); ok {
		var filtered []interface{}
		for _, item := range arr {
			if e.matchesRules(item, rules) {
				filtered = append(filtered, item)
			}
		}
		logs = append(logs, fmt.Sprintf("过滤后保留 %d/%d 条记录", len(filtered), len(arr)))
		return filtered, nil
	}

	// 如果是单个对象，检查是否匹配规则
	if e.matchesRules(data, rules) {
		return data, nil
	}

	return nil, nil
}

// matchesRules 检查数据是否匹配所有规则
func (e *DataTransformExecutor) matchesRules(data interface{}, rules []FilterRule) bool {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return false
	}

	for _, rule := range rules {
		if !e.matchRule(dataMap, rule) {
			return false
		}
	}
	return true
}

// matchRule 检查单个规则
func (e *DataTransformExecutor) matchRule(data map[string]interface{}, rule FilterRule) bool {
	value, exists := data[rule.Field]
	if !exists {
		return false
	}

	switch rule.Operator {
	case "eq":
		return reflect.DeepEqual(value, rule.Value)
	case "ne":
		return !reflect.DeepEqual(value, rule.Value)
	case "contains":
		strVal := fmt.Sprintf("%v", value)
		strRule := fmt.Sprintf("%v", rule.Value)
		return strings.Contains(strVal, strRule)
	default:
		return false
	}
}

// toNumber 转换为数字
func (e *DataTransformExecutor) toNumber(data interface{}) (float64, error) {
	switch v := data.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("无法转换为数字: %T", data)
	}
}

// toBoolean 转换为布尔值
func (e *DataTransformExecutor) toBoolean(data interface{}) (bool, error) {
	switch v := data.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	default:
		return false, fmt.Errorf("无法转换为布尔值: %T", data)
	}
}

// toArray 转换为数组
func (e *DataTransformExecutor) toArray(data interface{}) ([]interface{}, error) {
	if arr, ok := data.([]interface{}); ok {
		return arr, nil
	}
	// 单个元素包装为数组
	return []interface{}{data}, nil
}
