terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "identifier" {
  description = "Identifier for the RDS instance"
  type        = string
}

variable "engine_version" {
  description = "PostgreSQL engine version"
  type        = string
  default     = "16.1"
}

variable "instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "allocated_storage" {
  description = "Allocated storage in GB"
  type        = number
  default     = 20
}

variable "db_name" {
  description = "Database name"
  type        = string
  default     = "stair_platform"
}

variable "username" {
  description = "Database username"
  type        = string
}

variable "password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs for the DB subnet group"
  type        = list(string)
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to connect"
  type        = list(string)
  default     = []
}

variable "backup_retention_period" {
  description = "Backup retention in days"
  type        = number
  default     = 7
}

variable "deletion_protection" {
  description = "Prevent accidental cluster deletion"
  type        = bool
  default     = true
}

variable "preferred_backup_window" {
  description = "Daily backup window (UTC)"
  type        = string
  default     = "02:00-03:00"
}

variable "preferred_maintenance_window" {
  description = "Weekly maintenance window (UTC)"
  type        = string
  default     = "sun:04:00-sun:05:00"
}

variable "parameter_group_family" {
  description = "DB parameter group family (match engine_version)"
  type        = string
  default     = "aurora-postgresql16"
}

variable "tags" {
  description = "Tags for the RDS instance"
  type        = map(string)
  default     = {}
}

resource "aws_security_group" "postgres" {
  name_prefix = "${var.identifier}-postgres-"
  vpc_id      = var.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    cidr_blocks     = var.allowed_cidr_blocks
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.identifier}-postgres"
  })
}

resource "aws_db_subnet_group" "postgres" {
  name       = "${var.identifier}-postgres"
  subnet_ids = var.subnet_ids

  tags = var.tags
}

# Параметры кластера: принудительный SSL (rds.force_ssl) для соответствия
# sslmode=require на клиенте (P0-4). Family подбирается под engine_version.
resource "aws_rds_cluster_parameter_group" "postgres" {
  name        = "${var.identifier}-pg-secure"
  family      = var.parameter_group_family
  description = "STAIR Aurora PostgreSQL: force SSL"

  parameter {
    name         = "rds.force_ssl"
    value        = "1"
    apply_method = "pending-reboot"
  }

  tags = var.tags
}

resource "aws_rds_cluster" "postgres" {
  cluster_identifier              = var.identifier
  engine                          = "aurora-postgresql"
  engine_version                  = var.engine_version
  database_name                   = var.db_name
  master_username                 = var.username
  master_password                 = var.password
  vpc_security_group_ids          = [aws_security_group.postgres.id]
  db_subnet_group_name            = aws_db_subnet_group.postgres.name
  db_cluster_parameter_group_name = aws_rds_cluster_parameter_group.postgres.name
  storage_encrypted               = true

  # P0-5: защита от случайного destroy — финальный снапшот обязателен,
  # удаление кластера только снятием защиты, бэкап ≥ 7 дней.
  skip_final_snapshot        = false
  final_snapshot_identifier  = "${var.identifier}-final"
  deletion_protection       = var.deletion_protection
  copy_tags_to_snapshot     = true
  backup_retention_period   = var.backup_retention_period
  preferred_backup_window   = var.preferred_backup_window
  preferred_maintenance_window = var.preferred_maintenance_window

  tags = var.tags
}

resource "aws_rds_cluster_instance" "postgres" {
  count              = 1
  identifier         = "${var.identifier}-${count.index}"
  cluster_identifier = aws_rds_cluster.postgres.id
  instance_class     = var.instance_class
  engine             = aws_rds_cluster.postgres.engine
  engine_version     = aws_rds_cluster.postgres.engine_version

  tags = var.tags
}

output "endpoint" {
  value = aws_rds_cluster.postgres.endpoint
}

output "port" {
  value = aws_rds_cluster.postgres.port
}

output "database_name" {
  value = aws_rds_cluster.postgres.database_name
}
