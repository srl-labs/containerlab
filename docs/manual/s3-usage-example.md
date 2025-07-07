# S3 Support in Containerlab

Containerlab supports using S3 URLs to retrieve topology files and startup configurations for network devices.

## Prerequisites

AWS credentials are automatically discovered using the standard AWS credential chain (in order of precedence):

1. **Environment variables** (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. **Shared credentials file** (`~/.aws/credentials`)
3. **IAM roles** (EC2 instance profiles)

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

## Authentication

### Environment Variables

```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=us-east-1  # Optional, defaults to us-east-1
```

### AWS Credentials File

Configure in `~/.aws/credentials`:

```ini
[default]
aws_access_key_id = your-access-key
aws_secret_access_key = your-secret-key
region = us-east-1
```
