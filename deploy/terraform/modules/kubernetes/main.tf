terraform {
  required_version = ">= 1.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
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

# S2-2: единственный владелец инфраструктуры.
# terraform владеет ТОЛЬКО infra-ресурсами (кластер, сеть, БД, namespace).
# Приложение (deploy + helm-чарт) разворачивает CD (ci/cd.yml) — никакого
# helm_release / chart в terraform. Изменения ресурсов приложения ИДУТ через
# deploy/helm/* и CI-гейт check-infra-ownership.sh.
resource "kubernetes_namespace" "stair" {
  metadata {
    name = var.namespace

    labels = {
      app = "stair-platform"
    }
  }
}

output "namespace" {
  value = kubernetes_namespace.stair.metadata[0].name
}

output "cluster_name" {
  value = var.cluster_name
}