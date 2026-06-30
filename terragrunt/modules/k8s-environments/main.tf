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
    kind_config = <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 8080
    protocol: TCP
  - containerPort: 443
    hostPort: 8443
    protocol: TCP
EOF
}

provider "kubernetes" {
    config_path    = "~/.kube/config"
}

resource "kubernetes_namespace" "env" {
    metadata {
        name = var.namespace_name
    }
    depends_on     = [kind_cluster.cluster]
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