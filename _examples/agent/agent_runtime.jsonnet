// Example: Using native functions for dynamic configuration
local identity = std.native('callerIdentity')();
local accountId = identity.account;

// Resolve ECR image URI dynamically
local imageUri = std.native('ecrImageUri')('acrun/sample-agent', 'latest');

{
  agentRuntimeArtifact: {
    containerConfiguration: {
      // Using ecrImageUri to resolve and verify the image exists
      containerUri: imageUri,
    },
  },
  agentRuntimeName: 'sample_agent',
  environmentVariables: {
    env: 'dev',
    // You can also expose the account info
    awsAccountId: accountId,
    awsArn: identity.arn,
    awsUserId: identity.userId,
  },
  networkConfiguration: {
    networkMode: 'PUBLIC',
  },
  protocolConfiguration: {
    serverProtocol: 'HTTP',
  },
  // Dynamically construct IAM role ARN using callerIdentity
  roleArn: 'arn:aws:iam::' + accountId + ':role/AmazonBedrockAgentCoreRuntimeDefaultServiceRole',
}
