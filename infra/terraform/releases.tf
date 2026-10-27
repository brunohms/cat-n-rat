locals {
  charts_dir = "${path.module}/../charts"
}

resource "random_password" "postgres" {
  length  = 32
  special = false
}

resource "helm_release" "postgres" {
  name             = "postgres"
  chart            = "${local.charts_dir}/postgres"
  namespace        = var.namespace
  create_namespace = true
  wait             = true

  set_sensitive = [
    { name = "password", value = random_password.postgres.result },
  ]
}

# --- Monitoring: ClickStack (ClickHouse + HyperDX + OpenTelemetry Collector) ---
# The otel-collector agent on the node gathers pod logs, kubelet metrics and the OTLP data of the apps and
# forwards everything to the ClickStack collector, which writes it to ClickHouse. HyperDX is the UI.

locals {
  clickstack_repository = "https://clickhouse.github.io/ClickStack-helm-charts"
  otlp_endpoint         = "http://${helm_release.otel_collector.name}:4318"
  clickstack_values = templatefile("${path.module}/clickstack-values.yaml.tftpl", {
    hyperdx_node_port = var.hyperdx_node_port
    telemetry_ttl     = var.telemetry_ttl
  })
}

resource "random_password" "clickstack" {
  for_each = toset(["clickhouse", "clickhouse_app", "mongodb", "otlp_token"])

  length  = 32
  special = false
}

# MongoDB and ClickHouse operators (and their CRDs) used by the clickstack chart.
# The MongoDB operator only watches its own namespace, so it is installed next to ClickStack.
resource "helm_release" "clickstack_operators" {
  name             = "clickstack-operators"
  repository       = local.clickstack_repository
  chart            = "clickstack-operators"
  version          = var.clickstack_operators_chart_version
  namespace        = var.namespace
  create_namespace = true
  wait             = true
}

resource "helm_release" "clickstack" {
  name       = "clickstack"
  repository = local.clickstack_repository
  chart      = "clickstack"
  version    = var.clickstack_chart_version
  namespace  = var.namespace
  # ClickHouse and MongoDB are custom resources that Helm does not wait for; the pods retry until they are up
  wait    = true
  timeout = 600

  values = [local.clickstack_values]

  set_sensitive = [
    { name = "hyperdx.secrets.CLICKHOUSE_PASSWORD", value = random_password.clickstack["clickhouse"].result },
    { name = "hyperdx.secrets.CLICKHOUSE_APP_PASSWORD", value = random_password.clickstack["clickhouse_app"].result },
    { name = "hyperdx.secrets.MONGODB_PASSWORD", value = random_password.clickstack["mongodb"].result },
    { name = "hyperdx.secrets.OTLP_AUTH_TOKEN", value = random_password.clickstack["otlp_token"].result },
  ]

  depends_on = [helm_release.clickstack_operators]
}

resource "helm_release" "otel_collector" {
  name      = "otel-collector"
  chart     = "${local.charts_dir}/otel-collector"
  namespace = var.namespace
  wait      = true

  set = [
    { name = "clickstack.endpoint", value = "http://${helm_release.clickstack.name}-otel-collector:4318" },
  ]
}

# --- Application ---

resource "helm_release" "backend" {
  name      = "backend"
  chart     = "${local.charts_dir}/backend"
  namespace = var.namespace
  wait      = true

  set = [
    { name = "image.tag", value = local.image_tags.backend },
    { name = "service.type", value = "NodePort" },
    { name = "service.nodePort", value = var.backend_node_port },
    { name = "database.host", value = helm_release.postgres.name },
    { name = "database.passwordSecret.name", value = helm_release.postgres.name },
    { name = "generatorUrl", value = "http://generator:8000" },
    { name = "otel.endpoint", value = local.otlp_endpoint },
  ]

  depends_on = [terraform_data.image]
}

resource "helm_release" "generator" {
  name      = "generator"
  chart     = "${local.charts_dir}/generator"
  namespace = var.namespace
  wait      = true

  set = [
    { name = "image.tag", value = local.image_tags.generator },
    { name = "service.type", value = "NodePort" },
    { name = "service.nodePort", value = var.generator_node_port },
    { name = "config.HTTP_TARGET_URL", value = "http://${helm_release.backend.name}:8000/events" },
    { name = "config.EVENTS_PER_SECOND", value = tostring(var.events_per_second), type = "string" },
    { name = "config.OTEL_EXPORTER_OTLP_ENDPOINT", value = local.otlp_endpoint },
  ]

  depends_on = [terraform_data.image]
}

resource "helm_release" "frontend" {
  name      = "frontend"
  chart     = "${local.charts_dir}/frontend"
  namespace = var.namespace
  wait      = true

  set = [
    { name = "image.tag", value = local.image_tags.frontend },
    { name = "service.type", value = "NodePort" },
    { name = "service.nodePort", value = var.frontend_node_port },
    { name = "backendUrl", value = "http://${helm_release.backend.name}:8000" },
  ]

  depends_on = [terraform_data.image]
}
