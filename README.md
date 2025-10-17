# acrun

Simple, predictable deployments for AWS Bedrock AgentCore Runtime.

acrun is a lightweight, specialized deploy tool inspired by [lambroll](https://github.com/fujiwara/lambroll) (Lambda) and [ecspresso](https://github.com/kayac/ecspresso) (ECS). It focuses on the deployment operation only and plays well with infrastructure managed elsewhere (Terraform/CloudFormation).

## Features

- Simple workflow: `init`, `diff`, `deploy`, `invoke`
- Jsonnet/JSON config with useful native functions
- Infrastructure separation; no scaffolding, just deploy
- CI/CD friendly with clear exit codes and colored diffs

## Install

- Go: `go install github.com/mashiike/acrun/cmd/acrun@latest`
- Binaries: download from GitHub Releases (GoReleaser; darwin/linux/windows, amd64/arm64)

## Quick Start

1) Initialize from an existing runtime

```bash
acrun init --agent-runtime-name my-agent --format jsonnet
# writes ./agent_runtime.jsonnet (or json via --format json)
```

2) Edit `agent_runtime.jsonnet`

```jsonnet
{
  agentRuntimeName: "my-agent",
  roleArn: "arn:aws:iam::123456789012:role/MyAgentRole",
  agentRuntimeArtifact: {
    containerConfiguration: {
      containerUri: std.native('ecrImageUri')('my-agent', std.native('env')('IMAGE_TAG', 'latest')),
    },
  },
  environmentVariables: { stage: std.native('env')('STAGE', 'dev') },
}
```

3) See the diff against remote

```bash
acrun diff --exit-code            # exit code 2 when different
acrun diff --ignore '.agentRuntimeVersion'   # jq query to ignore fields
```

4) Deploy to an endpoint (non-DEFAULT)

```bash
acrun deploy --endpoint-name staging
# creates/updates runtime and the endpoint, then points it to the new version
```

5) Invoke for a quick check

```bash
echo '{"inputText":"ping"}' | acrun invoke --endpoint-name staging
```

## Commands

- `init`: Fetch runtime by name and write `agent_runtime.jsonnet` or `agent_runtime.json`.
  - Flags: `--agent-runtime-name`, `--qualifier <endpoint|version>`, `--format json|jsonnet`, `--force-overwrite`
- `diff`: Compare local file with remote runtime (version or endpoint).
  - Flags: `--qualifier <endpoint|version>` (default: `current`), `--ignore <jq>`, `--exit-code`
- `deploy`: Create/update runtime and update or create the specified endpoint.
  - Flags: `--endpoint-name <name>` (required; cannot be `DEFAULT`), `--dry-run`
- `invoke`: Call the deployed agent runtime with a payload.
  - Flags: `--payload`, `--content-type`, `--accept`, `--endpoint-name` (default: `current`), plus MCP/trace headers (`--mcp-proto-version`, `--mcp-session-id`, `--runtime-session-id`, `--runtime-user-id`, `--baggage`, `--trace-id`, `--trace-parent`, `--trace-state`)
- `render`: Print normalized config from local file.
  - Flags: `--format json|jsonnet`
- `delete`: Delete the runtime (safe by default).
  - Flags: `--force`, `--dry-run`
- `rollback`: Point an endpoint to an older version.
  - Flags: `--endpoint-name <name>` (cannot be `DEFAULT`), `--version <n>` (default: current-1), `--dry-run`

Global flags:

- `--agent-runtime <path>`: Path to config file (defaults: `agent_runtime.jsonnet` or `agent_runtime.json` in CWD)
- `--tfstate <url|path>`: Terraform state location; same as `ACRUN_TFSTATE`
- `--log-level <debug|info|warn|error>` and `--log-format <text|json>`

Exit codes:

- `diff` returns exit code 2 (via `--exit-code`) when there are differences; other commands use 0/1.

## Configuration

acrun reads `agent_runtime.jsonnet` or `agent_runtime.json` in the working directory by default. Fields are lowerCamelCase to align with AWS API. You can use Jsonnet to compose per-environment configs.

Jsonnet native functions are provided for convenience. See the detailed section below for usage and examples.

Example: Terraform integration

```jsonnet
local tf = std.native('tfstate');
{
  roleArn: tf('aws_iam_role.agent_runtime.arn'),
  agentRuntimeArtifact: {
    containerConfiguration: {
      containerUri: tf('aws_ecr_repository.agent.repository_url') + ':' + std.native('env')('IMAGE_TAG','latest'),
    },
  },
  networkConfiguration: {
    networkMode: 'VPC',
    vpcConfig: {
      subnetIds: [ tf('aws_subnet.private["az-a"].id'), tf('aws_subnet.private["az-b"].id') ],
      securityGroupIds: [ tf('aws_security_group.agent_runtime.id') ],
    },
  },
}
```

Terraform state locations supported: local files, S3 (`s3://...`), HTTP/HTTPS, GCS (`gs://...`), Azure Blob (`azurerm://...`).

