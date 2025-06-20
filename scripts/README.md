# Environment Variable Management Scripts

This directory contains scripts to manage environment variables for the YOK application using AWS Parameter Store.

## Overview

The scripts in this directory help you:

1. Upload environment variables from a local `.env` file to AWS Parameter Store
2. Fetch environment variables from AWS Parameter Store to create a local `.env` file

This approach provides a secure way to manage and distribute environment variables without hardcoding sensitive information in your code.

## Scripts

### 1. `upload-env-to-params.sh`

Uploads environment variables from your local `.env` file to AWS Parameter Store.

**Usage:**
```bash
./scripts/upload-env-to-params.sh [--include-aws-credentials] [--env-file path/to/env]
```

**Options:**
- `--include-aws-credentials`: Include AWS credentials when uploading parameters
- `--env-file`: Specify a different env file (default: `.env`)

**Requirements:**
- AWS CLI installed and configured
- A `.env` file in the root directory
- Appropriate IAM permissions to write to AWS Parameter Store

**Details:**
- Parameters are stored at path `/yok/prod/<VARIABLE_NAME>`
- All values are stored as SecureString type (encrypted)
- AWS credentials are not stored by default unless explicitly included with --include-aws-credentials

### 2. `fetch-params-to-env.sh`

Fetches environment variables from AWS Parameter Store and creates a local `.env` file.

**Usage:**
```bash
./scripts/fetch-params-to-env.sh [--include-aws-credentials] [--output output_file]
```

**Options:**
- `--include-aws-credentials`: Include AWS credentials in the generated .env file
- `--output`: Specify output file path (default: `.env`)

**Requirements:**
- AWS CLI installed and configured
- Appropriate IAM permissions to read from AWS Parameter Store

**Details:**
- Fetches all parameters from path `/yok/prod/`
- Decrypts SecureString values
- Creates a properly formatted .env file
- Sets secure permissions (600) on the output file
- When --include-aws-credentials is used, adds current AWS credentials to the .env file

## Setup Flow

1. **Development Environment**:
   - Create a `.env` file with all variables
   - Run `upload-env-to-params.sh` to store in Parameter Store
   - Parameters are now safely stored in AWS

2. **EC2 Instance**:
   - Instance has IAM role to access Parameter Store
   - On startup, `fetch-params-to-env.sh --include-aws-credentials` is automatically run
   - Docker Compose uses the generated `.env` file with AWS credentials

3. **Updates**:
   - Modify local `.env` file
   - Run `upload-env-to-params.sh` again to update Parameter Store
   - SSH to EC2 and run `fetch-params-to-env.sh --include-aws-credentials` or restart the instance

## AWS Credential Handling

These scripts intelligently handle AWS credentials:

1. First check for credentials in standard AWS CLI locations
2. Then check for environment variables
3. For `upload-env-to-params.sh`: Check `.env` file for credentials
4. For `fetch-params-to-env.sh`: Check common EC2 credential locations
5. When `--include-aws-credentials` is specified, AWS credentials are:
   - Included in the downloaded .env file (for fetch-params-to-env.sh)
   - Uploaded to Parameter Store (for upload-env-to-params.sh, though this is generally not recommended)

## Security Notes

- Environment variables are stored as SecureString in Parameter Store
- The `.env` file permissions are set to 600 (readable only by the owner)
- AWS IAM roles are used to control access to the parameters
- AWS credentials themselves are skipped when uploading environment variables (unless --include-aws-credentials is used)
- Never commit `.env` files to version control
- When using --include-aws-credentials, be extra careful with the .env file as it will contain sensitive credentials

## Troubleshooting

If you encounter issues:

1. **Permission Issues**:
   ```bash
   aws sts get-caller-identity  # Verify AWS identity
   ```

2. **Check Parameters in AWS**:
   ```bash
   aws ssm get-parameters-by-path --path "/yok/prod/" --recursive --with-decryption
   ```

3. **Invalid Environment Variables**:
   ```bash
   # Validate .env file format
   cat .env | grep -v "^#" | grep -v "^$" | grep -v "="
   # Should return no output if valid
   ```

4. **EC2 Credential Issues**:
   ```bash
   # On EC2 instance
   aws configure list  # Check current configuration
   cat /home/ec2-user/.aws/credentials  # Verify credentials file
   ```

5. **Docker Environment Variables**:
   ```bash
   # Check if Docker can see all variables
   docker-compose config | grep AWS
   ``` 