package mapx

import (
	"mime/multipart"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/sortx"
)

type TestStruct struct {
	Name       string        `json:"name"`
	Age        int           `json:"age"`
	Email      string        `json:"email,omitempty"`
	Active     bool          `json:"active"`
	Score      float64       `json:"score"`
	Created    time.Time     `json:"created"`
	Duration   time.Duration `json:"duration"`
	Website    *url.URL      `json:"website"`
	IP         net.IP        `json:"ip"`
	Unexported string
}

type EmbeddedStruct struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

type StructWithEmbedding struct {
	Name     string         `json:"name"`
	Embedded EmbeddedStruct `json:"embedded,inline"`
}

// TestNewDecoder tests decoder creation with various options.
func TestNewDecoder(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		var result TestStruct

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")
		assert.NotNil(t, decoder, "Decoder should not be nil")
	})

	t.Run("CustomOptions", func(t *testing.T) {
		var result TestStruct

		decoder, err := NewDecoder(&result, WithTagName("custom"), WithErrorUnused())
		require.NoError(t, err, "Decoder creation with custom options should succeed")
		assert.NotNil(t, decoder, "Decoder should not be nil")
	})
}

// TestToMap tests struct to map conversion.
func TestToMap(t *testing.T) {
	t.Run("ValidStruct", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		testURL, _ := url.Parse("https://example.com")
		input := TestStruct{
			Name:     "John Doe",
			Age:      30,
			Email:    "john@example.com",
			Active:   true,
			Score:    95.5,
			Created:  testTime,
			Duration: time.Hour,
			Website:  testURL,
			IP:       net.ParseIP("192.168.1.1"),
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Struct to map conversion should succeed")

		assert.Equal(t, "John Doe", result["name"], "Name should match")
		assert.Equal(t, 30, result["age"], "Age should match")
		assert.Equal(t, "john@example.com", result["email"], "Email should match")
		assert.Equal(t, true, result["active"], "Active should match")
		assert.Equal(t, 95.5, result["score"], "Score should match")
		assert.Contains(t, result, "created", "Created field should exist")
		assert.Contains(t, result, "duration", "Duration field should exist")
		assert.Contains(t, result, "website", "Website field should exist")
		assert.Contains(t, result, "ip", "IP field should exist")
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		input := &TestStruct{
			Name: "Jane Doe",
			Age:  25,
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Pointer to struct conversion should succeed")

		assert.Equal(t, "Jane Doe", result["name"], "Name should match")
		assert.Equal(t, 25, result["age"], "Age should match")
	})

	t.Run("StructWithEmbedding", func(t *testing.T) {
		input := StructWithEmbedding{
			Name: "Test",
			Embedded: EmbeddedStruct{
				ID:   123,
				Type: "example",
			},
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Struct with embedding conversion should succeed")

		assert.Equal(t, "Test", result["name"], "Name should match")
		assert.Equal(t, 123, result["id"], "Embedded id should be inlined")
		assert.Equal(t, "example", result["type"], "Embedded type should be inlined")
	})

	t.Run("NonStructValue", func(t *testing.T) {
		input := "not a struct"

		result, err := ToMap(input)
		assert.Error(t, err, "Non-struct value should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("SliceInput", func(t *testing.T) {
		input := []int{1, 2, 3}

		result, err := ToMap(input)
		assert.Error(t, err, "Slice input should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("CustomTagName", func(t *testing.T) {
		type CustomTagStruct struct {
			Name string `custom:"full_name"`
			Age  int    `custom:"years"`
		}

		input := CustomTagStruct{Name: "John", Age: 30}
		result, err := ToMap(input, WithTagName("custom"))
		require.NoError(t, err, "Conversion with custom tag should succeed")

		assert.Equal(t, "John", result["full_name"], "Custom tag name should be used")
		assert.Equal(t, 30, result["years"], "Custom tag name should be used")
	})
}

// TestFromMap tests map to struct conversion.
func TestFromMap(t *testing.T) {
	t.Run("ValidMapToStruct", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		input := map[string]any{
			"name":     "John Doe",
			"age":      30,
			"email":    "john@example.com",
			"active":   true,
			"score":    95.5,
			"created":  testTime,
			"duration": "1h",
			"website":  "https://example.com",
			"ip":       "192.168.1.1",
		}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Map to struct conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "John Doe", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
		assert.Equal(t, "john@example.com", result.Email, "Email should match")
		assert.Equal(t, true, result.Active, "Active should match")
		assert.Equal(t, 95.5, result.Score, "Score should match")
		assert.Equal(t, testTime, result.Created, "Created time should match")
		assert.Equal(t, time.Hour, result.Duration, "Duration should match")
		assert.Equal(t, "https://example.com", result.Website.String(), "Website URL should match")
		assert.Equal(t, "192.168.1.1", result.IP.String(), "IP address should match")
	})

	t.Run("PartialMap", func(t *testing.T) {
		input := map[string]any{
			"name": "Jane Doe",
			"age":  25,
		}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Partial map conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "Jane Doe", result.Name, "Name should match")
		assert.Equal(t, 25, result.Age, "Age should match")
		assert.Equal(t, "", result.Email, "Email should be empty")
		assert.Equal(t, false, result.Active, "Active should be false (zero value)")
	})

	t.Run("EmptyMap", func(t *testing.T) {
		input := map[string]any{}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Empty map conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "", result.Name, "Name should be empty (zero value)")
		assert.Equal(t, 0, result.Age, "Age should be 0 (zero value)")
	})

	t.Run("MapWithEmbedding", func(t *testing.T) {
		input := map[string]any{
			"name": "Test",
			"id":   123,
			"type": "example",
		}

		result, err := FromMap[StructWithEmbedding](input)
		require.NoError(t, err, "Map with embedded fields conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "Test", result.Name, "Name should match")
		assert.Equal(t, 123, result.Embedded.ID, "Embedded id should match")
		assert.Equal(t, "example", result.Embedded.Type, "Embedded type should match")
	})

	t.Run("NonStructTypeParameter", func(t *testing.T) {
		input := map[string]any{"value": "test"}

		result, err := FromMap[string](input)
		assert.Error(t, err, "Non-struct type parameter should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("CustomTagName", func(t *testing.T) {
		type CustomTagStruct struct {
			Name string `custom:"full_name"`
			Age  int    `custom:"years"`
		}

		input := map[string]any{
			"full_name": "John",
			"years":     30,
		}

		result, err := FromMap[CustomTagStruct](input, WithTagName("custom"))
		require.NoError(t, err, "Conversion with custom tag should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
	})
}

// TestDecoderOptions tests various decoder configuration options.
func TestDecoderOptions(t *testing.T) {
	t.Run("WithTagName", func(t *testing.T) {
		type TestStruct struct {
			Name string `yaml:"fullName"`
		}

		input := map[string]any{"fullName": "John"}
		result, err := FromMap[TestStruct](input, WithTagName("yaml"))
		require.NoError(t, err, "Decoding with custom tag name should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
	})

	t.Run("WithIgnoreUntaggedFields", func(t *testing.T) {
		type TestStruct struct {
			Name          string `json:"name"`
			UntaggedField string
		}

		input := map[string]any{
			"name":          "John",
			"UntaggedField": "should be ignored",
		}

		result, err := FromMap[TestStruct](input, WithIgnoreUntaggedFields(true))
		require.NoError(t, err, "Decoding with ignored untagged fields should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, "", result.UntaggedField, "Untagged field should be empty")
	})

	t.Run("WithWeaklyTypedInput", func(t *testing.T) {
		type TestStruct struct {
			Age int `json:"age"`
		}

		input := map[string]any{"age": "30"}
		result, err := FromMap[TestStruct](input, WithWeaklyTypedInput())
		require.NoError(t, err, "Weakly typed input conversion should succeed")
		assert.Equal(t, 30, result.Age, "Age should be converted from string to int")
	})

	t.Run("WithZeroFields", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		input := map[string]any{"name": "New Name"}
		result, err := FromMap[TestStruct](input, WithZeroFields())
		require.NoError(t, err, "Decoding with zero fields should succeed")

		assert.Equal(t, "New Name", result.Name, "Name should match")
		assert.Equal(t, 0, result.Age, "Age should be zero (default)")
	})

	t.Run("WithMetadata", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var metadata Metadata

		input := map[string]any{
			"name":  "John",
			"age":   30,
			"extra": "unused field",
		}

		result, err := FromMap[TestStruct](input, WithMetadata(&metadata))
		require.NoError(t, err, "Decoding with metadata should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
		assert.Contains(t, metadata.Unused, "extra", "Metadata should contain unused fields")
	})
}

// TestComplexTypeConversions tests conversion of complex types.
func TestComplexTypeConversions(t *testing.T) {
	t.Run("TimeConversion", func(t *testing.T) {
		type TimeStruct struct {
			Created time.Time `json:"created"`
		}

		input := map[string]any{
			"created": "2023-01-01T12:00:00Z",
		}

		result, err := FromMap[TimeStruct](input)
		require.NoError(t, err, "Time conversion should succeed")

		expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
		assert.Equal(t, expectedTime, result.Created, "Time should match")
	})

	t.Run("DurationConversion", func(t *testing.T) {
		type DurationStruct struct {
			Timeout time.Duration `json:"timeout"`
		}

		input := map[string]any{
			"timeout": "5m30s",
		}

		result, err := FromMap[DurationStruct](input)
		require.NoError(t, err, "Duration conversion should succeed")

		expected, _ := time.ParseDuration("5m30s")
		assert.Equal(t, expected, result.Timeout, "Duration should match")
	})

	t.Run("URLConversion", func(t *testing.T) {
		type URLStruct struct {
			Website *url.URL `json:"website"`
		}

		input := map[string]any{
			"website": "https://example.com/path?param=value",
		}

		result, err := FromMap[URLStruct](input)
		require.NoError(t, err, "URL conversion should succeed")
		assert.Equal(t, "https://example.com/path?param=value", result.Website.String(), "URL should match")
	})

	t.Run("IPConversion", func(t *testing.T) {
		type IPStruct struct {
			Address net.IP `json:"address"`
		}

		input := map[string]any{
			"address": "192.168.1.100",
		}

		result, err := FromMap[IPStruct](input)
		require.NoError(t, err, "IP conversion should succeed")
		assert.Equal(t, "192.168.1.100", result.Address.String(), "IP address should match")
	})
}

// TestRoundTripConversion tests struct-to-map and back-to-struct conversion.
func TestRoundTripConversion(t *testing.T) {
	t.Run("StructToMapAndBack", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		original := TestStruct{
			Name:     "John Doe",
			Age:      30,
			Email:    "john@example.com",
			Active:   true,
			Score:    95.5,
			Created:  testTime,
			Duration: time.Hour * 2,
		}

		mapResult, err := ToMap(original)
		require.NoError(t, err, "Struct to map should succeed")

		structResult, err := FromMap[TestStruct](mapResult)
		require.NoError(t, err, "Map to struct should succeed")

		assert.Equal(t, original.Name, structResult.Name, "Name should match")
		assert.Equal(t, original.Age, structResult.Age, "Age should match")
		assert.Equal(t, original.Email, structResult.Email, "Email should match")
		assert.Equal(t, original.Active, structResult.Active, "Active should match")
		assert.Equal(t, original.Score, structResult.Score, "Score should match")
	})
}

// TestFileHeaderConversion tests multipart file header conversions.
func TestFileHeaderConversion(t *testing.T) {
	t.Run("SliceWithSingleFileToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		fileHeader := &multipart.FileHeader{
			Filename: "avatar.jpg",
			Size:     1024,
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader{fileHeader},
		}

		result, err := FromMap[StructWithSingleFile](input)
		require.NoError(t, err, "Slice to single file conversion should succeed")
		require.NotNil(t, result.Avatar, "Avatar should not be nil")
		assert.Equal(t, "avatar.jpg", result.Avatar.Filename, "Filename should match")
		assert.Equal(t, int64(1024), result.Avatar.Size, "Size should match")
	})

	t.Run("SliceWithMultipleFilesRemainsSlice", func(t *testing.T) {
		type StructWithMultipleFiles struct {
			Attachments []*multipart.FileHeader `json:"attachments"`
		}

		fileHeaders := []*multipart.FileHeader{
			{Filename: "file1.pdf", Size: 2048},
			{Filename: "file2.pdf", Size: 3072},
		}

		input := map[string]any{
			"attachments": fileHeaders,
		}

		result, err := FromMap[StructWithMultipleFiles](input)
		require.NoError(t, err, "Multiple files conversion should succeed")
		require.Len(t, result.Attachments, 2, "Should have 2 attachments")
		assert.Equal(t, "file1.pdf", result.Attachments[0].Filename, "First filename should match")
		assert.Equal(t, "file2.pdf", result.Attachments[1].Filename, "Second filename should match")
	})

	t.Run("EmptySliceToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader{},
		}

		_, err := FromMap[StructWithSingleFile](input)
		assert.Error(t, err, "Empty slice to pointer should error")
		assert.Contains(t, err.Error(), "expected a map or struct", "Error should mention type mismatch")
	})

	t.Run("SliceToSliceRemainsUnchanged", func(t *testing.T) {
		type StructWithFileSlice struct {
			Files []*multipart.FileHeader `json:"files"`
		}

		fileHeader := &multipart.FileHeader{
			Filename: "document.pdf",
			Size:     4096,
		}

		input := map[string]any{
			"files": []*multipart.FileHeader{fileHeader},
		}

		result, err := FromMap[StructWithFileSlice](input)
		require.NoError(t, err, "Slice to slice conversion should succeed")
		require.Len(t, result.Files, 1, "Should have 1 file")
		assert.Equal(t, "document.pdf", result.Files[0].Filename, "Filename should match")
		assert.Equal(t, int64(4096), result.Files[0].Size, "Size should match")
	})

	t.Run("NilSliceToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader(nil),
		}

		result, err := FromMap[StructWithSingleFile](input)
		require.NoError(t, err, "Nil slice conversion should succeed")
		assert.Nil(t, result.Avatar, "Avatar should be nil")
	})
}

