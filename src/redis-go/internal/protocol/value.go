package protocol

type ValueType string

const (
	TypeSimpleString ValueType = "simple_string"
	TypeError        ValueType = "error"
	TypeInteger      ValueType = "integer"
	TypeBulkString   ValueType = "bulk_string"
	TypeArray        ValueType = "array"
)

type Value struct {
	Type  ValueType
	Str   string
	Int   int64
	Array []Value
	Null  bool
}
