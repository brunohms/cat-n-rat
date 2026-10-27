output "kubeconfig_path" {
  description = "Kubeconfig of the cluster (also merged into ~/.kube/config as context kind-<cluster_name>)"
  value       = kind_cluster.this.kubeconfig_path
}

output "kube_context" {
  description = "kubectl context of the cluster"
  value       = "kind-${kind_cluster.this.name}"
}

output "frontend_url" {
  description = "URL of the frontend on the host"
  value       = "http://localhost:${var.frontend_node_port}"
}

output "backend_url" {
  description = "URL of the backend on the host"
  value       = "http://localhost:${var.backend_node_port}"
}

output "generator_url" {
  description = "URL of the generator on the host"
  value       = "http://localhost:${var.generator_node_port}"
}

output "hyperdx_url" {
  description = "URL of the HyperDX UI of ClickStack on the host"
  value       = "http://localhost:${var.hyperdx_node_port}"
}

output "image_tags" {
  description = "Image tags deployed to the cluster"
  value       = local.image_tags
}
