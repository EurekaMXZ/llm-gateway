package domain

import "time"

type TemplateStatus string

const (
	TemplateStatusEnabled  TemplateStatus = "enabled"
	TemplateStatusDisabled TemplateStatus = "disabled"
)

type VariableType string

const (
	VariableTypeString  VariableType = "string"
	VariableTypeNumber  VariableType = "number"
	VariableTypeBoolean VariableType = "boolean"
)

type VariableDefinition struct {
	Name         string       `json:"name"`
	Type         VariableType `json:"type"`
	Required     bool         `json:"required"`
	DefaultValue *string      `json:"default_value,omitempty"`
}

type SceneTemplate struct {
	ID        string               `json:"id"`
	OwnerID   string               `json:"owner_id"`
	Scene     string               `json:"scene"`
	Content   string               `json:"content"`
	Variables []VariableDefinition `json:"variables"`
	Status    TemplateStatus       `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type RenderIssue struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}