// TestDecodeOrderDirection tests sort.OrderDirection type conversions.
func TestDecodeOrderDirection(t *testing.T) {
	t.Run("DecodeStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"column":    "name",
			"direction": "asc",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, "name", result.Column, "Column should match")
		assert.Equal(t, sortx.OrderAsc, result.Direction, "Direction should be asc")
	})

	t.Run("DecodeUppercaseStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": "DESC",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, sortx.OrderDesc, result.Direction, "Direction should be desc")
	})

	t.Run("DecodeMixedCaseStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		tests := []struct {
			name     string
			input    string
			expected sortx.OrderDirection
		}{
			{"Asc", "Asc", sortx.OrderAsc},
			{"AsC", "AsC", sortx.OrderAsc},
			{"Desc", "Desc", sortx.OrderDesc},
			{"DeSc", "DeSc", sortx.OrderDesc},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				input := map[string]any{
					"direction": tt.input,
				}

				var result SortSpec

				decoder, err := NewDecoder(&result)
				require.NoError(t, err, "Decoder creation should succeed")

				err = decoder.Decode(input)
				require.NoError(t, err, "Decode should succeed")

				assert.Equal(t, tt.expected, result.Direction, "Direction should match expected value")
			})
		}
	})

	t.Run("DecodeOrderDirectionWithSpaces", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": " asc ",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, sortx.OrderAsc, result.Direction, "Direction should be asc with spaces trimmed")
	})

	t.Run("DecodeInvalidOrderDirectionValue", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": "invalid",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		assert.Error(t, err, "Invalid value should error")
		assert.Contains(t, err.Error(), "invalid OrderDirection value", "Error should mention invalid value")
	})

	t.Run("DecodeMultipleOrderDirectionInSlice", func(t *testing.T) {
		type SortRequest struct {
			Sort []sortx.OrderSpec `json:"sort"`
		}

		input := map[string]any{
			"sort": []map[string]any{
				{"column": "name", "direction": "asc"},
				{"column": "age", "direction": "desc"},
			},
		}

		var result SortRequest

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		require.Len(t, result.Sort, 2, "Should have 2 sort specs")
		assert.Equal(t, "name", result.Sort[0].Column, "First column should be name")
		assert.Equal(t, sortx.OrderAsc, result.Sort[0].Direction, "First direction should be asc")
		assert.Equal(t, "age", result.Sort[1].Column, "Second column should be age")
		assert.Equal(t, sortx.OrderDesc, result.Sort[1].Direction, "Second direction should be desc")
	})

	t.Run("DecodeNestedOrderDirectionInComplexStruct", func(t *testing.T) {
		type FilterSpec struct {
			Field    string `json:"field"`
			Operator string `json:"operator"`
			Value    any    `json:"value"`
		}

		type QueryRequest struct {
			Filters []FilterSpec      `json:"filters"`
			Sort    []sortx.OrderSpec `json:"sort"`
			Page    int               `json:"page"`
			Size    int               `json:"size"`
		}

		input := map[string]any{
			"filters": []map[string]any{
				{"field": "status", "operator": "eq", "value": "active"},
			},
			"sort": []map[string]any{
				{"column": "created_at", "direction": "desc"},
				{"column": "name", "direction": "asc"},
			},
			"page": 1,
			"size": 20,
		}

		var result QueryRequest

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, 1, result.Page, "Page should match")
		assert.Equal(t, 20, result.Size, "Size should match")
		require.Len(t, result.Sort, 2, "Should have 2 sort specs")
		assert.Equal(t, "created_at", result.Sort[0].Column, "First column should be created_at")
		assert.Equal(t, sortx.OrderDesc, result.Sort[0].Direction, "First direction should be desc")
		assert.Equal(t, "name", result.Sort[1].Column, "Second column should be name")
		assert.Equal(t, sortx.OrderAsc, result.Sort[1].Direction, "Second direction should be asc")
	})

	t.Run("FromMapWithOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"column":    "email",
			"direction": "desc",
		}

		result, err := FromMap[SortSpec](input)
		require.NoError(t, err, "FromMap should succeed")

		assert.Equal(t, "email", result.Column, "Column should match")
		assert.Equal(t, sortx.OrderDesc, result.Direction, "Direction should be desc")
	})

	t.Run("ToMapWithOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := SortSpec{
			Column:    "username",
			Direction: sortx.OrderAsc,
		}

		result, err := ToMap(input)
		require.NoError(t, err, "ToMap should succeed")

		assert.Equal(t, "username", result["column"], "Column should match")
		assert.Equal(t, sortx.OrderAsc, result["direction"], "Direction should match")
	})
}
