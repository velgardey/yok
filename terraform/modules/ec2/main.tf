data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-kernel-6.1-x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_iam_role" "ec2_ssm_role" {
  name = "${var.project}-ec2-ssm-role-${var.environment}"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
  
  tags = {
    Name        = "${var.project}-ec2-ssm-role-${var.environment}"
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_iam_policy" "ssm_parameter_policy" {
  name        = "${var.project}-ssm-parameter-policy-${var.environment}"
  description = "Allow EC2 instance to access SSM Parameter Store parameters"
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath",
          "ssm:PutParameter",
          "ssm:DescribeParameters",
        ]
        Effect = "Allow"
        Resource = "arn:aws:ssm:*:*:parameter/yok/*"
      }
    ]
  })
}

# Additional policy for EC2 to access other AWS services
resource "aws_iam_policy" "additional_services_policy" {
  name        = "${var.project}-additional-services-policy-${var.environment}"
  description = "Allow EC2 instance to access additional AWS services"
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "s3:ListBucket",
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "ecs:ListClusters",
          "ecs:ListTaskDefinitions",
          "ecs:RunTask",
          "ecs:StopTask",
          "ecs:DescribeTasks"
        ]
        Effect = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ssm_policy_attachment" {
  role       = aws_iam_role.ec2_ssm_role.name
  policy_arn = aws_iam_policy.ssm_parameter_policy.arn
}

resource "aws_iam_role_policy_attachment" "additional_services_attachment" {
  role       = aws_iam_role.ec2_ssm_role.name
  policy_arn = aws_iam_policy.additional_services_policy.arn
}

# Attach AWS managed SSM policy for full SSM functionality
resource "aws_iam_role_policy_attachment" "ssm_managed_policy" {
  role       = aws_iam_role.ec2_ssm_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ec2_ssm_profile" {
  name = "${var.project}-ec2-ssm-profile-${var.environment}"
  role = aws_iam_role.ec2_ssm_role.name
}

resource "aws_instance" "web_server" {
  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  vpc_security_group_ids = [var.security_group_id]
  subnet_id              = var.subnet_id
  iam_instance_profile   = aws_iam_instance_profile.ec2_ssm_profile.name

  root_block_device {
    volume_size = 30
    volume_type = "gp3"
  }
  
  user_data = <<-EOF
#!/bin/bash
# Update system packages
dnf update -y

# Install Docker
dnf install -y docker
systemctl start docker
systemctl enable docker
usermod -a -G docker ec2-user

# Install Docker Compose
curl -L https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m) -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Install Git
dnf install -y git

# Install Nginx
dnf install -y nginx
systemctl start nginx
systemctl enable nginx

# Install Certbot and plugins (using package manager instead of pip)
dnf install -y certbot python3-certbot-nginx

# Install AWS CLI if not already installed
dnf install -y awscli jq

# Create app directory
mkdir -p /home/ec2-user/yok-app
chown -R ec2-user:ec2-user /home/ec2-user/yok-app

# Clone repository (Replace with your actual repo URL)
su - ec2-user -c "cd /home/ec2-user/yok-app && git clone https://github.com/velgardey/yok.git ."

