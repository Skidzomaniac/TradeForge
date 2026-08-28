resource "aws_db_instance" "timescale" {
  identifier           = "trade-eval-timescale"
  engine               = "postgres"
  engine_version       = "16"
  instance_class       = "db.t3.medium"
  allocated_storage    = 100
  storage_type         = "gp3"
  storage_encrypted    = true
  db_name              = "tradeeval"
  username             = "postgres"
  password             = "CHANGE_ME_VIA_SECRETS"
  skip_final_snapshot  = true
  publicly_accessible  = false
}

resource "aws_db_instance" "orchestrator" {
  identifier          = "trade-eval-orchestrator"
  engine              = "postgres"
  engine_version      = "16"
  instance_class      = "db.t3.micro"
  allocated_storage   = 20
  storage_encrypted   = true
  db_name             = "orchestrator"
  username            = "postgres"
  password            = "CHANGE_ME_VIA_SECRETS"
  skip_final_snapshot = true
  publicly_accessible = false
}

resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "trade-eval-redis"
  engine               = "redis"
  node_type            = "cache.t3.micro"
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
}

resource "aws_msk_cluster" "kafka" {
  cluster_name           = "trade-eval-kafka"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 3
  broker_node_group_info {
    instance_type   = "kafka.t3.small"
    client_subnets  = concat(aws_subnet.private[*].id, [aws_subnet.public[0].id])
    storage_info {
      ebs_storage_info { volume_size = 50 }
    }
  }
}
