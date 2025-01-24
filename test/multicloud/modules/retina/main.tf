resource "helm_release" "retina" {
  name       = "retina"
  repository = "oci://ghcr.io/microsoft/retina/charts"
  chart      = "retina"
  version    = var.retina_version
  namespace  = "kube-system"

  set {
    name  = "image.tag"
    value = var.retina_version
  }

  set {
    name  = "operator.tag"
    value = var.retina_version
  }

  set {
    name  = "logLevel"
    value = "info"
  }

  # set {
  #     name  = "enabledPlugin_linux"
  #     value = "[\"dropreason\",\"packetforward\",\"linuxutil\",\"dns\"]"
  # }
}
