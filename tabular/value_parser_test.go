package tabular

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultParserParseEmptyString tests DefaultParser Parse empty string scenarios.
func TestDefaultParserParseEmptyString(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		targetType reflect.Type
	}{
		{"String", reflect.TypeFor[string]()},
		{"Int", reflect.TypeFor[int]()},
		{"Float", reflect.TypeFor[float64]()},
		{"Bool", reflect.TypeFor[bool]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse("", tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, reflect.Zero(tt.targetType).Interface(), result, "Should equal expected value")
		})
	}
}

// TestDefaultParserParseBasicTypes tests DefaultParser Parse basic types scenarios.
func TestDefaultParserParseBasicTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"String", "hello", reflect.TypeFor[string](), "hello"},
		{"Int", "42", reflect.TypeFor[int](), 42},
		{"Int8", "127", reflect.TypeFor[int8](), int8(127)},
		{"Int16", "32767", reflect.TypeFor[int16](), int16(32767)},
		{"Int32", "2147483647", reflect.TypeFor[int32](), int32(2147483647)},
		{"Int64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
		{"Uint", "42", reflect.TypeFor[uint](), uint(42)},
		{"Uint8", "255", reflect.TypeFor[uint8](), uint8(255)},
		{"Uint16", "65535", reflect.TypeFor[uint16](), uint16(65535)},
		{"Uint32", "4294967295", reflect.TypeFor[uint32](), uint32(4294967295)},
		{"Uint64", "18446744073709551615", reflect.TypeFor[uint64](), uint64(18446744073709551615)},
		{"Float32", "3.14", reflect.TypeFor[float32](), float32(3.14)},
		{"Float64", "3.14159265359", reflect.TypeFor[float64](), float64(3.14159265359)},
		{"BoolTrue", "true", reflect.TypeFor[bool](), true},
		{"BoolFalse", "false", reflect.TypeFor[bool](), false},
		{"Bool1", "1", reflect.TypeFor[bool](), true},
		{"Bool0", "0", reflect.TypeFor[bool](), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParserParseInvalidBasicTypes tests DefaultParser Parse invalid basic types scenarios.
func TestDefaultParserParseInvalidBasicTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"InvalidInt", "not_a_number", reflect.TypeFor[int]()},
		{"InvalidFloat", "abc", reflect.TypeFor[float64]()},
		{"InvalidBool", "maybe", reflect.TypeFor[bool]()},
		{"InvalidUint", "-1", reflect.TypeFor[uint]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
		})
	}
}

// TestDefaultParserParsePointerTypes tests DefaultParser Parse pointer types scenarios.
func TestDefaultParserParsePointerTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "StringPointer",
			cellValue:  "test",
			targetType: reflect.TypeFor[*string](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*string)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, "test", *ptr, "Should equal expected value")
			},
		},
		{
			name:       "IntPointer",
			cellValue:  "42",
			targetType: reflect.TypeFor[*int](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*int)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, 42, *ptr, "Should equal expected value")
			},
		},
		{
			name:       "BoolPointer",
			cellValue:  "true",
			targetType: reflect.TypeFor[*bool](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*bool)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, true, *ptr, "Should equal expected value")
			},
		},
		{
			name:       "Float64Pointer",
			cellValue:  "3.14",
			targetType: reflect.TypeFor[*float64](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*float64)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, 3.14, *ptr, "Should equal expected value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			require.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParserParseTimeTypes tests DefaultParser Parse time types scenarios.
func TestDefaultParserParseTimeTypes(t *testing.T) {
	parser := NewDefaultParser("")

	testTimeStr := "2024-01-15 14:30:45"

	result, err := parser.Parse(testTimeStr, typeTime)
	require.NoError(t, err, "Should not return error")

	parsed := result.(time.Time)
	expected := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)
	assert.Equal(t, expected, parsed, "Should equal expected value")
}

// TestDefaultParserParseTimeTypesWithFormat tests DefaultParser Parse time types with format scenarios.
func TestDefaultParserParseTimeTypesWithFormat(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		cellValue string
		validate  func(*testing.T, any)
	}{
		{
			name:      "TimeTimeCustomFormat",
			format:    "2006-01-02",
			cellValue: "2024-01-15",
			validate: func(t *testing.T, result any) {
				parsed := result.(time.Time)
				expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
				assert.Equal(t, expected, parsed, "Should equal expected value")
			},
		},
		{
			name:      "TimeTimeRFC3339",
			format:    time.RFC3339,
			cellValue: "2024-01-15T14:30:45+08:00",
			validate: func(t *testing.T, result any) {
				parsed := result.(time.Time)
				assert.Equal(t, 2024, parsed.Year(), "Should equal expected value")
				assert.Equal(t, time.January, parsed.Month(), "Should equal expected value")
				assert.Equal(t, 15, parsed.Day(), "Should equal expected value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewDefaultParser(tt.format)
			result, err := parser.Parse(tt.cellValue, typeTime)
			require.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParserParseInvalidTimeTypes tests DefaultParser Parse invalid time types scenarios.
func TestDefaultParserParseInvalidTimeTypes(t *testing.T) {
	parser := NewDefaultParser("")

	_, err := parser.Parse("not_a_time", typeTime)
	assert.Error(t, err, "Should return error for invalid time")
}

// TestDefaultParserParseDecimalTypes tests DefaultParser Parse decimal types scenarios.
func TestDefaultParserParseDecimalTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name      string
		cellValue string
		expected  string
	}{
		{"DecimalInteger", "100", "100"},
		{"DecimalFloat", "3.14", "3.14"},
		{"DecimalScientific", "1.23e+2", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, typeDecimal)
			assert.NoError(t, err, "Should not return error")

			d := result.(decimal.Decimal)
			assert.Equal(t, tt.expected, d.String(), "Should equal expected value")
		})
	}
}

