# YOK Project

A complete infrastructure and application deployment solution with Docker, Terraform, and AWS.

## Project Structure

```
yok/
├── api/                   # API server code and configurations
├── build-server/          # Build server components
├── docker-compose.yaml    # Docker Compose configuration
├── reverse-proxy/         # Reverse proxy service
├── scripts/               # Utility scripts
├── terraform/             # Infrastructure as Code (Terraform)
└── README.md              # This file
```

## Prerequisites

- AWS account with appropriate permissions
- AWS CLI installed and configured
- Terraform 1.0+ installed
- Docker and Docker Compose installed
- Domain name with access to DNS settings

## Environment Setup

### 1. Create Local Environment File

Create a `.env` file in the root directory with the following variables:

```
# Database
DATABASE_URL=your_database_url
DIRECT_URL=your_direct_database_url

# Kafka
KAFKA_BROKER=your_kafka_broker
KAFKA_USERNAME=your_kafka_username
KAFKA_PASSWORD=your_kafka_password
KAFKA_TOPIC=your_kafka_topic

# ClickHouse
CLICKHOUSE_URL=your_clickhouse_url
CLICKHOUSE_DATABASE=your_clickhouse_database

# AWS Configuration
AWS_REGION=ap-south-1
AWS_S3_BUCKET=your_s3_bucket

# AWS ECS Configuration (if needed)
AWS_ECS_CLUSTER=your_ecs_cluster
AWS_ECS_TASK_DEFINITION=your_task_definition
AWS_ECS_CONTAINER_NAME=your_container_name
AWS_ECS_SUBNETS=subnet-id1,subnet-id2
AWS_ECS_SECURITY_GROUPS=sg-id1

# AWS Credentials (needed for Docker services)
AWS_ACCESS_KEY_ID=your_aws_access_key_id
AWS_SECRET_ACCESS_KEY=your_aws_secret_access_key
```

### 2. SSH Key Setup

1. Create an SSH key pair in the AWS region you plan to use (default: ap-south-1)
2. Name the key pair `yok-key`
3. Save the private key file (yok-key.pem) securely

## Deployment Steps

### 1. Infrastructure Deployment with Terraform

```bash
# Change to terraform directory
cd terraform

# Initialize Terraform (first time setup)
terraform init

# If this is your first time, create the S3 bucket and DynamoDB table for state management
terraform apply -target=aws_s3_bucket.terraform_state \
  -target=aws_dynamodb_table.terraform_lock \
  -target=aws_s3_bucket_versioning.terraform_state \
  -target=aws_s3_bucket_server_side_encryption_configuration.terraform_state

# Reconfigure Terraform to use the remote backend
terraform init -reconfigure

# Plan the deployment
terraform plan

# Apply the configuration
terraform apply
```

### 2. Upload Environment Variables to AWS Parameter Store

After successful infrastructure deployment:

```bash
# Make scripts executable if needed
chmod +x scripts/upload-env-to-params.sh scripts/fetch-params-to-env.sh

# Upload environment variables to AWS Parameter Store (excluding AWS credentials)
./scripts/upload-env-to-params.sh

# Alternatively, include AWS credentials in Parameter Store (exercise caution)
./scripts/upload-env-to-params.sh --include-aws-credentials
```

### 3. Domain Configuration

1. Configure your domain's DNS settings to point to the EC2 instance:
   - Add an A record for `api.yourdomain.com` pointing to the EC2 public IP
   - Add a wildcard A record for `*.yourdomain.com` pointing to the EC2 public IP

### 4. Connect to EC2 Instance

```bash
# Use the SSH command provided in Terraform output
ssh -i 'yok-key.pem' ec2-user@<EC2_IP_ADDRESS>
```

### 5. SSL Certificate Setup

On the EC2 instance:

```bash
# Run the certificate setup script
sudo /home/ec2-user/setup-certificates.sh
```

Follow the prompts to set up SSL certificates for:
1. `api.yourdomain.com`
2. `*.yourdomain.com` (requires DNS validation)

