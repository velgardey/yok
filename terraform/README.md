# Terraform Remote State Configuration

This project uses Terraform with remote state stored in AWS S3 and state locking with DynamoDB.

## Configuration

The S3 bucket and DynamoDB table names are configured as variables in `terraform.tfvars`:

```
terraform_state_bucket = "your-bucket-name"
terraform_state_key    = "your/state/path.tfstate"
terraform_lock_table   = "your-lock-table-name"
```

*Note: For security, consider adding terraform.tfvars to .gitignore if you're using custom bucket/table names.*

## Initial Setup

Before you can use the remote backend, you need to create the S3 bucket and DynamoDB table:

```bash
# Initialize Terraform without backend config
pnpm terraform init

# Create the S3 bucket and DynamoDB table
pnpm terraform apply -target=aws_s3_bucket.terraform_state -target=aws_s3_bucket_versioning.terraform_state -target=aws_s3_bucket_server_side_encryption_configuration.terraform_state -target=aws_dynamodb_table.terraform_lock
```

Once the resources are created, reinitialize Terraform to use the remote backend:

```bash
# Initialize with the S3 backend
terraform init -reconfigure
```

If you've changed the bucket, key, or table name in terraform.tfvars, use:

```bash
terraform init -reconfigure \
  -backend-config="bucket=your-bucket-name" \
  -backend-config="key=your/state/path.tfstate" \
  -backend-config="dynamodb_table=your-lock-table-name"
```

## Regular Usage

After the initial setup, you can use Terraform as normal:

```bash
# Plan changes
terraform plan

# Apply changes
terraform apply
```

## State Management

With the remote state configuration:

1. The state file is stored in the S3 bucket defined in terraform.tfvars
2. State locking uses the DynamoDB table defined in terraform.tfvars
3. All state operations are encrypted
4. State versioning is enabled

This ensures:
- Team collaboration without state conflicts
- State history preservation
- Improved security for sensitive data 