output "ip" {
  value = element(kubernetes_service.load_balancer_service.status[0].load_balancer[0].ingress, 0).ip
}
