/*
 * @module service/executors/python_script_executor
 * @description Python脚本节点执行器
 * @architecture 策略模式 - 实现Python脚本节点的执行逻辑
 * @documentReference 业务编排引擎设计文档
 * @stateFlow Python脚本执行：解析入参 -> 变量替换 -> 执行脚本 -> 解析出参 -> 写回变量
 * @rules 支持多个入参（从变量上下文注入）和多个出参（写回变量上下文）
 */

package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// PythonScriptExecutor Python脚本节点执行器
type PythonScriptExecutor struct {
	*BaseExecutor
}

// NewPythonScriptExecutor 创建Python脚本执行器
func NewPythonScriptExecutor() *PythonScriptExecutor {
	return &PythonScriptExecutor{
		BaseExecutor: NewBaseExecutor("python_script"),
	}
}

// ScriptParam 脚本参数定义
type ScriptParam struct {
	// 参数名，对应脚本中的变量名
	Name string `json:"name"`
	// 参数类型：string / number / boolean / object / array
	Type string `json:"type"`
	// 入参：从变量上下文中读取的 key（支持 "node_key.field" 引用格式）
	// 出参：写回变量上下文的 key
	VarKey string `json:"var_key"`
	// 默认值（仅入参使用，当 var_key 对应的变量不存在时使用）
	DefaultValue interface{} `json:"default_value,omitempty"`
}

// PythonScriptConfig Python脚本节点配置
type PythonScriptConfig struct {
	// 脚本内容，支持 {{param_name}} 占位符替换
	ScriptContent string `json:"script_content"`
	// 入参列表：执行前从变量上下文注入到脚本
	InputParams []ScriptParam `json:"input_params"`
	// 出参列表：脚本执行后从输出 JSON 中提取并写回变量上下文
	OutputParams []ScriptParam `json:"output_params"`
	// 超时时间（秒），默认 30
	TimeoutSeconds int `json:"timeout_seconds"`
	// Python 解释器路径，默认 "python3"
	PythonBin string `json:"python_bin"`
}

