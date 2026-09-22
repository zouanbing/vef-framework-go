package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestNewFormData(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		wantLen int
	}{
		{"NilInput", nil, 0},
		{"NonNilInput", map[string]any{"key": "value"}, 1},
		{"EmptyMap", map[string]any{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := approval.NewFormData(tt.input)
			require.NotNil(t, fd, "NewFormData should never return nil")
			assert.Len(t, fd, tt.wantLen, "%s: should have length %d", tt.name, tt.wantLen)
		})
	}
}

func TestFormDataGet(t *testing.T) {
	fd := approval.NewFormData(map[string]any{"name": "alice", "age": 30})

	tests := []struct {
		name     string
		key      string
		expected any
	}{
		{"ExistingKey", "name", "alice"},
		{"NonexistentKey", "missing", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, fd.Get(tt.key), "%s: Get(%q) should return expected value", tt.name, tt.key)
		})
	}
}

func TestFormDataSet(t *testing.T) {
	fd := approval.NewFormData(nil)
	fd.Set("key", "value")
	assert.Equal(t, "value", fd.Get("key"), "Set then Get should round-trip the value")
}

func TestFormDataToMap(t *testing.T) {
	original := map[string]any{"a": 1, "b": "two"}
	fd := approval.NewFormData(original)
	assert.Equal(t, original, fd.ToMap(), "ToMap should return the underlying map unchanged")
}

func TestFormDataClone(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		fd := approval.NewFormData(map[string]any{
			"name":   "alice",
			"nested": map[string]any{"key": "value"},
		})
		cloned, err := fd.Clone()
		require.NoError(t, err, "Clone of a JSON-serializable map should not error")

		cloned.Set("name", "bob")
		assert.Equal(t, "alice", fd.Get("name"), "Modifying clone should not affect original")
		assert.Equal(t, "bob", cloned.Get("name"), "Clone should reflect the mutation")
	})

	t.Run("EmptyFormData", func(t *testing.T) {
		fd := approval.NewFormData(nil)
		cloned, err := fd.Clone()
		require.NoError(t, err, "Clone of empty FormData should not error")
		require.NotNil(t, cloned, "Clone of empty FormData should return non-nil")
		assert.Empty(t, cloned, "Clone of empty FormData should be empty")
	})

	t.Run("MarshalError", func(t *testing.T) {
		fd := approval.FormData{"bad": make(chan int)}
		cloned, err := fd.Clone()
		require.Error(t, err, "Clone with a non-JSON-serializable value should return an error")
		assert.Nil(t, cloned, "Clone should return nil on marshal error, not silently empty data")
	})

	t.Run("NestedMapClone", func(t *testing.T) {
		fd := approval.NewFormData(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "name": "item1"},
				map[string]any{"id": 2, "name": "item2"},
			},
		})
		cloned, err := fd.Clone()
		require.NoError(t, err, "Clone of nested map should not error")

		items := cloned.Get("items").([]any)
		firstItem := items[0].(map[string]any)
		firstItem["name"] = "modified"

		origItems := fd.Get("items").([]any)
		origFirst := origItems[0].(map[string]any)
		assert.Equal(t, "item1", origFirst["name"], "Modifying cloned nested data should not affect the original")
	})
}

func TestFormFieldIDs(t *testing.T) {
	successTests := []struct {
		name     string
		field    string
		formData approval.FormData
		expected []string
	}{
		{"StringValue", "approver", approval.FormData{"approver": "user1"}, []string{"user1"}},
		{"StringValueWithWhitespace", "approver", approval.FormData{"approver": " user1 "}, []string{"user1"}},
		{"FieldNameWithWhitespace", " approver ", approval.FormData{"approver": "user1"}, []string{"user1"}},
		{"StringSlice", "approvers", approval.FormData{"approvers": []string{"u1", "u2"}}, []string{"u1", "u2"}},
		{"StringSliceWithEmptyElement", "approvers", approval.FormData{"approvers": []string{"u1", "", "u2"}}, []string{"u1", "u2"}},
		{"StringSliceWithWhitespaceElement", "approvers", approval.FormData{"approvers": []string{" u1 ", " ", "u2"}}, []string{"u1", "u2"}},
		{"AnySlice", "approvers", approval.FormData{"approvers": []any{"u1", "u2"}}, []string{"u1", "u2"}},
		{"AnySliceWithEmptyElement", "approvers", approval.FormData{"approvers": []any{"u1", "", "u2"}}, []string{"u1", "u2"}},
		{"AnySliceWithWhitespaceElement", "approvers", approval.FormData{"approvers": []any{" u1 ", " ", "u2"}}, []string{"u1", "u2"}},
		{"AnySliceWithNumericElement", "approvers", approval.FormData{"approvers": []any{"u1", 42}}, []string{"u1", "42"}},
		{"KeepsDuplicatesInOrder", "approvers", approval.FormData{"approvers": []any{"u2", "u1", "u2"}}, []string{"u2", "u1", "u2"}},
		{"MissingField", "missing", approval.FormData{}, nil},
		{"EmptyStringValue", "approver", approval.FormData{"approver": ""}, []string{}},
		{"WhitespaceStringValue", "approver", approval.FormData{"approver": "   "}, []string{}},
		{"EmptySlice", "approvers", approval.FormData{"approvers": []any{}}, []string{}},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := approval.FormFieldIDs(tt.formData, new(tt.field))
			require.NoError(t, err, "Should read the field without error")
			assert.Equal(t, tt.expected, ids, "Should return the trimmed, non-blank IDs in order")
		})
	}

	errorTests := []struct {
		name      string
		field     *string
		formData  approval.FormData
		wantError error
	}{
		{"NilFieldName", nil, approval.FormData{"approver": "user1"}, approval.ErrFormFieldNameEmpty},
		{"EmptyFieldName", new(""), approval.FormData{"approver": "user1"}, approval.ErrFormFieldNameEmpty},
		{"WhitespaceFieldName", new("   "), approval.FormData{"approver": "user1"}, approval.ErrFormFieldNameEmpty},
		{"NumberValue", new("count"), approval.FormData{"count": 42}, approval.ErrUnsupportedFieldValueType},
		{"MapValue", new("meta"), approval.FormData{"meta": map[string]string{"k": "v"}}, approval.ErrUnsupportedFieldValueType},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := approval.FormFieldIDs(tt.formData, tt.field)
			require.ErrorIs(t, err, tt.wantError, "Should return %v", tt.wantError)
		})
	}
}
