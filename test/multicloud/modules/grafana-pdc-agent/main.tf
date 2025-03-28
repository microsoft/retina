resource "kubernetes_deployment" "grafana_pdc_agent" {
  metadata {
    name = "grafana-pdc-agent"
    labels = {
      app  = "grafana-pdc-agent"
      name = "grafana-pdc-agent"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        name = "grafana-pdc-agent"
      }
    }

    strategy {
      type = "RollingUpdate"
      rolling_update {
        max_surge       = 1
        max_unavailable = 0
      }
    }

    template {
      metadata {
        labels = {
          name = "grafana-pdc-agent"
        }
      }

      spec {
        security_context {
          run_as_user  = 30000
          run_as_group = 30000
          fs_group     = 30000
        }

        container {
          name              = "grafana-pdc-agent"
          image             = "grafana/pdc-agent:latest"
          image_pull_policy = "Always"

          args = [
            "-cluster",
            "$(CLUSTER)",
            "-token",
            "$(TOKEN)",
            "-gcloud-hosted-grafana-id",
            "$(HOSTED_GRAFANA_ID)"
          ]

          env {
            name = "TOKEN"
            value_from {
              secret_key_ref {
                name = "grafana-pdc-agent"
                key  = "token"
              }
            }
          }

          env {
            name = "CLUSTER"
            value_from {
              secret_key_ref {
                name = "grafana-pdc-agent"
                key  = "cluster"
              }
            }
          }

          env {
            name = "HOSTED_GRAFANA_ID"
            value_from {
              secret_key_ref {
                name = "grafana-pdc-agent"
                key  = "hosted-grafana-id"
              }
            }
          }

          resources {
            limits = {
              memory = "1Gi"
            }
            requests = {
              cpu    = "1"
              memory = "1Gi"
            }
          }

          security_context {
            allow_privilege_escalation = false
            privileged                 = false
            run_as_non_root            = true
            capabilities {
              drop = ["all"]
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_secret" "grafana_pdc_agent" {
  metadata {
    name      = "grafana-pdc-agent"
    namespace = "default"
  }

  type = "Opaque"

  data = {
    cluster             = var.grafana_pdc_cluster
    "hosted-grafana-id" = var.grafana_pdc_hosted_grafana_id
    token               = var.grafana_pdc_token
  }
}