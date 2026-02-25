package reflectx

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsEmpty tests IsEmpty for all supported value kinds.
func TestIsEmpty(t *testing.T) {
	var (
		nilStringPtr *string
		nilIntPtr    *int
		emptyStr     = ""
		nonEmptyStr  = "hello"
		zeroInt      = 0
		nilSlice     []int
		nilMap       map[string]int
		nilChan      chan int
		nilFunc      func()
	)

	type testStruct struct {
		Name string
		Age  int
	}

	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"Nil", nil, true},

		// bool
		{"FalseBool", false, true},
		{"TrueBool", true, false},

		// signed integers
		{"ZeroInt", 0, true},
		{"ZeroInt8", int8(0), true},
		{"ZeroInt16", int16(0), true},
		{"ZeroInt32", int32(0), true},
		{"ZeroInt64", int64(0), true},
		{"NonZeroInt", 1, false},
		{"NegativeInt64", int64(-1), false},

		// unsigned integers
		{"ZeroUint", uint(0), true},
		{"ZeroUint8", uint8(0), true},
		{"ZeroUint16", uint16(0), true},
		{"ZeroUint32", uint32(0), true},
		{"ZeroUint64", uint64(0), true},
		{"ZeroUintptr", uintptr(0), true},
		{"NonZeroUint", uint(1), false},

		// floats
		{"ZeroFloat32", float32(0), true},
		{"ZeroFloat64", float64(0), true},
		{"NonZeroFloat64", float64(0.1), false},

		// strings
		{"EmptyString", "", true},
		{"NonEmptyString", "hello", false},

		// *string (deep check)
		{"NilStringPointer", nilStringPtr, true},
		{"EmptyStringPointer", &emptyStr, true},
		{"NonEmptyStringPointer", &nonEmptyStr, false},

		// non-string pointers (shallow)
		{"NilIntPointer", nilIntPtr, true},
		{"ZeroIntPointer", &zeroInt, false},

		// slices
		{"NilSlice", nilSlice, true},
		{"EmptySlice", []int{}, true},
		{"NonEmptySlice", []int{1}, false},

		// maps
		{"NilMap", nilMap, true},
		{"EmptyMap", map[string]int{}, true},
		{"NonEmptyMap", map[string]int{"a": 1}, false},

		// channels
		{"NilChan", nilChan, true},
		{"NonNilChan", make(chan int), false},

		// functions
		{"NilFunc", nilFunc, true},
		{"NonNilFunc", func() {}, false},

		// structs
		{"ZeroStruct", testStruct{}, true},
		{"NonZeroStruct", testStruct{Name: "a"}, false},

		// arrays
		{"ZeroLengthArray", [0]int{}, true},
		{"NonZeroLengthArray", [1]int{0}, false},

		// complex (unsupported kind, falls to default)
		{"ZeroComplex", complex(0, 0), false},
		{"NonZeroComplex", complex(1, 2), false},

		// reflect.Value input
		{"ReflectValueEmptyString", reflect.ValueOf(""), true},
		{"ReflectValueNonEmptyString", reflect.ValueOf("hi"), false},
		{"ZeroReflectValue", reflect.Value{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsEmpty(tt.input), "IsEmpty result should match expected")
		})
	}
}

// TestIsNotEmpty tests IsNotEmpty as the inverse of IsEmpty.
func TestIsNotEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"Nil", nil, false},
		{"EmptyString", "", false},
		{"NonEmptyString", "hi", true},
		{"NonZeroInt", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsNotEmpty(tt.input), "IsNotEmpty result should match expected")
		})
	}
}

