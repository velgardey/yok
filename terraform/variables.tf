variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "ap-south-1"
}

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "production"
}

variable "project" {
  description = "Project name"
  type        = string
  default     = "yok"
}

variable "key_name" {
  description = "SSH key name in AWS (must exist prior to applying Terraform)"
  type        = string
  validation {
    condition     = length(var.key_name) > 0
    error_message = "The key_name value must not be empty."
  }
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t2.micro"
}

variable "domain_name" {
  description = "Your domain name (e.g., yok.ninja)"
  type        = string
  default     = "yok.ninja"
}

variable "email" {
  description = "Email address for SSL certificate notifications"
  type        = string
}

variable "terraform_state_bucket" {
  description = "S3 bucket for storing Terraform state"
  type        = string
  default     = "yok-terraform-state"
}

variable "terraform_state_key" {
  description = "S3 key for Terraform state"
  type        = string
  default     = "yok/terraform.tfstate"
}

variable "terraform_lock_table" {
  description = "DynamoDB table for Terraform state locking"
  type        = string
  default     = "yok-terraform-lock"
}

variable "aws_access_key_id" {
  description = "AWS Access Key ID"
  type        = string
  sensitive   = true
}

variable "aws_secret_access_key" {
  description = "AWS Secret Access Key"
  type        = string
  sensitive   = true
}
