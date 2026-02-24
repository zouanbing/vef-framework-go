package approval

// Condition represents a branch condition evaluated by condition nodes.
type Condition struct {
	Type       ConditionKind `json:"type"`
	Subject    string        `json:"subject"`
	Operator   string        `json:"operator"`
	Value      any           `json:"value"`
	Expression string        `json:"expression"`
}

// ConditionGroup represents a group of conditions evaluated with AND logic.
// Multiple groups in a branch are evaluated with OR logic.
type ConditionGroup struct {
	Conditions []Condition `json:"conditions"`
}

// ConditionBranch represents a branch in a condition node.
// Each branch has its own condition groups and can be linked to an edge via its ID.
type ConditionBranch struct {
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	ConditionGroups []ConditionGroup `json:"conditionGroups,omitempty"`
	IsDefault       bool             `json:"isDefault,omitempty"`
	Priority        int              `json:"priority"`
}

// FlowDefinition represents the structure of a flow definition JSON (React Flow compatible).
type FlowDefinition struct {
	Nodes []NodeDefinition `json:"nodes"`
	Edges []EdgeDefinition `json:"edges"`
}

// Position represents the visual position of a node on the canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeDefinition represents a node in the flow definition.
type NodeDefinition struct {
	ID       string         `json:"id"`
	Type     NodeKind       `json:"type"`
	Position Position       `json:"position"`
	Data     map[string]any `json:"data,omitempty"`
}

// AssigneeDefinition represents an assignee configuration in the flow definition.
type AssigneeDefinition struct {
	Kind      string   `json:"kind"`
	IDs       []string `json:"ids,omitempty"`
	FormField string   `json:"formField,omitempty"`
	SortOrder int      `json:"sortOrder"`
}

// EdgeDefinition represents a connection between nodes.
type EdgeDefinition struct {
	ID           string         `json:"id,omitempty"`
	Source       string         `json:"source"`
	Target       string         `json:"target"`
	SourceHandle string         `json:"sourceHandle,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}

// FormDefinition represents the form schema definition for a flow version.
type FormDefinition struct {
	Fields []FormFieldDefinition `json:"fields"`
}

// FormFieldDefinition represents a single form field.
type FormFieldDefinition struct {
	// Key is the unique identifier for this field (used in form data keys).
	Key string `json:"key"`
	// Kind is the field type (e.g., "input", "textarea", "select", "number", "date", "upload").
	Kind string `json:"kind"`
	// Label is the display label.
	Label string `json:"label"`
	// Placeholder is the input placeholder text.
	Placeholder string `json:"placeholder,omitempty"`
	// DefaultValue is the default value for this field.
	DefaultValue any `json:"defaultValue,omitempty"`
	// IsRequired indicates whether this field is required.
	IsRequired bool `json:"isRequired,omitempty"`
	// Options is the list of selectable options (for select, radio, checkbox, etc.).
	Options []FieldOption `json:"options,omitempty"`
	// Validation contains validation rules.
	Validation *ValidationRule `json:"validation,omitempty"`
	// Props contains additional component-specific properties.
	Props map[string]any `json:"props,omitempty"`
	// SortOrder controls the display order.
	SortOrder int `json:"sortOrder"`
}

// FieldOption represents a selectable option for select/radio/checkbox fields.
type FieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// ValidationRule contains validation constraints for a form field.
type ValidationRule struct {
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Message   string   `json:"message,omitempty"`
}