// TestEqual tests Equal for all supported comparison scenarios.
func TestEqual(t *testing.T) {
	type comparableStruct struct{ X int }
	type nonComparableStruct struct{ Items []int }

	var (
		ptrA        = new(int)
		ptrB        = new(int)
		nilSliceA   []int
		nilSliceB   []int
		nilMapA     map[string]int
	)

	tests := []struct {
		name     string
		a, b     any
		expected bool
	}{
		// nil
		{"BothNil", nil, nil, true},
		{"NilVsNonNil", nil, 0, false},
		{"NonNilVsNil", "", nil, false},

		// signed integers
		{"SameInt", 1, 1, true},
		{"CrossSignedInt8ToInt64", int8(1), int64(1), true},
		{"CrossSignedInt32ToInt16", int32(42), int16(42), true},
		{"DifferentInt", 1, 2, false},

		// unsigned integers
		{"CrossUnsignedUintToUint64", uint(1), uint64(1), true},
		{"CrossUnsignedUint8ToUint32", uint8(255), uint32(255), true},
		{"DifferentUint", uint(1), uint(2), false},

		// floats
		{"CrossFloat32ToFloat64", float32(1.5), float64(1.5), true},
		{"DifferentFloat", float64(1.5), float64(2.5), false},

		// cross numeric categories
		{"IntVsUint", int(1), uint(1), false},
		{"IntVsFloat", int(1), float64(1), false},
		{"UintVsFloat", uint(1), float64(1), false},

		// strings
		{"SameString", "hello", "hello", true},
		{"DifferentString", "hello", "world", false},

		// bools
		{"BothTrue", true, true, true},
		{"BothFalse", false, false, true},
		{"TrueVsFalse", true, false, false},

		// pointers (shallow: compares address)
		{"SamePointer", ptrA, ptrA, true},
		{"DifferentPointers", ptrA, ptrB, false},
		{"NilPointersSameType", (*int)(nil), (*int)(nil), true},

		// non-comparable nilable types
		{"NilSlicesSameType", nilSliceA, nilSliceB, true},
		{"NilVsNonNilSlice", nilSliceA, []int{1}, false},
		{"NilVsEmptyMap", nilMapA, map[string]int{}, false},
		{"NonNilSlicesShallow", []int{1}, []int{1}, false},

		// non-comparable non-nilable types
		{"NonComparableStruct", nonComparableStruct{Items: []int{1}}, nonComparableStruct{Items: []int{1}}, false},

		// different types
		{"StringVsInt", "1", 1, false},
		{"BoolVsInt", true, 1, false},

		// comparable structs
		{"SameStruct", comparableStruct{X: 1}, comparableStruct{X: 1}, true},
		{"DifferentStruct", comparableStruct{X: 1}, comparableStruct{X: 2}, false},

		// reflect.Value input
		{"ReflectValueSameInt", reflect.ValueOf(42), reflect.ValueOf(42), true},
		{"ReflectValueCrossSignedInt", reflect.ValueOf(int8(1)), int64(1), true},
		{"BothInvalidReflectValue", reflect.Value{}, reflect.Value{}, true},
		{"InvalidVsValidReflectValue", reflect.Value{}, reflect.ValueOf(1), false},
		{"ValidVsInvalidReflectValue", reflect.ValueOf(1), reflect.Value{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Equal(tt.a, tt.b), "Equal result should match expected")
		})
	}
}

// TestContains tests Contains for slices, arrays, maps, and strings.
func TestContains(t *testing.T) {
	var (
		nilSlice []int
		nilMap   map[string]int
	)

	slice := []int{1, 2, 3}
	ptrToSlice := &slice

	tests := []struct {
		name       string
		collection any
		element    any
		expected   bool
	}{
		// nil / invalid collection
		{"NilCollection", nil, 1, false},
		{"InvalidReflectValue", reflect.Value{}, 1, false},
		{"NilPointerToSlice", (*[]int)(nil), 1, false},

		// slices
		{"SliceContains", []int{1, 2, 3}, 2, true},
		{"SliceNotContains", []int{1, 2, 3}, 4, false},
		{"EmptySlice", []int{}, 1, false},
		{"NilSlice", nilSlice, 1, false},
		{"SliceCrossNumericType", []int{1, 2, 3}, int64(2), true},
		{"SliceContainsNil", []any{nil, "a"}, nil, true},
		{"SliceNotContainsNil", []any{"a", "b"}, nil, false},

		// arrays
		{"ArrayContains", [3]string{"a", "b", "c"}, "b", true},
		{"ArrayNotContains", [3]string{"a", "b", "c"}, "d", false},

		// maps (key lookup)
		{"MapKeyExists", map[string]int{"a": 1, "b": 2}, "a", true},
		{"MapKeyNotExists", map[string]int{"a": 1, "b": 2}, "c", false},
		{"MapCrossNumericKey", map[int64]string{1: "a", 2: "b"}, int(1), true},
		{"MapInterfaceKeyString", map[any]int{"a": 1, 42: 2}, "a", true},
		{"MapInterfaceKeyInt", map[any]int{"a": 1, 42: 2}, 42, true},
		{"MapInterfaceKeyMissing", map[any]int{"a": 1, 42: 2}, "missing", false},
		{"MapNilElement", map[string]int{"a": 1}, nil, false},
		{"NilMap", nilMap, "a", false},
		{"MapInconvertibleKeyType", map[string]int{"a": 1}, struct{}{}, false},

		// strings (substring)
		{"StringContainsSubstring", "hello world", "world", true},
		{"StringContainsEmpty", "hello", "", true},
		{"StringNotContains", "hello", "xyz", false},
		{"StringNonStringElement", "hello", 123, false},

		// pointer dereferencing
		{"PointerToSlice", &slice, 2, true},
		{"NestedPointerToSlice", &ptrToSlice, 2, true},

		// unsupported collection type
		{"UnsupportedType", 42, 1, false},

		// reflect.Value collection
		{"ReflectValueSliceContains", reflect.ValueOf([]int{10, 20, 30}), 20, true},
		{"ReflectValueSliceNotContains", reflect.ValueOf([]int{10, 20, 30}), 99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Contains(tt.collection, tt.element), "Contains result should match expected")
		})
	}
}
