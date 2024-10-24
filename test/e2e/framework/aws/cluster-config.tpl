apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  region: {{.Region}}
  name: {{.ClusterName}}
iam:
  withOIDC: true

addons:
  - name: vpc-cni
    configurationValues: |-
      enableNetworkPolicy: "true"

managedNodeGroups:
  - name: spot
    spot: true
    desiredCapacity: 3

