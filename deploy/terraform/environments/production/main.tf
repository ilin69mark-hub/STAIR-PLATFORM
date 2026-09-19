terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }

  backend "s3" {
    bucket  = "stair-platform-terraform"
    key     = "production/terraform.tfstate"
    region  = "us-east-1"
    encrypt = true
    # Требуется перед работой в команде: создать DynamoDB table "stair-platform-tf-lock"
    # (partition key: LockID, type: String) и раскомментировать строку ниже.
    # dynamodb_table = "stair-platform-tf-lock"
  }
}

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "database_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

# VPC
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "stair-platform-${var.environment}"
  cidr = "10.0.0.0/16"

  azs             = ["${var.aws_region}a", "${var.aws_region}b", "${var.aws_region}c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway = true
  single_nat_gateway = var.environment != "production"

  tags = {
    Environment = var.environment
    Project     = "stair-platform"
  }
}

# PostgreSQL
module "postgres" {
  source = "../../modules/postgres"

  identifier     = "stair-platform-${var.environment}"
  instance_class = "db.r6g.large"
  db_name        = "stair_platform"
  username       = "stair"
  password       = var.database_password

  vpc_id              = module.vpc.vpc_id
  subnet_ids          = module.vpc.private_subnets
  allowed_cidr_blocks = module.vpc.private_subnets_cidr_blocks

  tags = {
    Environment = var.environment
    Project     = "stair-platform"
  }
}

# Kubernetes (EKS)
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "19.0.0"

  cluster_name    = "stair-platform-${var.environment}"
  cluster_version = "1.28"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  eks_managed_node_groups = {
    general = {
      desired_size = 2
      min_size     = 1
      max_size     = 10

      instance_types = ["t3.medium"]
    }
  }

  tags = {
    Environment = var.environment
    Project     = "stair-platform"
  }
}

# Kubernetes resources (S2-2): terraform = ТОЛЬКО infra (namespace).
# Приложение (чарт deploy/helm) разворачивает CD (ci/cd.yml); image.tag и
# прочие values задаются там, а не в terraform — один владелец ресурсов.
module "kubernetes" {
  source = "../../modules/kubernetes"

  cluster_name = module.eks.cluster_name
  namespace    = "stair-platform"
}

output "vpc_id" {
  value = module.vpc.vpc_id
}

output "database_endpoint" {
  value = module.postgres.endpoint
}

output "eks_cluster_name" {
  value = module.eks.cluster_name
}

output "kubernetes_namespace" {
  value = module.kubernetes.namespace
}
