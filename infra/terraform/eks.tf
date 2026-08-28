resource "aws_iam_role" "eks_cluster" {
  name = "trade-eval-eks-cluster"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
    }]
  })
}

resource "aws_eks_cluster" "main" {
  name     = var.eks_cluster_name
  role_arn = aws_iam_role.eks_cluster.arn
  version  = "1.30"
  vpc_config {
    subnet_ids = concat(aws_subnet.private[*].id, aws_subnet.public[*].id)
  }
}

resource "aws_iam_role" "eks_nodes" {
  name = "trade-eval-eks-nodes"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_eks_node_group" "system" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "system"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = aws_subnet.private[*].id
  instance_types  = ["t3.medium"]
  scaling_config {
    desired_size = var.desired_node_count
    min_size     = 2
    max_size     = 4
  }
}

resource "aws_eks_node_group" "bot_fleet" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "bot-fleet"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = aws_subnet.private[*].id
  instance_types  = ["c6i.2xlarge"]
  scaling_config {
    desired_size = 0
    min_size     = 0
    max_size     = 10
  }
  taint {
    key    = "dedicated"
    value  = "bot-fleet"
    effect = "NO_SCHEDULE"
  }
}
