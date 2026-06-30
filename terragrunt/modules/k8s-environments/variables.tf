variable "namespace_name" {
    description = "Name of the Kubernetes namespaces"
    type        = string
}

variable "cluster_name" {
    description = "Name of the Kubernetes cluster"
    type        = string
}

variable "memory_limit" {
    description = "Maximum memory allowed for this namespace"
    type        = string
}

variable "cpu_limit" {
    description = "Maximum CPU allowed for this namespace"
    type        = string
}
