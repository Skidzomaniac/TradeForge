output "eks_cluster_endpoint" {
  value = aws_eks_cluster.main.endpoint
}

output "rds_timescale_endpoint" {
  value = aws_db_instance.timescale.endpoint
}

output "rds_orchestrator_endpoint" {
  value = aws_db_instance.orchestrator.endpoint
}

output "redis_endpoint" {
  value = aws_elasticache_cluster.redis.cache_nodes[0].address
}

output "msk_bootstrap_brokers" {
  value = aws_msk_cluster.kafka.bootstrap_brokers
}

output "s3_bucket_name" {
  value = aws_s3_bucket.submissions.bucket
}
