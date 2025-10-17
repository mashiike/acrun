package acrun

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	cases := []struct {
		Name   string
		Format string
	}{
		{
			Name:   "json format",
			Format: "json",
		},
		{
			Name:   "jsonnet format",
			Format: "jsonnet",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			app := &App{
				agentRuntimeFilepath: "testdata/agent_runtime.json",
			}
			var stdout, stderr bytes.Buffer
			app.SetOutput(&stdout, &stderr)

			opt := &RenderOption{
				Format: tc.Format,
			}

			err := app.Render(context.Background(), opt)
			require.NoError(t, err)

			output := stdout.String()
			require.NotEmpty(t, output)

			switch tc.Format {
			case "json":
				// Verify output is valid JSON
				var result map[string]interface{}
				err = json.Unmarshal([]byte(output), &result)
				require.NoError(t, err, "output should be valid JSON")

				// Verify required fields exist (lowerCamelCase)
				require.Contains(t, result, "agentRuntimeName")
				require.Contains(t, result, "roleArn")
			case "jsonnet":
				// Verify output contains Jsonnet-style content
				require.Contains(t, output, "agentRuntimeName:")
				require.Contains(t, output, "roleArn:")
			}
		})
	}
}
