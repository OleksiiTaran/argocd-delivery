terraform {
    required_providers {
        kubernetes = {
            source = "hashicorp/kubernetes"
            version = "~> 2.0"
        }
    }
}

provider "kubernetes" {
    config_path = "~/.kube/config"
}

resource "kubernetes_namespace" "env" {
    metadata {
        name = var.namespace_name
    }
}

resource "kubernetes_resource_quota" "env_quota" {
    metadata {
        name      = "${var.namespace_name}-quota"
        namespace = kubernetes_namespace.env.metadata[0].name
    }
    spec {
        hard = {
            "limits.cpu"    = var.cpu_limit
            "limits.memory" = var.memory_limit
        }
    }
}

resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  namespace        = "argocd"
  create_namespace = true
  version          = "5.51.6"

  values = [
    <<-EOT
    server:
      extraArgs:
        - --insecure
    EOT
  ]
}