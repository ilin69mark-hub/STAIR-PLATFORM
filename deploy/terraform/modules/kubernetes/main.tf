terraform {
  required_version = ">= 1.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.0"
    }
  }
}

variable "cluster_name" {
  description = "Kubernetes cluster name"
  type        = string
}

variable "namespace" {
  description = "Namespace for the application"
  type        = string
  default     = "stair-platform"
}

variable "release_name" {
  description = "Helm release name"
  type        = string
  default     = "stair-platform"
}

variable "chart_path" {
  description = "Path to the Helm chart"
  type        = string
  default     = "../helm/stair-platform"
}

variable "values" {
  description = "Values to pass to the Helm chart"
  type        = map(any)
  default     = {}
}

variable "database_url" {
  description = "Database connection URL"
  type        = string
  sensitive   = true
}

variable "replica_count" {
  description = "Number of replicas"
  type        = number
  default     = 2
}

resource "kubernetes_namespace" "stair" {
  metadata {
    name = var.namespace

    labels = {
      app = "stair-platform"
    }
  }
}

resource "helm_release" "stair_platform" {
  name       = var.release_name
  repository = ""
  chart      = var.chart_path
  namespace  = kubernetes_namespace.stair.metadata[0].name
  version    = "0.1.0"

  set {
    name  = "replicaCount"
    value = var.replica_count
  }

  set_sensitive {
    name  = "database.url"
    value = var.database_url
  }

  dynamic "set" {
    for_each = var.values
    content {
      name  = set.key
      value = set.value
    }
  }

  depends_on = [
    kubernetes_namespace.stair
  ]
}

output "namespace" {
  value = kubernetes_namespace.stair.metadata[0].name
}

output "release_name" {
  value = helm_release.stair_platform.name
}
