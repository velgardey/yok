provider "aws" {
  region = var.aws_region
}

module "networking" {
  source      = "./modules/networking"
  aws_region  = var.aws_region
  environment = var.environment
  project     = var.project
}

module "ec2" {
  source               = "./modules/ec2"
  vpc_id               = module.networking.vpc_id
  subnet_id            = module.networking.public_subnet_id
  security_group_id    = module.networking.security_group_id
  key_name             = var.key_name
  instance_type        = var.instance_type
  environment          = var.environment
  project              = var.project
  domain_name          = var.domain_name
  email                = var.email
  aws_region           = var.aws_region
  aws_access_key_id    = var.aws_access_key_id
  aws_secret_access_key = var.aws_secret_access_key
}
