package acrun

import "testing"

func TestToLowerCamelCase(t *testing.T) {
	cases := []struct {
		Name     string
		Input    string
		Expected string
	}{
		{
			Name:     "simple snake_case",
			Input:    "ssm_lookup",
			Expected: "ssmLookup",
		},
		{
			Name:     "single word",
			Input:    "simple",
			Expected: "simple",
		},
		{
			Name:     "multiple underscores",
			Input:    "get_secret_value",
			Expected: "getSecretValue",
		},
		{
			Name:     "already camelCase (no underscores)",
			Input:    "alreadyCamelCase",
			Expected: "alreadycamelcase",
		},
		{
			Name:     "empty string",
			Input:    "",
			Expected: "",
		},
		{
			Name:     "single underscore",
			Input:    "_",
			Expected: "",
		},
		{
			Name:     "leading underscore",
			Input:    "_leading",
			Expected: "Leading",
		},
		{
			Name:     "trailing underscore",
			Input:    "trailing_",
			Expected: "trailing",
		},
		{
			Name:     "multiple consecutive underscores",
			Input:    "foo__bar",
			Expected: "fooBar",
		},
		{
			Name:     "uppercase input",
			Input:    "SSM_LOOKUP",
			Expected: "ssmLOOKUP",
		},
		{
			Name:     "mixed case input",
			Input:    "My_Variable_Name",
			Expected: "myVariableName",
		},
		{
			Name:     "single character parts",
			Input:    "a_b_c",
			Expected: "aBC",
		},
		{
			Name:     "numbers in parts",
			Input:    "get_value_123",
			Expected: "getValue123",
		},
		{
			Name:     "aws style",
			Input:    "aws_account_id",
			Expected: "awsAccountId",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual := ToLowerCamelCase(c.Input)
			if actual != c.Expected {
				t.Errorf("ToLowerCamelCase(%q) = %q, expected %q", c.Input, actual, c.Expected)
			}
		})
	}
}
