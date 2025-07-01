# S3 Support in Containerlab

Containerlab supports using S3 URLs to retrieve topology files and startup configurations for network devices.

## Prerequisites

AWS credentials are automatically discovered using the standard AWS credential chain (in order of precedence):

1. **Environment variables** (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. **Shared credentials file** (`~/.aws/credentials`)
3. **IAM roles** (EC2 instance profiles, ECS task roles, Lambda execution roles)

Optional: You can also use a `.env` file in the current directory to set credentials.

## Usage Examples

### Topology Files from S3

Deploy a lab using a topology file stored in S3:

```bash
containerlab deploy -t s3://my-bucket/topologies/my-lab.clab.yml
```

### Startup Configurations from S3

In your topology file, you can reference startup configurations stored in S3:

```yaml
name: my-lab
topology:
  nodes:
    router1:
      kind: srl
      image: ghcr.io/nokia/srlinux:latest
      startup-config: s3://my-bucket/configs/router1.cli
    
    router2:
      kind: srl
      image: ghcr.io/nokia/srlinux:latest
      startup-config: s3://my-bucket/configs/router2.cli
```

## Configuration

### Environment Variables
```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=us-east-1  # Optional, defaults to us-east-1
```

### .env File
Create a `.env` file in your current directory:
```
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
AWS_REGION=us-east-1
```

### AWS Credentials File
Configure in `~/.aws/credentials`:
```ini
[default]
aws_access_key_id = your-access-key
aws_secret_access_key = your-secret-key
region = us-east-1
```

## S3 URL Format

S3 URLs must follow this format:
```
s3://bucket-name/path/to/file
```
Both the bucket name and file path are required.

## Implementation Details

This implementation uses the MinIO Go client library which provides:
- Full AWS credential chain support
- Compatible with S3 and S3-compatible storage services
- Minimal binary size impact (approximately 1MB vs 7MB with AWS SDK)