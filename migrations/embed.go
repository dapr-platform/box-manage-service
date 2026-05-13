package migrations

import _ "embed"

//go:embed create_workflow_tables.sql
var CreateWorkflowTablesSQL string

//go:embed node_template_init_data.sql
var NodeTemplateInitDataSQL string
