package jsonrpc2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_id_Unmarshal(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect id
	}{
		{
			name:   "null",
			input:  `null`,
			expect: newNullID(),
		},
		{
			name:   "number",
			input:  `12345`,
			expect: newNumberID(12345),
		},
		{
			name:   "string",
			input:  `"hello"`,
			expect: newStringID("hello"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var actual id
			err := json.Unmarshal([]byte(tc.input), &actual)
			require.NoError(t, err)
			require.Equal(t, actual, tc.expect)
		})
	}
}

func Test_id_Marshal(t *testing.T) {
	tt := []struct {
		name   string
		input  id
		expect string
	}{
		{
			name:   "null",
			input:  newNullID(),
			expect: `null`,
		},
		{
			name:   "number",
			input:  newNumberID(12345),
			expect: `12345`,
		},
		{
			name:   "string",
			input:  newStringID("hello"),
			expect: `"hello"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			bb, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.JSONEq(t, tc.expect, string(bb))
		})
	}
}