# Make scripts executable
chmod +x /home/ec2-user/yok-app/scripts/*.sh

# Configure AWS CLI region
export AWS_REGION="${var.aws_region}"
mkdir -p /home/ec2-user/.aws
cat > /home/ec2-user/.aws/config << AWSCONFIG
[default]
region = ${var.aws_region}
output = json
AWSCONFIG

chown -R ec2-user:ec2-user /home/ec2-user/.aws
chmod 700 /home/ec2-user/.aws
chmod 600 /home/ec2-user/.aws/config

# Also configure AWS CLI for root user
mkdir -p /root/.aws
cp /home/ec2-user/.aws/config /root/.aws/
chmod 700 /root/.aws
chmod 600 /root/.aws/config

# Fetch environment variables from Parameter Store
echo "Fetching environment variables from Parameter Store..."
su - ec2-user -c "cd /home/ec2-user/yok-app && AWS_REGION=${var.aws_region} sudo -E ./scripts/fetch-params-to-env.sh --include-aws-credentials"

# Fix permissions for .env file
chown ec2-user:ec2-user /home/ec2-user/yok-app/.env
chmod 600 /home/ec2-user/yok-app/.env

# Set up Docker Compose with the environment variables
echo "Starting Docker services..."
su - ec2-user -c "cd /home/ec2-user/yok-app && sudo docker-compose up -d"

# Set up Nginx configurations
cat > /etc/nginx/conf.d/api.conf << 'CONFFILE'
server {
    listen 80;
    listen [::]:80;
    server_name api.${var.domain_name};

    location / {
        proxy_pass http://localhost:9000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Original-URI $request_uri;
        
        # Support all HTTP methods
        proxy_method $request_method;
        proxy_pass_request_headers on;
        proxy_pass_request_body on;
    }
}
CONFFILE

cat > /etc/nginx/conf.d/subdomain.conf << 'CONFFILE'
server {
    listen 80;
    listen [::]:80;
    server_name *.${var.domain_name};
    
    # Exclude api subdomain
    if ($host = api.${var.domain_name}) {
        return 404;
    }

    location / {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
CONFFILE

# Create certificate setup script
cat > /home/ec2-user/setup-certificates.sh << 'SCRIPTFILE'
#!/bin/bash

# Define variables
DOMAIN="${var.domain_name}"
EMAIL="${var.email}"

# Display header
echo "========================================================="
echo "SSL Certificate Setup for $DOMAIN"
echo "========================================================="
echo ""
echo "This script will help you set up SSL certificates for:"
echo "1. api.$DOMAIN (HTTP validation)"
echo "2. *.$DOMAIN (DNS validation)"
echo ""

# Function to set up API certificate
setup_api_cert() {
  echo "Setting up certificate for api.$DOMAIN..."
  sudo certbot --nginx -d api.$DOMAIN --non-interactive --agree-tos --email $EMAIL --redirect
  
  if [ $? -eq 0 ]; then
    echo "✅ Successfully obtained certificate for api.$DOMAIN"
  else
    echo "❌ Failed to obtain certificate for api.$DOMAIN"
    return 1
  fi
}

# Function to set up wildcard certificate
setup_wildcard_cert() {
  echo ""
  echo "========================================================="
  echo "Setting up wildcard certificate for *.$DOMAIN"
  echo "========================================================="
  echo ""
  echo "⚠️  IMPORTANT: This requires DNS validation ⚠️"
  echo ""
  echo "You will need to:"
  echo "1. Create TXT records in your DNS settings"
  echo "2. Wait for DNS propagation (can take 5-30 minutes)"
  echo ""
  echo "Press Enter to continue or Ctrl+C to cancel"
  read
  
  sudo certbot certonly --manual --preferred-challenges dns \
    -d "*.$DOMAIN" \
    -d "$DOMAIN" \
    --email $EMAIL \
    --agree-tos
  
  if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Successfully obtained wildcard certificate!"
    echo ""
    echo "Now applying the certificate to Nginx..."
    sudo certbot --nginx -d "*.$DOMAIN" -d "$DOMAIN" --expand
    
    if [ $? -eq 0 ]; then
      echo "✅ Successfully configured Nginx with the wildcard certificate"
    else
      echo "❌ Failed to configure Nginx with the wildcard certificate"
      return 1
    fi
  else
    echo "❌ Failed to obtain wildcard certificate"
    return 1
  fi
}

# Main execution
echo "Choose an option:"
echo "1) Set up API certificate only (api.$DOMAIN)"
echo "2) Set up wildcard certificate only (*.$DOMAIN)"
echo "3) Set up both certificates"
echo "q) Quit"
echo ""
read -p "Enter your choice (1-3, q): " choice

case $choice in
  1)
    setup_api_cert
    ;;
  2)
    setup_wildcard_cert
    ;;
  3)
    setup_api_cert && setup_wildcard_cert
    ;;
  q|Q)
    echo "Exiting without changes"
    exit 0
    ;;
  *)
    echo "Invalid choice"
    exit 1
    ;;
esac

echo ""
echo "Certificate setup process completed."
echo "You can check certificate status with: sudo certbot certificates"
echo ""
echo "Certificates will auto-renew via the certbot systemd timer."
echo "You can check its status with: sudo systemctl status certbot.timer"
SCRIPTFILE

chmod +x /home/ec2-user/setup-certificates.sh
chown ec2-user:ec2-user /home/ec2-user/setup-certificates.sh

# Reload Nginx to apply configurations
systemctl reload nginx

# Create a startup script for Docker services
cat > /home/ec2-user/start-services.sh << 'STARTSCRIPT'
#!/bin/bash
echo "===== Starting YOK services ====="
cd /home/ec2-user/yok-app

# Check if environment file exists, if not fetch from Parameter Store
if [ ! -f .env ]; then
  echo "Environment file not found, fetching from Parameter Store..."
  sudo -E ./scripts/fetch-params-to-env.sh --include-aws-credentials
  sudo chown ec2-user:ec2-user .env
  sudo chmod 600 .env
fi

echo "Starting Docker services..."
sudo docker-compose up -d

echo "Services started. Check status with: docker-compose ps"
STARTSCRIPT

chmod +x /home/ec2-user/start-services.sh
chown ec2-user:ec2-user /home/ec2-user/start-services.sh

echo "Setup complete. Docker services should now be running."
EOF

  tags = {
    Name        = "${var.project}-server-${var.environment}"
    Environment = var.environment
    Project     = var.project
  }
}

resource "aws_eip" "web_server" {
  instance = aws_instance.web_server.id
  domain   = "vpc"
  
  tags = {
    Name        = "${var.project}-eip-${var.environment}"
    Environment = var.environment
    Project     = var.project
  }
}
