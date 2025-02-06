resource "kubernetes_service" "load_balancer_service" {
  metadata {
    name = var.name
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