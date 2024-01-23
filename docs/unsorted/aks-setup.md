# Setup and Deploy Retina on AKS

1. Create a resource group
`az group create --name <resource-group-name> --location <location>`
2. Create a cluster for the resource group and use your ssh key
`az aks create -g <resource-group-name> -n <cluster-name> --node-count 2 --ssh-key-value ~/.ssh/id_rsa.pub --network-plugin azure`
3. Set credentials
`az aks get-credentials --name <cluster-name> --resource-group <resource-group-name> --overwrite-existing`
4. Have a container registry setup and attach to the cluster (Create your own or login to an existing one)
`az acr login --name <acr-name>`
5. Additional flags for acr login if using an existing container registry: `--username <username> --password <password>`
6. Attach credentials to your cluster/resource-group to the container registry
`az aks update -n <cluster> -g <resource-group> --attach-acr <acr-name>`
7. Build the docker image for retina `make retina-image`
8. Push to container registry:
`docker push <container-registry-url>/retina:<TAG>`
9. Installing retina onto the cluster -
`helm install retina <retina-repository-path>/deploy/manifests/controller/helm/retina/ --create-namespace --namespace retina --dependency-update`
