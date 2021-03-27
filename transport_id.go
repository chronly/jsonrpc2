package jsonrpc2

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type idType int

const (
	idTypeNull idType = iota
	idTypeString
	idTypeNumber
)

// id represents a JSON-RPC 2.0 id. The zero value is an undefined id.
type id struct {
	value   string
	ty      idType
	defined bool
}

func newUndefinedID() id { return id{} }

func newNullID() id {
	return id{ty: idTypeNull, defined: true}
}

func newStringID(value string) id {
	return id{value: value, ty: idTypeString, defined: true}
}

func newNumberID(value int64) id {
	return id{value: strconv.FormatInt(value, 10), ty: idTypeNumber, defined: true}
}

func (v *id) IsNull() bool      { return v.ty == idTypeNull }
func (v *id) IsString() bool    { return v.ty == idTypeString }
func (v *id) IsNumber() bool    { return v.ty == idTypeNumber }
func (v *id) IsUndefined() bool { return !v.defined }
func (v *id) String() string    { return v.value }

func (v *id) UnmarshalJSON(bb []byte) error {
	v.defined = true

	// Try unmarshaling an *int first. This covers null types
	// and numeric IDs.
	var numericVal *int64
	if err := json.Unmarshal(bb, &numericVal); err == nil {
		if numericVal == nil {
			*v = newNullID()
			return nil
		}
		*v = newNumberID(*numericVal)
		return nil
	}

	// Fall back to string.
	var stringVal string
	if err := json.Unmarshal(bb, &stringVal); err == nil {
		*v = newStringID(stringVal)
		return nil
	}

	return fmt.Errorf("id must be string, number, or null")
}

func (v id) MarshalJSON() ([]byte, error) {
	switch v.ty {
	case idTypeNumber:
		val, err := strconv.Atoi(v.value)
		if err != nil {
			return nil, fmt.Errorf("invalid numeric id: %w", err)
		}
		return json.Marshal(val)
	case idTypeString:
		return json.Marshal(v.value)
	case idTypeNull:
		return json.Marshal(nil)
	default:
		return nil, fmt.Errorf("unknown id type")
	}
}
