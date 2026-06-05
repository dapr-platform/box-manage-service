package service

import (
	"box-manage-service/models"
	"encoding/json"
	"log"
	"strings"
)

func normalizeBoxVariableType(rawType string) (string, bool) {
	typeName := strings.ToLower(strings.TrimSpace(rawType))
	if typeName == "" {
		return "string", rawType != "string"
	}

	switch typeName {
	case "string", "number", "boolean", "object", "array", "reference":
		return typeName, typeName != rawType
	case "json", "map", "dict", "any":
		return "object", true
	case "text", "textarea", "markdown", "url", "image", "file", "password", "select", "enum":
		return "string", true
	case "int", "integer", "float", "double":
		return "number", true
	case "bool":
		return "boolean", true
	case "list":
		return "array", true
	case "ref":
		return "reference", true
	default:
		return "string", true
	}
}

func buildWorkflowPayloadForBox(workflow *models.Workflow) json.RawMessage {
	if workflow == nil {
		return nil
	}
	payload := normalizeWorkflowStructureJSONForBox(workflow.StructureJSON)
	if len(payload) == 0 && strings.TrimSpace(workflow.StructureJSONView) == "" {
		return payload
	}

	var structure map[string]interface{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &structure); err != nil {
			log.Printf("[WorkflowPayload] parse normalized workflow payload failed, sending original payload: %v", err)
			return payload
		}
	} else {
		structure = map[string]interface{}{}
	}

	if strings.TrimSpace(workflow.StructureJSONView) != "" {
		structure["structure_json_view"] = workflow.StructureJSONView
	}

	encoded, err := json.Marshal(structure)
	if err != nil {
		log.Printf("[WorkflowPayload] marshal workflow payload failed, sending original payload: %v", err)
		return payload
	}
	return json.RawMessage(encoded)
}

func normalizeWorkflowStructureJSONForBox(raw string) json.RawMessage {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var structure map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &structure); err != nil {
		log.Printf("[WorkflowPayload] parse StructureJSON failed, sending original payload: %v", err)
		return json.RawMessage(raw)
	}

	normalizeWorkflowVariablesForBox(structure["variables"])

	normalized, err := json.Marshal(structure)
	if err != nil {
		log.Printf("[WorkflowPayload] marshal normalized StructureJSON failed, sending original payload: %v", err)
		return json.RawMessage(raw)
	}
	return json.RawMessage(normalized)
}

func normalizeWorkflowVariablesForBox(variables interface{}) {
	items, ok := variables.([]interface{})
	if !ok {
		return
	}

	for _, item := range items {
		variable, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rawType, _ := variable["type"].(string)
		normalizedType, changed := normalizeBoxVariableType(rawType)
		if changed {
			keyName, _ := variable["key_name"].(string)
			log.Printf("[WorkflowPayload] normalize variable type for box: key=%s %q -> %q",
				keyName, rawType, normalizedType)
			variable["type"] = normalizedType
		}
	}
}

func normalizeVariableDefinitionForBox(variable *models.VariableDefinition) *models.VariableDefinition {
	if variable == nil {
		return nil
	}

	copyValue := *variable
	if normalizedType, changed := normalizeBoxVariableType(copyValue.Type); changed {
		log.Printf("[WorkflowPayload] normalize variable definition for box: key=%s %q -> %q",
			copyValue.KeyName, copyValue.Type, normalizedType)
		copyValue.Type = normalizedType
	}
	return &copyValue
}
