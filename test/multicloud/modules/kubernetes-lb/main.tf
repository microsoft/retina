resource "kubernetes_service" "load_balancer_service" {
  metadata {
    name      = var.name
    namespace = var.namespace
  }

  spec {
    type = "LoadBalancer"

    port {
      port        = var.port
      target_port = var.port
      protocol    = "TCP"
    }

    selector = var.label_selector
  }
}