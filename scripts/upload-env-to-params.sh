#!/bin/bash
set -e

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "AWS CLI is not installed. Please install it first."
    exit 1
fi

# Parse command line options
INCLUDE_AWS_CREDENTIALS=false
ENV_FILE=".env"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --include-aws-credentials)
      INCLUDE_AWS_CREDENTIALS=true
      shift
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    *)
      ENV_FILE="$1"
      shift
      ;;
  esac
done

# Source environment file if it exists
if [ ! -f "$ENV_FILE" ]; then
    echo "Error: $ENV_FILE does not exist."
    exit 1
fi

# Extract AWS region from env file or use default
AWS_REGION=$(grep AWS_REGION "$ENV_FILE" | cut -d '=' -f2 || echo "ap-south-1")

# Verify AWS CLI access
echo "Verifying AWS CLI configuration..."
if ! aws sts get-caller-identity &>/dev/null; then
    echo "Error: AWS CLI is not properly configured or doesn't have necessary permissions."
    echo "Please run 'aws configure' to set up your AWS CLI."
    exit 1
fi

echo "====================================================================================="
echo "Uploading environment variables to AWS Parameter Store for YOK project"
echo "====================================================================================="
echo "Using AWS Region: $AWS_REGION"
echo "Using environment file: $ENV_FILE"
echo "Include AWS credentials: $INCLUDE_AWS_CREDENTIALS"
echo "====================================================================================="

# Parse the .env file and upload each variable to Parameter Store
while IFS= read -r line || [ -n "$line" ]; do
    # Skip comments and empty lines
    if [[ "$line" =~ ^# ]] || [[ -z "$line" ]]; then
        continue
    fi

    # Skip AWS credentials unless explicitly included
    if [[ "$INCLUDE_AWS_CREDENTIALS" != "true" ]] && ([[ "$line" =~ ^AWS_ACCESS_KEY_ID= ]] || [[ "$line" =~ ^AWS_SECRET_ACCESS_KEY= ]]); then
        echo "Skipping AWS credential: ${line%%=*} (use --include-aws-credentials to include)"
        continue
    fi

    # Split the line into key and value
    if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
        
        # Remove any surrounding quotes from the value
        value=$(echo "$value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
        
        echo "Uploading parameter: /yok/prod/$key"

        # Write the value to a temporary file to avoid command line parsing issues
        temp_file=$(mktemp)
        echo "$value" > "$temp_file"
        
        aws ssm put-parameter \
            --name "/yok/prod/$key" \
            --value "file://$temp_file" \
            --type "SecureString" \
            --overwrite \
            --region "$AWS_REGION"
            
        # Clean up the temp file
        rm -f "$temp_file"
        
        echo "Parameter /yok/prod/$key uploaded successfully."
    fi
done < "$ENV_FILE"

echo "====================================================================================="
echo "All environment variables have been uploaded to AWS Parameter Store."
echo "=====================================================================================" 