// TestDefaultParserParseInvalidDecimalTypes tests DefaultParser Parse invalid decimal types scenarios.
func TestDefaultParserParseInvalidDecimalTypes(t *testing.T) {
	parser := NewDefaultParser("")

	_, err := parser.Parse("not_a_number", typeDecimal)
	assert.Error(t, err, "Should return error for invalid decimal")
}

// TestDefaultParserParseUnsupportedTypes tests DefaultParser Parse unsupported types scenarios.
func TestDefaultParserParseUnsupportedTypes(t *testing.T) {
	parser := NewDefaultParser("")

	type CustomStruct struct {
		Field string
	}

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"UnsupportedStruct", "data", reflect.TypeFor[CustomStruct]()},
		{"UnsupportedSlice", "[1,2,3]", reflect.TypeFor[[]int]()},
		{"UnsupportedMap", "{}", reflect.TypeFor[map[string]string]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
			assert.ErrorIs(t, err, ErrUnsupportedType, "Error should be ErrUnsupportedType")
		})
	}
}

// TestDefaultParserParseEdgeCases tests DefaultParser Parse edge cases scenarios.
func TestDefaultParserParseEdgeCases(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"ZeroInt", "0", reflect.TypeFor[int](), 0},
		{"ZeroFloat", "0.0", reflect.TypeFor[float64](), 0.0},
		{"NegativeInt", "-42", reflect.TypeFor[int](), -42},
		{"NegativeFloat", "-3.14", reflect.TypeFor[float64](), -3.14},
		{"LeadingZeros", "007", reflect.TypeFor[int](), 7},
		{"WhitespaceString", "  test  ", reflect.TypeFor[string](), "  test  "},
		{"MaxInt64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
		{"MinInt64", "-9223372036854775808", reflect.TypeFor[int64](), int64(-9223372036854775808)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParserParseUnicodeStrings tests DefaultParser Parse unicode strings scenarios.
func TestDefaultParserParseUnicodeStrings(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name      string
		cellValue string
		expected  string
	}{
		{"ChineseCharacters", "你好世界", "你好世界"},
		{"EmojiCharacters", "👍🎉", "👍🎉"},
		{"MixedUnicode", "Hello世界🌍", "Hello世界🌍"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, reflect.TypeFor[string]())
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParserParseBooleanVariants tests DefaultParser Parse boolean variants scenarios.
func TestDefaultParserParseBooleanVariants(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name      string
		cellValue string
		expected  bool
	}{
		{"True", "true", true},
		{"False", "false", false},
		{"One", "1", true},
		{"Zero", "0", false},
		{"TrueUpperCase", "True", true},
		{"TrueCapitalized", "TRUE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, reflect.TypeFor[bool]())
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParserParseFloatPrecision tests DefaultParser Parse float precision scenarios.
func TestDefaultParserParseFloatPrecision(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "Float32Precision",
			cellValue:  "3.14159265359",
			targetType: reflect.TypeFor[float32](),
			validate: func(t *testing.T, result any) {
				f := result.(float32)
				assert.InDelta(t, 3.14159, f, 0.00001, "InDelta assertion should pass")
			},
		},
		{
			name:       "Float64Precision",
			cellValue:  "3.14159265359",
			targetType: reflect.TypeFor[float64](),
			validate: func(t *testing.T, result any) {
				f := result.(float64)
				assert.InDelta(t, 3.14159265359, f, 0.000000001, "InDelta assertion should pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParserNewDefaultParser tests DefaultParser NewDefaultParser scenarios.
func TestDefaultParserNewDefaultParser(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"EmptyFormat", ""},
		{"DateFormat", "2006-01-02"},
		{"TimeFormat", time.RFC3339},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewDefaultParser(tt.format)
			assert.NotNil(t, parser, "Should not be nil")
		})
	}
}

// TestDefaultParserParseEmptyStringForPointer tests DefaultParser Parse empty string for pointer scenarios.
func TestDefaultParserParseEmptyStringForPointer(t *testing.T) {
	parser := NewDefaultParser("")

	result, err := parser.Parse("", reflect.TypeFor[*string]())
	assert.NoError(t, err, "Should not return error")
	assert.Nil(t, result, "Should be nil")
}
