/* 
 * NOTE: Terraform doesn't allow variables in backend configuration.
 * These values should match your terraform.tfvars settings.
 * If you change values in terraform.tfvars, you'll need to run:
 * terraform init -reconfigure -backend-config="bucket=your-bucket" -backend-config="key=your-key" -backend-config="region=your-region" -backend-config="dynamodb_table=your-table"
 */
terraform {
  backend "s3" {
    bucket         = "yok-terraform-state"
    key            = "yok/terraform.tfstate"
    region         = "ap-south-1"
    encrypt        = true
    dynamodb_table = "yok-terraform-lock"
  }
} 