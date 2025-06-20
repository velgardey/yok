#!/bin/bash
set -e

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "AWS CLI is not installed. Please install it first."
    exit 1
fi

# Parse command line options
INCLUDE_AWS_CREDENTIALS=false
OUTPUT_FILE=".env"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --include-aws-credentials)
      INCLUDE_AWS_CREDENTIALS=true
      shift
      ;;
    --output)
      OUTPUT_FILE="$2"
      shift 2
      ;;
    *)
      OUTPUT_FILE="$1"
      shift
      ;;
  esac
done

# Default values
AWS_REGION=${AWS_REGION:-"ap-south-1"}
PARAMETER_PATH="/yok/prod"

# Verify AWS CLI access
echo "Verifying AWS CLI configuration..."
if ! aws sts get-caller-identity &>/dev/null; then
    echo "Error: AWS CLI is not properly configured or doesn't have necessary permissions."
    echo "Please run 'aws configure' to set up your AWS CLI."
    exit 1
fi

echo "====================================================================================="
echo "Fetching environment variables from AWS Parameter Store for YOK project"
echo "====================================================================================="
echo "Using AWS Region: $AWS_REGION"
echo "Output file: $OUTPUT_FILE"
echo "Parameter path: $PARAMETER_PATH"
echo "Include AWS credentials: $INCLUDE_AWS_CREDENTIALS"
echo "====================================================================================="

# Create or overwrite the output file
echo "# Environment variables fetched from AWS Parameter Store on $(date)" > "$OUTPUT_FILE"
echo "# Path: $PARAMETER_PATH" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Get all parameters using get-parameters-by-path instead of describe-parameters
echo "Retrieving parameters..."
parameters=$(aws ssm get-parameters-by-path \
    --path "$PARAMETER_PATH" \
    --recursive \
    --with-decryption \
    --region "$AWS_REGION" \
    --output json)

# Check if we got any parameters
parameter_count=$(echo "$parameters" | jq -r '.Parameters | length')
if [ "$parameter_count" -eq 0 ]; then
    echo "No parameters found at path $PARAMETER_PATH"
    exit 1
fi

# Extract and process each parameter
echo "$parameters" | jq -c '.Parameters[]' | while read -r param; do
    # Extract the parameter name and value
    param_name=$(echo "$param" | jq -r '.Name')
    param_value=$(echo "$param" | jq -r '.Value')
    
    # Extract the environment variable name from the path
    env_var_name=$(basename "$param_name")
    
    # Remove any trailing newline characters
    param_value=$(echo "$param_value" | tr -d '\n')
    
    # Add to .env file
    echo "$env_var_name=$param_value" >> "$OUTPUT_FILE"
    echo "Added $env_var_name to $OUTPUT_FILE"
done

# Add AWS region and credentials if not already added
region_exists=$(grep -c "^AWS_REGION=" "$OUTPUT_FILE" || true)
if [ "$region_exists" -eq 0 ] && [ -n "$AWS_REGION" ]; then
    echo "Adding AWS_REGION to $OUTPUT_FILE"
    echo "AWS_REGION=$AWS_REGION" >> "$OUTPUT_FILE"
fi

# Add AWS credentials if requested
if [ "$INCLUDE_AWS_CREDENTIALS" = true ]; then
    # Get AWS credentials from current configuration
    echo "Adding AWS credentials to $OUTPUT_FILE"
    
    # Try to get credentials from AWS CLI configuration
    aws_access_key_id=$(aws configure get aws_access_key_id)
    aws_secret_access_key=$(aws configure get aws_secret_access_key)
    
    if [ -n "$aws_access_key_id" ] && [ -n "$aws_secret_access_key" ]; then
        echo "AWS_ACCESS_KEY_ID=$aws_access_key_id" >> "$OUTPUT_FILE"
        echo "AWS_SECRET_ACCESS_KEY=$aws_secret_access_key" >> "$OUTPUT_FILE"
        echo "Added AWS credentials to $OUTPUT_FILE"
    else
        echo "Warning: Could not retrieve AWS credentials from AWS CLI configuration."
        echo "You may need to add them manually to $OUTPUT_FILE."
    fi
fi

# Fix permissions for .env file
chmod 600 "$OUTPUT_FILE"
echo "Set secure permissions on $OUTPUT_FILE"

echo "====================================================================================="
echo "Environment variables have been successfully fetched and saved to $OUTPUT_FILE"
echo "=====================================================================================" 