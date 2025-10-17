package acrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/mattn/go-isatty"
)

type InvokeOption struct {
	Payload      *string `help:"payload to invoke. if not specified, read from STDIN"`
	ContentType  *string `help:"The MIME type of the input data in the payload"`
	Accept       *string `help:"Accept header for the response"`
	EndpointName *string `help:"the endpoint name to invoke. if not specified, use the CURRENT endpoint."`

	MCPProtocolVersion *string `name:"mcp-proto-version" help:"The version of the MCP protocol being used."`
	MCPSessionID       *string `name:"mcp-session-id" help:"The identifier of the MCP session."`
	RuntimeSessionID   *string `name:"runtime-session-id" help:"The identifier of the runtime session."`
	RuntimeUserID      *string `name:"runtime-user-id" help:"The user identifier for the runtime session."`
	Baggage            *string `help:"Baggage for distributed tracing"`
	TraceID            *string `name:"trace-id" help:"The trace identifier for request tracking."`
	TraceParent        *string `name:"trace-parent" help:"The parent span identifier for distributed tracing."`
	TraceState         *string `name:"trace-state" help:"The state information for distributed tracing."`
}

func (app *App) Invoke(ctx context.Context, opt *InvokeOption) error {
	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}
	arn, err := app.GetAgentRuntimeARNByName(ctx, *agentRuntime.AgentRuntimeName)
	if err != nil {
		return fmt.Errorf("get agent runtime ARN by name: %w", err)
	}
	slog.InfoContext(ctx, "invoking agent runtime", "name", *agentRuntime.AgentRuntimeName, "arn", arn)
	var payloadReader io.Reader
	if opt.Payload != nil {
		payloadReader = strings.NewReader(*opt.Payload)
	} else {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Println("Enter JSON payloads for the invoking function into STDIN. (Type Ctrl-D to close.)")
		}
		payloadReader = os.Stdin
	}
	bs, err := io.ReadAll(payloadReader)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}
	if opt.ContentType == nil {
		if json.Valid(bs) {
			opt.ContentType = aws.String("application/json")
		} else {
			opt.ContentType = aws.String("text/plain")
		}
	}
	if opt.Accept == nil {
		opt.Accept = aws.String("application/json")
	}
	resp, err := app.client.InvokeAgentRuntime(ctx, &bedrockagentcore.InvokeAgentRuntimeInput{
		AgentRuntimeArn:    aws.String(arn),
		Payload:            bs,
		ContentType:        opt.ContentType,
		Accept:             opt.Accept,
		Qualifier:          aws.String(fillEndpointName(opt.EndpointName)),
		McpProtocolVersion: opt.MCPProtocolVersion,
		McpSessionId:       opt.MCPSessionID,
		RuntimeSessionId:   opt.RuntimeSessionID,
		RuntimeUserId:      opt.RuntimeUserID,
		Baggage:            opt.Baggage,
		TraceId:            opt.TraceID,
		TraceParent:        opt.TraceParent,
		TraceState:         opt.TraceState,
	})
	if err != nil {
		return fmt.Errorf("InvokeAgentRuntime: %w", err)
	}

	stdout := bufio.NewWriter(app.stdout)
	args := []any{
		"status_code", aws.ToInt32(resp.StatusCode),
		"content_type", aws.ToString(resp.ContentType),
	}
	if resp.TraceId != nil {
		args = append(args, "trace_id", *resp.TraceId)
	}
	if resp.TraceParent != nil {
		args = append(args, "trace_parent", *resp.TraceParent)
	}
	if resp.TraceState != nil {
		args = append(args, "trace_state", *resp.TraceState)
	}
	if resp.Baggage != nil {
		args = append(args, "baggage", *resp.Baggage)
	}
	if resp.McpProtocolVersion != nil {
		args = append(args, "mcp_protocol_version", *resp.McpProtocolVersion)
	}
	if resp.McpSessionId != nil {
		args = append(args, "mcp_session_id", *resp.McpSessionId)
	}
	if resp.RuntimeSessionId != nil {
		args = append(args, "runtime_session_id", *resp.RuntimeSessionId)
	}
	slog.InfoContext(ctx, "invoke agent runtime success", args...)
	_, err = io.Copy(stdout, resp.Response)
	stdout.Flush()
	return err
}
