output "ip" {
  value = element(kubernetes_service.load_balancer_service.status[0].load_balancer[0].ingress, 0).ip
}

output "hostname" {
  value = element(kubernetes_service.load_balancer_service.status[0].load_balancer[0].ingress, 0).hostname
}
