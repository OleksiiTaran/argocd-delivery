terraform {
    required_providers {
        kubernetes = {
            source = "hashicorp/kubernetes"
            version = "~> 2.0"
        }
        kind = {
            source = "tehcyx/kind"
            version = "~> 0.5.0"
        }
    }
}

provider kind {}

resource "kind_cluster" "cluster" {
    name           = "local-sandbox"
    wait_for_ready = true
    kind_config {
        kind        = "Cluster"
        api_version  = "kind.x-k8s.io/v1alpha4"
        node {
            role = "control-plane"
      
            extra_port_mappings {
                container_port = 80
                host_port      = 8080
                protocol       = "TCP"
            }
            extra_port_mappings {
                container_port = 443
                host_port      = 8443
                protocol       = "TCP"
            } 
        }
    }
}

resource "time_sleep" "wait_for_kind" {
  depends_on = [kind_cluster.cluster]
  create_duration = "10s"
}

provider "kubernetes" {
    config_path    = "~/.kube/config"
}

data "kubernetes_api_versions" "health_check" {
  depends_on = [time_sleep.wait_for_kind]
}

resource "kubernetes_namespace" "env" {
    metadata {
        name = var.namespace_name
    }
    depends_on = [time_sleep.wait_for_kind]
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

  depends_on = [
    time_sleep.wait_for_kind,
    kubernetes_namespace.env
  ]

  values = [
    <<-EOT
    server:
      extraArgs:
        - --insecure
    EOT
  ]
}