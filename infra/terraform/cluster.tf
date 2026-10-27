resource "kind_cluster" "this" {
  name           = var.cluster_name
  node_image     = var.node_image
  wait_for_ready = true

  kind_config {
    kind        = "Cluster"
    api_version = "kind.x-k8s.io/v1alpha4"

    node {
      role = "control-plane"

      # Exposes the NodePort services on localhost
      extra_port_mappings {
        container_port = var.frontend_node_port
        host_port      = var.frontend_node_port
        listen_address = "127.0.0.1"
      }

      extra_port_mappings {
        container_port = var.backend_node_port
        host_port      = var.backend_node_port
        listen_address = "127.0.0.1"
      }

      extra_port_mappings {
        container_port = var.generator_node_port
        host_port      = var.generator_node_port
        listen_address = "127.0.0.1"
      }

      extra_port_mappings {
        container_port = var.hyperdx_node_port
        host_port      = var.hyperdx_node_port
        listen_address = "127.0.0.1"
      }
    }
  }
}
