package approval

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cast"
)

// FormData wraps a map to provide helper methods for form data operations.
type FormData map[string]any

func NewFormData(data map[string]any) FormData {
	if data == nil {
		return make(FormData)
	}

	return FormData(data)
}

func (f FormData) Get(key string) any      { return f[key] }
func (f FormData) Set(key string, val any) { f[key] = val }
func (f FormData) ToMap() map[string]any   { return f }

// Clone creates a deep copy via JSON serialization.
// It returns an error if the map contains values that cannot be marshaled to JSON
// (e.g. channels, functions). The caller should treat an error as uncloneable data
// and act accordingly rather than silently losing the original content.
func (f FormData) Clone() (FormData, error) {
	if len(f) == 0 {
		return make(FormData), nil
	}

	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("approval: FormData.Clone marshal: %w", err)
	}

	var cloned map[string]any
	if err := json.Unmarshal(jsonBytes, &cloned); err != nil {
		return nil, fmt.Errorf("approval: FormData.Clone unmarshal: %w", err)
	}

	return FormData(cloned), nil
}

// FormFieldIDs reads the IDs a form field carries, for resolvers of a
// SelectionFormField kind — field is the rule's FormField. The value may be a
// single string or a list of them (a multi-select submits []any); entries are
// trimmed and blanks dropped, preserving order.
//
// The IDs are whatever the field holds, not necessarily user IDs: a host kind
// whose form stores, say, staff IDs maps them to the user accounts tasks are
// assigned to. A missing or blank value resolves to no IDs rather than an
// error, so the node's EmptyAssigneeAction (or, for CC, an empty recipient
// list) decides; a nil or blank field name returns ErrFormFieldNameEmpty and
// any other value type ErrUnsupportedFieldValueType.
func FormFieldIDs(formData FormData, field *string) ([]string, error) {
	if field == nil || strings.TrimSpace(*field) == "" {
		return nil, ErrFormFieldNameEmpty
	}

	var raw []string

	switch v := formData.Get(strings.TrimSpace(*field)).(type) {
	case nil:
		return nil, nil
	case string:
		raw = []string{v}
	case []string:
		raw = v
	case []any:
		raw = make([]string, 0, len(v))
		for _, item := range v {
			raw = append(raw, cast.ToString(item))
		}

	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupportedFieldValueType, v)
	}

	ids := make([]string, 0, len(raw))
	for _, value := range raw {
		if id := strings.TrimSpace(value); id != "" {
			ids = append(ids, id)
		}
	}

	return ids, nil
}
