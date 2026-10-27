variable "cluster_name" {
  description = "Name of the kind cluster"
  type        = string
  default     = "catnrat"
}

variable "node_image" {
  description = "kind node image (Kubernetes version); null uses the default of the kind provider"
  type        = string
  default     = null
}

variable "namespace" {
  description = "Namespace where the releases are installed"
  type        = string
  default     = "catnrat"
}

variable "frontend_node_port" {
  description = "Port on the host (and NodePort in the cluster) that exposes the frontend"
  type        = number
  default     = 30080
}

variable "backend_node_port" {
  description = "Port on the host (and NodePort in the cluster) that exposes the backend"
  type        = number
  default     = 30001
}

variable "generator_node_port" {
  description = "Port on the host (and NodePort in the cluster) that exposes the generator"
  type        = number
  default     = 30000
}

variable "hyperdx_node_port" {
  description = "Port on the host (and NodePort in the cluster) that exposes the HyperDX UI of ClickStack"
  type        = number
  default     = 30030
}

variable "clickstack_chart_version" {
  description = "Version of the clickstack Helm chart"
  type        = string
  default     = "3.4.0"
}

variable "clickstack_operators_chart_version" {
  description = "Version of the clickstack-operators Helm chart (MongoDB and ClickHouse operators)"
  type        = string
  default     = "1.1.0"
}

variable "telemetry_ttl" {
  description = "How long ClickHouse keeps the logs, metrics and traces (Go duration, e.g. 72h)"
  type        = string
  default     = "72h"
}

variable "events_per_second" {
  description = "Rate of the continuous event stream sent by the generator to the backend"
  type        = number
  default     = 1
}
