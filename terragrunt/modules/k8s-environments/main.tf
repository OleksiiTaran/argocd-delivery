terraform {
  backend "local" {
    path = "terraform.tfstate"
  }

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    kind = {
      source  = "tehcyx/kind"
      version = "~> 0.5.0"
    }
  }
}

provider "kind" {}

resource "kind_cluster" "cluster" {
  name           = "local-prod"
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
    }
  }
}

resource "null_resource" "wait_for_cluster" {
  depends_on = [kind_cluster.cluster]
  
  provisioner "local-exec" {
    command = "until kubectl cluster-info --kubeconfig ~/.kube/config; do echo 'Waiting for K8s API...'; sleep 5; done"
  }
}

provider "kubernetes" {
  host                   = kind_cluster.cluster.endpoint
  client_certificate     = kind_cluster.cluster.client_certificate
  client_key             = kind_cluster.cluster.client_key
  cluster_ca_certificate = kind_cluster.cluster.cluster_ca_certificate
}

resource "kubernetes_namespace" "env" {
  metadata {
    name = var.namespace_name
  }
  depends_on = [null_resource.wait_for_cluster]
}

resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  namespace        = "argocd"
  create_namespace = true
  version          = "5.51.6"

  depends_on = [
    null_resource.wait_for_cluster,
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