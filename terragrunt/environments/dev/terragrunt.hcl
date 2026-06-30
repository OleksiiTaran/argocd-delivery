terraform {
    source = "../../modules/k8s-environments"
}

inputs = {
    namespace_name = "myapp-dev"
    cpu_limit      = "1" # 1 ядро CPU для Dev
    memory_limit   = "1Gi"  # 1 Гігабайт RAM для Dev
}