### 6. Verify Deployment

1. Check Docker services:
   ```bash
   cd /home/ec2-user/yok-app
   sudo docker-compose ps
   ```

2. Verify Nginx configuration:
   ```bash
   sudo nginx -t
   ```

3. Test API access:
   ```
   curl https://api.yourdomain.com/health
   ```

## Maintenance

### Restarting Services

If you need to restart the services:

```bash
# SSH into the EC2 instance
ssh -i 'yok-key.pem' ec2-user@<EC2_IP_ADDRESS>

# Run the service restart script
/home/ec2-user/start-services.sh
```

### Updating Environment Variables

1. Update your local `.env` file
2. Upload the changes to Parameter Store:
   ```bash
   ./scripts/upload-env-to-params.sh
   ```
3. SSH into the EC2 instance and restart services:
   ```bash
   ssh -i 'yok-key.pem' ec2-user@<EC2_IP_ADDRESS>
   cd /home/ec2-user/yok-app
   ./scripts/fetch-params-to-env.sh --include-aws-credentials
   sudo docker-compose up -d
   ```

## Infrastructure Modifications

If you need to modify the infrastructure:

1. Update the Terraform files
2. Run:
   ```bash
   cd terraform
   terraform plan  # Review changes
   terraform apply # Apply changes
   ```

## AWS Credentials for Docker

Docker services require AWS credentials to interact with AWS services. There are two approaches to providing these credentials:

1. **IAM Role (EC2 Only)**: 
   - The EC2 instance uses an IAM role with appropriate permissions
   - This works for services running on the EC2 instance but not for local development

2. **Environment Variables**:
   - For Docker services to access AWS resources, credentials must be in the .env file
   - Use `--include-aws-credentials` when fetching parameters:
     ```bash
     ./scripts/fetch-params-to-env.sh --include-aws-credentials
     ```
   - This adds AWS credentials to the .env file used by Docker Compose

### Security Best Practices for AWS Credentials

- For local development, use your own AWS profile credentials
- For EC2, credentials are managed automatically through the IAM role
- Set restrictive permissions on the .env file (chmod 600)
- Never commit .env files to source control
- Rotate AWS credentials regularly

## Troubleshooting

### 502 Bad Gateway Error

If you encounter 502 Bad Gateway errors:

1. Check Docker services:
   ```bash
   sudo docker-compose ps
   sudo docker-compose logs
   ```

2. Verify Nginx is running:
   ```bash
   sudo systemctl status nginx
   sudo cat /var/log/nginx/error.log
   ```

3. Ensure environment variables are properly loaded:
   ```bash
   cat .env  # Check if variables exist
   ./scripts/fetch-params-to-env.sh --include-aws-credentials  # Fetch from Parameter Store again
   sudo docker-compose config  # Verify Docker can see all variables
   ```

### AWS Credential Issues

If your Docker services cannot access AWS resources:

1. Verify AWS credentials are in the .env file:
   ```bash
   grep AWS .env
   ```

2. Check if Docker services have access to credentials:
   ```bash
   sudo docker-compose config | grep AWS_ACCESS_KEY_ID
   sudo docker-compose config | grep AWS_SECRET_ACCESS_KEY
   ```

3. Verify AWS credentials work:
   ```bash
   aws s3 ls --profile default  # Should list S3 buckets
   ```

4. Re-fetch parameters with AWS credentials:
   ```bash
   ./scripts/fetch-params-to-env.sh --include-aws-credentials
   ```

### State Lock Issues

If you encounter Terraform state lock issues:

```bash
# List current locks
aws dynamodb scan --table-name yok-terraform-lock --attributes-to-get LockID

# Force unlock (use with caution!)
terraform force-unlock <LOCK_ID>
```

## Security Considerations

- Keep your `.env` file secure and never commit it to version control
- Rotate AWS credentials regularly
- Monitor EC2 instance for security updates
- Review IAM permissions regularly 