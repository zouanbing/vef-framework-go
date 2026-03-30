package mapx

import (
	"encoding/json"
	"mime/multipart"
	"reflect"
)

var (
	// Multipart.FileHeader types.
	jsonRawMessageType     = reflect.TypeFor[json.RawMessage]()
	fileHeaderPtrType      = reflect.TypeFor[*multipart.FileHeader]()
	fileHeaderPtrSliceType = reflect.TypeFor[[]*multipart.FileHeader]()
)

// convertJSONRawMessage handles conversion of arbitrary data to json.RawMessage.
// When the target type is json.RawMessage ([]byte), it re-marshals the source value to JSON bytes.
func convertJSONRawMessage(_, to reflect.Type, value any) (any, error) {
	if to != jsonRawMessageType {
		return value, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// convertFileHeader handles conversion from []*multipart.FileHeader to *multipart.FileHeader.
func convertFileHeader(from, to reflect.Type, value any) (any, error) {
	if from == fileHeaderPtrSliceType && to == fileHeaderPtrType {
		if files := value.([]*multipart.FileHeader); len(files) == 1 {
			return files[0], nil
		}
	}

	return value, nil
}