// Execute 执行Python脚本节点
func (e *PythonScriptExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"Python脚本节点开始执行"}

	// 解析配置
	cfg, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}

	if cfg.ScriptContent == "" {
		err := fmt.Errorf("script_content 不能为空")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), nil
	}

	// Step 1: 解析入参，从变量上下文中取值
	inputValues, err := e.resolveInputParams(cfg.InputParams, execCtx)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析入参失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}
	logs = append(logs, fmt.Sprintf("入参解析完成，共 %d 个参数", len(inputValues)))
	for name, val := range inputValues {
		logs = append(logs, fmt.Sprintf("  入参 %s = %v", name, val))
	}

	// Step 2: 将入参注入脚本（变量替换 + 在脚本头部注入变量赋值代码）
	finalScript, err := e.injectInputsToScript(cfg.ScriptContent, inputValues)
	if err != nil {
		logs = append(logs, fmt.Sprintf("脚本变量注入失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}

	// Step 3: 在脚本末尾追加输出序列化代码
	finalScript = e.appendOutputSerializer(finalScript, cfg.OutputParams)

	logs = append(logs, fmt.Sprintf("脚本准备完成，长度: %d 字符", len(finalScript)))

	// Step 4: 执行脚本
	scriptOutput, err := e.runScript(ctx, finalScript, cfg)
	if err != nil {
		logs = append(logs, fmt.Sprintf("脚本执行失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}
	logs = append(logs, fmt.Sprintf("脚本执行完成，输出长度: %d 字符", len(scriptOutput)))

	// Step 5: 解析脚本输出，提取出参
	outputValues, err := e.parseScriptOutput(scriptOutput, cfg.OutputParams)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析脚本输出失败: %v，原始输出: %s", err, scriptOutput))
		return CreateFailureResult(err, logs), nil
	}
	logs = append(logs, fmt.Sprintf("出参解析完成，共 %d 个参数", len(outputValues)))

	// Step 6: 将出参写回变量上下文
	e.writeOutputsToContext(outputValues, cfg.OutputParams, execCtx)
	for name, val := range outputValues {
		logs = append(logs, fmt.Sprintf("  出参 %s = %v", name, val))
	}

	logs = append(logs, "Python脚本节点执行成功")
	return CreateSuccessResult(outputValues, logs), nil
}

// Validate 验证Python脚本节点配置
func (e *PythonScriptExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	if nodeInstance.Config == nil {
		return fmt.Errorf("节点配置不能为空")
	}
	if _, ok := nodeInstance.Config["script_content"]; !ok {
		return fmt.Errorf("缺少 script_content 配置")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 私有方法
// ─────────────────────────────────────────────────────────────────────────────

// parseConfig 从 Inputs 中解析脚本配置
func (e *PythonScriptExecutor) parseConfig(inputs map[string]interface{}) (*PythonScriptConfig, error) {
	cfg := &PythonScriptConfig{
		TimeoutSeconds: 30,
		PythonBin:      "python3",
	}

	if v, ok := inputs["script_content"].(string); ok {
		cfg.ScriptContent = v
	}
	if v, ok := inputs["timeout_seconds"].(float64); ok {
		cfg.TimeoutSeconds = int(v)
	}
	if v, ok := inputs["python_bin"].(string); ok && v != "" {
		cfg.PythonBin = v
	}

	// 解析 input_params
	if raw, ok := inputs["input_params"]; ok {
		params, err := e.unmarshalParams(raw)
		if err != nil {
			return nil, fmt.Errorf("解析 input_params 失败: %w", err)
		}
		cfg.InputParams = params
	}

	// 解析 output_params
	if raw, ok := inputs["output_params"]; ok {
		params, err := e.unmarshalParams(raw)
		if err != nil {
			return nil, fmt.Errorf("解析 output_params 失败: %w", err)
		}
		cfg.OutputParams = params
	}

	return cfg, nil
}

// unmarshalParams 将 interface{} 反序列化为 []ScriptParam
func (e *PythonScriptExecutor) unmarshalParams(raw interface{}) ([]ScriptParam, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var params []ScriptParam
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	return params, nil
}

// resolveInputParams 从变量上下文中解析入参值
// 支持 "node_key.field" 格式的跨节点引用
func (e *PythonScriptExecutor) resolveInputParams(params []ScriptParam, execCtx *ExecutionContext) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, p := range params {
		val, found := e.resolveVarKey(p.VarKey, execCtx)
		if !found {
			if p.DefaultValue != nil {
				val = p.DefaultValue
			} else {
				// 没有默认值时用 None（Python null）
				val = nil
			}
		}
		result[p.Name] = val
	}
	return result, nil
}

// resolveVarKey 从上下文中解析变量值，支持 "key" 和 "node_key.field" 两种格式
func (e *PythonScriptExecutor) resolveVarKey(varKey string, execCtx *ExecutionContext) (interface{}, bool) {
	if varKey == "" {
		return nil, false
	}

	// 先从 Variables 中查找
	if val, ok := execCtx.Variables[varKey]; ok {
		return val, true
	}

	// 支持 "node_key.field" 格式：先取 node_key 对应的 map，再取 field
	if idx := strings.Index(varKey, "."); idx > 0 {
		nodeKey := varKey[:idx]
		field := varKey[idx+1:]
		if nodeVal, ok := execCtx.Variables[nodeKey]; ok {
			if nodeMap, ok := nodeVal.(map[string]interface{}); ok {
				if fieldVal, ok := nodeMap[field]; ok {
					return fieldVal, true
				}
			}
		}
	}

	// 再从 Inputs 中查找
	if val, ok := execCtx.Inputs[varKey]; ok {
		return val, true
	}

	return nil, false
}

// injectInputsToScript 将入参注入脚本
// 两种方式：
//  1. 在脚本头部生成 Python 变量赋值语句
//  2. 替换脚本中的 {{param_name}} 占位符（字符串替换）
func (e *PythonScriptExecutor) injectInputsToScript(script string, inputs map[string]interface{}) (string, error) {
	var header strings.Builder
	header.WriteString("# === 自动注入的入参变量 ===\n")
	header.WriteString("import json as _json\n")

	for name, val := range inputs {
		pyLiteral, err := e.toPythonLiteral(val)
		if err != nil {
			return "", fmt.Errorf("参数 %s 转换为 Python 字面量失败: %w", name, err)
		}
		header.WriteString(fmt.Sprintf("%s = %s\n", name, pyLiteral))
	}
	header.WriteString("# === 入参注入结束 ===\n\n")

	// 替换 {{param_name}} 占位符
	result := header.String() + script
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		paramName := re.FindStringSubmatch(match)[1]
		if val, ok := inputs[paramName]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match // 未找到则保留原占位符
	})

	return result, nil
}

// appendOutputSerializer 在脚本末尾追加输出序列化代码
// 脚本执行完后，将出参变量序列化为 JSON 输出到 stdout
func (e *PythonScriptExecutor) appendOutputSerializer(script string, outputParams []ScriptParam) string {
	if len(outputParams) == 0 {
		return script
	}

	var sb strings.Builder
	sb.WriteString(script)
	sb.WriteString("\n\n# === 自动生成的出参序列化代码 ===\n")
	sb.WriteString("_output = {}\n")
	for _, p := range outputParams {
		// 用 try/except 避免变量未定义时报错
		sb.WriteString(fmt.Sprintf("try:\n    _output[%q] = %s\nexcept NameError:\n    _output[%q] = None\n",
			p.Name, p.Name, p.Name))
	}
	sb.WriteString("print('__OUTPUT__:' + _json.dumps(_output, ensure_ascii=False))\n")
	sb.WriteString("# === 出参序列化结束 ===\n")

	return sb.String()
}

// runScript 执行 Python 脚本，返回 stdout 输出
func (e *PythonScriptExecutor) runScript(ctx context.Context, script string, cfg *PythonScriptConfig) (string, error) {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cfg.PythonBin, "-c", script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("脚本执行错误: %w\nSTDERR: %s", err, stderrStr)
		}
		return "", fmt.Errorf("脚本执行错误: %w", err)
	}

	return stdout.String(), nil
}

// parseScriptOutput 从脚本输出中提取出参
// 查找 "__OUTPUT__:{...}" 标记行并解析 JSON
func (e *PythonScriptExecutor) parseScriptOutput(output string, outputParams []ScriptParam) (map[string]interface{}, error) {
	if len(outputParams) == 0 {
		return map[string]interface{}{"raw_output": strings.TrimSpace(output)}, nil
	}

	const marker = "__OUTPUT__:"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, marker) {
			jsonStr := line[len(marker):]
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
				return nil, fmt.Errorf("解析出参 JSON 失败: %w，原始内容: %s", err, jsonStr)
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("脚本输出中未找到 __OUTPUT__ 标记，请确认脚本正常执行完成。原始输出:\n%s", output)
}

// writeOutputsToContext 将出参写回变量上下文
func (e *PythonScriptExecutor) writeOutputsToContext(outputValues map[string]interface{}, outputParams []ScriptParam, execCtx *ExecutionContext) {
	for _, p := range outputParams {
		val, ok := outputValues[p.Name]
		if !ok {
			continue
		}
		// 写回到 Variables，key 使用 var_key（若为空则用 param name）
		writeKey := p.VarKey
		if writeKey == "" {
			writeKey = p.Name
		}
		execCtx.Variables[writeKey] = val
	}
}

// toPythonLiteral 将 Go 值转换为 Python 字面量字符串
func (e *PythonScriptExecutor) toPythonLiteral(val interface{}) (string, error) {
	if val == nil {
		return "None", nil
	}
	switch v := val.(type) {
	case bool:
		if v {
			return "True", nil
		}
		return "False", nil
	case string:
		// 用 json.Marshal 处理转义，再去掉外层双引号换成单引号
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil // JSON 字符串本身就是合法的 Python 字符串字面量
	case float64, int, int64:
		return fmt.Sprintf("%v", v), nil
	default:
		// 对象/数组：序列化为 JSON 字符串，再用 json.loads 解析
		b, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		// 生成：_json.loads('...')
		escaped := strings.ReplaceAll(string(b), `'`, `\'`)
		return fmt.Sprintf("_json.loads('%s')", escaped), nil
	}
}
