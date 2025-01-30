resource "helm_release" "retina" {
  name       = "retina"
  repository = "oci://ghcr.io/microsoft/retina/charts"
  chart      = "retina"
  version    = var.retina_version
  namespace  = "kube-system"

  dynamic "set" {
    for_each = var.values
    content {
      name  = set.value.name
      value = set.value.value
    }
  }
}
