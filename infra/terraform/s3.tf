resource "aws_s3_bucket" "submissions" {
  bucket = "trade-eval-submissions-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_public_access_block" "submissions" {
  bucket                  = aws_s3_bucket.submissions.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "submissions" {
  bucket = aws_s3_bucket.submissions.id
  rule {
    id     = "expire-old"
    status = "Enabled"
    expiration { days = 7 }
  }
}
