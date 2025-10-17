package acrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalJSON(t *testing.T) {
	cases := []struct {
		Name   string
		Object any
		JSON   string
	}{
		{
			Name:   "toLowerCamel",
			Object: struct{ FooBar string }{FooBar: "baz"},
			JSON:   `{"fooBar":"baz"}`,
		},
		{
			Name:   "nested",
			Object: struct{ FooBar struct{ BazQux string } }{FooBar: struct{ BazQux string }{BazQux: "quux"}},
			JSON:   `{"fooBar":{"bazQux":"quux"}}`,
		},
		{
			Name:   "array",
			Object: []any{struct{ FooBar string }{FooBar: "baz"}, 0, 1, true, nil},
			JSON:   `[{"fooBar":"baz"},0,1,true,null]`,
		},
		{
			Name:   "string",
			Object: "FooBar",
			JSON:   `"FooBar"`,
		},
		{
			Name:   "number",
			Object: 42,
			JSON:   `42`,
		},
		{
			Name:   "true",
			Object: true,
			JSON:   `true`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			b, err := marshalJSON(tc.Object)
			assert.NoError(t, err)
			if diff := cmp.Diff(tc.JSON, string(b)); diff != "" {
				t.Errorf("expect match (-got +want):\n%s", diff)
			}
		})
	}
}

func TestMatchJSONKey(t *testing.T) {
	cases := []struct {
		Name   string
		From   string
		To     string
		Expect bool
	}{
		{
			Name:   "exact match",
			From:   "FooBar",
			To:     "FooBar",
			Expect: true,
		},
		{
			Name:   "case insensitive match",
			From:   "fooBar",
			To:     "FooBar",
			Expect: true,
		},
		{
			Name:   "not match",
			From:   "FooBar",
			To:     "Foo_Bar",
			Expect: false,
		},
		{
			Name:   "* match",
			From:   "FooBar.Hoge",
			To:     "FooBar.*",
			Expect: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := matchJSONKey(tc.From, tc.To)
			assert.Equal(t, tc.Expect, got)
		})
	}
}

func TestExtractUnknownFieldKey(t *testing.T) {
	var v struct {
		FooBar string `json:"fooBar"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"fooBar":"baz","unknownField":"value"}`))
	dec.DisallowUnknownFields()
	unknownFieldErr := dec.Decode(&v)
	if unknownFieldErr == nil {
		t.Fatal("expected error but got nil")
	}
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "valid unknown field error",
			input:    unknownFieldErr,
			expected: "unknownField",
		},
		{
			name:     "wrapped unknown field error",
			input:    fmt.Errorf("failed to decode: %w", unknownFieldErr),
			expected: "unknownField",
		},
		{
			name:     "non-unknown field error",
			input:    errors.New("some other error"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractUnknownFieldKey(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
