# acrun

acrun is a deployment tool for **AWS Bedrock AgentCore Runtime**.

A lightweight, specialized deployment tool inspired by [lambroll](https://github.com/fujiwara/lambroll) and [ecspresso](https://github.com/kayac/ecspresso).

## Features

- **Simple deployment workflow**: `init`, `diff`, `deploy`, `invoke`
- **Jsonnet template support**: Dynamic configuration with native functions
- **Infrastructure separation**: Focuses on deployment, not infrastructure management
- **CI/CD friendly**: Predictable behavior, clear exit codes

## Jsonnet Native Functions

acrun provides several native functions for use in `agent_runtime.jsonnet`, following Jsonnet's camelCase naming convention:

### `callerIdentity()`

Returns AWS caller identity information from STS GetCallerIdentity API.

**Returns**: Object with `account`, `arn`, and `userId` fields (camelCase).

**Example**:
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

**Example**:
```jsonnet
{
  environmentVariables: {
    stage: std.native('env')('STAGE', 'dev'),
  },
}
```

### `mustEnv(name)`

Returns environment variable value, or raises an error if not set.

**Example**:
```jsonnet
{
  environmentVariables: {
    apiKey: std.native('mustEnv')('API_KEY'),
  },
}
```

### `ecrImageUri(repositoryName, imageTag)`

Resolves ECR container image URI and verifies the repository exists.

**Parameters**:
- `repositoryName`: ECR repository name (e.g., `"acrun/sample-mcp"`)
- `imageTag`: Image tag (e.g., `"latest"`, `"v1.0.0"`)

**Returns**: Full ECR image URI string (e.g., `"123456789012.dkr.ecr.us-west-2.amazonaws.com/acrun/sample-mcp:latest"`).

**Features**:
- Verifies repository exists in ECR using `DescribeRepositories`
- Automatically obtains registry ID and repository URI
- Constructs the complete ECR URI with the specified tag

**Example**:
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

**Use with environment variables**:
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

**Prerequisites**: Set `--tfstate` flag or `ACRUN_TFSTATE` environment variable to point to your Terraform state file (local path or S3 URL).

**Parameters**:
- `address`: Terraform resource address (e.g., `"aws_iam_role.agent_runtime.arn"`, `"data.aws_subnet.private.id"`)

**Returns**: The value from the Terraform state at the specified address.

**Example**:
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

**Supported state locations**:
- Local file: `/path/to/terraform.tfstate`
- S3: `s3://bucket/path/to/terraform.tfstate`
- HTTP/HTTPS: `https://example.com/terraform.tfstate`
- Google Cloud Storage: `gs://bucket/path/to/terraform.tfstate`
- Azure Blob Storage: `azurerm://container/path/to/terraform.tfstate`
