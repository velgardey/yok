output "ec2_public_ip" {
  description = "Public IP address of the EC2 instance"
  value       = module.ec2.public_ip
}

output "ec2_instance_id" {
  description = "ID of the EC2 instance"
  value       = module.ec2.instance_id
}

output "vpc_id" {
  description = "ID of the created VPC"
  value       = module.networking.vpc_id
}

output "ssh_command" {
  description = "SSH command to connect to the EC2 instance"
  value       = "ssh -i '${var.key_name}.pem' ec2-user@${module.ec2.public_ip}"
}

output "website_urls" {
  description = "URLs of the deployed services"
  value = {
    api      = "https://api.${var.domain_name}"
    wildcard = "https://<subdomain>.${var.domain_name}"
  }
}

output "certificate_setup" {
  description = "Instructions for setting up SSL certificates"
  value       = <<EOF
After the instance is provisioned, connect to it using:
ssh -i '${var.key_name}.pem' ec2-user@${module.ec2.public_ip}

Then run the certificate setup script:
sudo /home/ec2-user/setup-certificates.sh

Follow the interactive prompts to set up certificates for:
1. api.${var.domain_name} (HTTP validation)
2. *.${var.domain_name} (DNS validation - requires DNS TXT record creation)
EOF
}
