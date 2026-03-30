package copier

import "github.com/jinzhu/copier"

// defaultConverters contains built-in type converters for value↔pointer conversions.
var defaultConverters = []TypeConverter{
	stringToStringPtrConverter,
	stringPtrToStringConverter,
	boolToBoolPtrConverter,
	boolPtrToBoolConverter,
	intToIntPtrConverter,
	intPtrToIntConverter,
	int8ToInt8PtrConverter,
	int8PtrToInt8Converter,
	int16ToInt16PtrConverter,
	int16PtrToInt16Converter,
	int32ToInt32PtrConverter,
	int32PtrToInt32Converter,
	int64ToInt64PtrConverter,
	int64PtrToInt64Converter,
	uintToUintPtrConverter,
	uintPtrToUintConverter,
	uint8ToUint8PtrConverter,
	uint8PtrToUint8Converter,
	uint16ToUint16PtrConverter,
	uint16PtrToUint16Converter,
	uint32ToUint32PtrConverter,
	uint32PtrToUint32Converter,
	uint64ToUint64PtrConverter,
	uint64PtrToUint64Converter,
	float32ToFloat32PtrConverter,
	float32PtrToFloat32Converter,
	float64ToFloat64PtrConverter,
	float64PtrToFloat64Converter,
	decimalToDecimalPtrConverter,
	decimalPtrToDecimalConverter,
	timeToTimePtrConverter,
	timePtrToTimeConverter,
	dateTimeToDateTimePtrConverter,
	dateTimePtrToDateTimeConverter,
	dateToDatePtrConverter,
	datePtrToDateConverter,
	timexTimeToTimexTimePtrConverter,
	timexTimePtrToTimexTimeConverter,
}

type (
	// CopyOption configures the copy behavior.
	CopyOption func(option *copier.Option)

	// TypeConverter is an alias for copier.TypeConverter.
	TypeConverter = copier.TypeConverter

	// FieldNameMapping is an alias for copier.FieldNameMapping.
	FieldNameMapping = copier.FieldNameMapping
)

// WithIgnoreEmpty skips copying fields with zero values.
func WithIgnoreEmpty() CopyOption {
	return func(option *copier.Option) {
		option.IgnoreEmpty = true
	}
}

// WithDeepCopy enables deep copying of nested structures.
func WithDeepCopy() CopyOption {
	return func(option *copier.Option) {
		option.DeepCopy = true
	}
}

// WithCaseInsensitive enables case-insensitive field name matching.
func WithCaseInsensitive() CopyOption {
	return func(option *copier.Option) {
		option.CaseSensitive = false
	}
}

// WithFieldNameMapping adds custom field name mappings.
func WithFieldNameMapping(mappings ...FieldNameMapping) CopyOption {
	return func(option *copier.Option) {
		option.FieldNameMapping = append(option.FieldNameMapping, mappings...)
	}
}

// WithTypeConverters adds custom type converters.
func WithTypeConverters(converters ...TypeConverter) CopyOption {
	return func(option *copier.Option) {
		option.Converters = append(option.Converters, converters...)
	}
}

// Copy copies fields from src to dst with optional configuration.
// The dst parameter must be a pointer to a struct.
func Copy(src, dst any, options ...CopyOption) error {
	opt := copier.Option{
		CaseSensitive: true,
		Converters:    defaultConverters,
	}
	for _, apply := range options {
		apply(&opt)
	}

	return copier.CopyWithOption(dst, src, opt)
}
