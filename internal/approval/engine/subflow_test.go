package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

func TestPrepareSubFormData(t *testing.T) {
	p := &SubFlowProcessor{}

	tests := []struct {
		name       string
		parentData approval.FormData
		config     map[string]any
		expected   map[string]any
	}{
		{
			name:       "ValidMapping",
			parentData: approval.NewFormData(map[string]any{"amount": 100, "title": "test"}),
			config: map[string]any{
				"dataMapping": []any{
					map[string]any{"sourceField": "amount", "targetField": "sub_amount"},
				},
			},
			expected: map[string]any{"sub_amount": 100},
		},
		{
			name:       "NoDataMapping",
			parentData: approval.NewFormData(map[string]any{"amount": 100}),
			config:     map[string]any{"flowId": "f1"},
			expected:   map[string]any{},
		},
		{
			name:       "InvalidMappingFormat",
			parentData: approval.NewFormData(map[string]any{"amount": 100}),
			config:     map[string]any{"dataMapping": "not-an-array"},
			expected:   map[string]any{},
		},
		{
			name:       "MissingSourceField",
			parentData: approval.NewFormData(map[string]any{"title": "test"}),
			config: map[string]any{
				"dataMapping": []any{
					map[string]any{"sourceField": "amount", "targetField": "sub_amount"},
				},
			},
			expected: map[string]any{},
		},
		{
			name:       "EmptyFieldNames",
			parentData: approval.NewFormData(map[string]any{"amount": 100}),
			config: map[string]any{
				"dataMapping": []any{
					map[string]any{"sourceField": "", "targetField": "sub_amount"},
					map[string]any{"sourceField": "amount", "targetField": ""},
				},
			},
			expected: map[string]any{},
		},
		{
			name:       "MultipleMapping",
			parentData: approval.NewFormData(map[string]any{"amount": 100, "title": "test", "extra": "ignored"}),
			config: map[string]any{
				"dataMapping": []any{
					map[string]any{"sourceField": "amount", "targetField": "sub_amount"},
					map[string]any{"sourceField": "title", "targetField": "sub_title"},
				},
			},
			expected: map[string]any{"sub_amount": 100, "sub_title": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.prepareSubFormData(tt.parentData, tt.config)
			assert.Equal(t, approval.FormData(tt.expected), got)
		})
	}
}

func TestNewSubFlowProcessor(t *testing.T) {
	p := NewSubFlowProcessor()
	require.NotNil(t, p, "Should return a non-nil processor")
	assert.Equal(t, approval.NodeSubFlow, p.NodeKind(), "Should return NodeSubFlow kind")
}

func TestSubFlowProcessorSetFlowEngine(t *testing.T) {
	t.Run("SetsEngine", func(t *testing.T) {
		p := NewSubFlowProcessor()
		require.Nil(t, p.engine, "Should have nil engine initially")

		engine := NewFlowEngine(nil, nil, nil)
		p.SetFlowEngine(engine)
		assert.Same(t, engine, p.engine, "Should set the engine reference")
	})
}