## Jsonnet Native Functions

acrun provides several native functions for use in `agent_runtime.jsonnet`, following Jsonnet's camelCase naming convention:

### `callerIdentity()`

Returns AWS caller identity information from STS GetCallerIdentity API.

- Returns: Object with `account`, `arn`, and `userId` fields (camelCase).

Example:
```jsonnet
local identity = std.native('callerIdentity')();
local accountId = identity.account;

{
  roleArn: 'arn:aws:iam::' + accountId + ':role/MyRole',
  agentRuntimeArtifact: {
    containerConfiguration: {
      containerUri: accountId + '.dkr.ecr.us-west-2.amazonaws.com/my-agent:latest',
    },
  },
  environmentVariables: {
    awsAccountId: accountId,
    awsArn: identity.arn,
    awsUserId: identity.userId,
  },
}
```

### `env(name, default)`

Returns environment variable value, or default if not set.

Example:
```jsonnet
{
  environmentVariables: {
    stage: std.native('env')('STAGE', 'dev'),
  },
}
```

### `mustEnv(name)`

Returns environment variable value, or raises an error if not set.

Example:
```jsonnet
{
  environmentVariables: {
    apiKey: std.native('mustEnv')('API_KEY'),
  },
}
```

### `ecrImageUri(repositoryName, imageTag)`

Resolves ECR container image URI and verifies the repository exists.

- Parameters:
  - `repositoryName`: ECR repository name (e.g., `"acrun/sample-mcp"`)
  - `imageTag`: Image tag (e.g., `"latest"`, `"v1.0.0"`)
- Returns: Full ECR image URI string (e.g., `"123456789012.dkr.ecr.us-west-2.amazonaws.com/acrun/sample-mcp:latest"`).
- Features:
  - Verifies repository exists in ECR using DescribeRepositories
  - Automatically obtains registry ID and repository URI
  - Constructs the complete ECR URI with the specified tag

Example:
```jsonnet
{
  agentRuntimeArtifact: {
    containerConfiguration: {
      // Automatically resolves account/region and verifies image exists
      containerUri: std.native('ecrImageUri')('my-agent', 'v1.0.0'),
    },
  },
}
```

Use with environment variables:
```jsonnet
local tag = std.native('env')('IMAGE_TAG', 'latest');
{
  agentRuntimeArtifact: {
    containerConfiguration: {
      containerUri: std.native('ecrImageUri')('my-agent', tag),
    },
  },
}
```

### `tfstate(address)`

Looks up values from Terraform state file.

- Prerequisites: Set `--tfstate` flag or `ACRUN_TFSTATE` environment variable to point to your Terraform state file (local path or S3 URL).
- Parameters:
  - `address`: Terraform resource address (e.g., `"aws_iam_role.agent_runtime.arn"`, `"data.aws_subnet.private.id"`)
- Returns: The value from the Terraform state at the specified address.

Example:
```bash
# Set tfstate location
export ACRUN_TFSTATE="s3://my-bucket/terraform.tfstate"
# or
acrun deploy --tfstate s3://my-bucket/terraform.tfstate
```

```jsonnet
// In agent_runtime.jsonnet
local tfstate = std.native('tfstate');

{
  // Reference IAM role managed by Terraform
  roleArn: tfstate('aws_iam_role.agent_runtime.arn'),

  agentRuntimeArtifact: {
    containerConfiguration: {
      // Reference ECR repository
      containerUri: tfstate('aws_ecr_repository.agent.repository_url') + ':latest',
    },
  },

  networkConfiguration: {
    networkMode: 'VPC',
    vpcConfig: {
      // Reference VPC subnets
      subnetIds: [
        tfstate('aws_subnet.private["az-a"].id'),
        tfstate('aws_subnet.private["az-b"].id'),
      ],
      // Reference security groups
      securityGroupIds: [
        tfstate('aws_security_group.agent_runtime.id'),
      ],
    },
  },
}
```

Supported state locations:
- Local file: `/path/to/terraform.tfstate`
- S3: `s3://bucket/path/to/terraform.tfstate`
- HTTP/HTTPS: `https://example.com/terraform.tfstate`
- Google Cloud Storage: `gs://bucket/path/to/terraform.tfstate`
- Azure Blob Storage: `azurerm://container/path/to/terraform.tfstate`

## Endpoint Semantics

- `current` qualifier resolves the version backing the named endpoint and is used by default in `diff`/`invoke`.
- `DEFAULT` endpoint is reserved; `deploy`/`rollback` against `DEFAULT` are intentionally blocked. Use your own endpoint names (e.g., `dev`, `staging`, `prod`).

## Examples

See `_examples/agent/` for a minimal agent project and example configs.

## Build from source

```bash
go build ./cmd/acrun
```

## Acknowledgements

acrun’s philosophy is heavily inspired by:

- lambroll: https://github.com/fujiwara/lambroll
- ecspresso: https://github.com/kayac/ecspresso

## License

See `LICENSE`.
