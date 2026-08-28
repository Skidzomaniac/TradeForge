terraform {
  backend "s3" {
    bucket         = "trade-eval-terraform-state"
    key            = "production/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "trade-eval-terraform-locks"
  }
}
