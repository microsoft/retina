# Azure Managed Prometheus/Grafana

## Pre-Requisites

1. Install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).
2. [Create an AKS cluster](https://learn.microsoft.com/en-us/azure/aks/learn/quick-kubernetes-deploy-cli#create-a-resource-group).
3. Install Retina DaemonSet (see [Quick Installation](./setup.md)).

## Deploying Prometheus and Grafana

1. Create an Azure Monitor resource:

   ```shell
   az resource create \
     --resource-group $RESOURCE_GROUP \
     --namespace microsoft.monitor \
     --resource-type accounts \
     --name $AZURE_MONITOR_NAME \
     --location $REGION \
     --properties '{}'
   ```

2. Create a Grafana instance:

   ```shell
   az grafana create \
     --name $GRAFANA_NAME \
     --resource-group $RESOURCE_GROUP
   ```

3. Get the Azure Monitor and Grafana resource IDs

   ```bash
   export AZMON_RESOURCE_ID=$(az resource show --resource-group $RESOURCE_GROUP --name $AZURE_MONITOR_NAME --resource-type "Microsoft.Monitor/accounts" --query id -o tsv)
   export GRAFANA_RESOURCE_ID=$(az resource show --resource-group $RESOURCE_GROUP --name $GRAFANA_NAME --resource-type "microsoft.dashboard/grafana" --query id -o tsv)
   ```

4. Link both the Azure Monitor Workspace and Grafana instance to your cluster:

   ```shell
   az aks update --enable-azure-monitor-metrics \
     -n $NAME \
     -g $RESOURCE_GROUP \
     --azure-monitor-workspace-resource-id $AZMON_RESOURCE_ID \
     --grafana-resource-id  $GRAFANA_RESOURCE_ID
   ```

5. Verify that the Azure Monitor Pods are running. For example:

   ```shell
   kubectl get pod -n kube-system
   ```

   ```shell
   NAME                                  READY   STATUS    RESTARTS   AGE
   ama-metrics-5bc6c6d948-zkgc9          2/2     Running   0          26h
   ama-metrics-ksm-556d86b5dc-2ndkv      1/1     Running   0          26h
   ama-metrics-node-lbwcj                2/2     Running   0          26h
   ama-metrics-node-rzkzn                2/2     Running   0          26h
   ama-metrics-win-node-gqnkw            2/2     Running   0          26h
   ama-metrics-win-node-tkrm8            2/2     Running   0          26h
   ```

6. Verify that the Retina Pods are discovered by port-forwarding an AMA node Pod:

   ```bash
   kubectl port-forward -n kube-system $(kubectl get pod -n kube-system -l dsName=ama-metrics-node -o name | head -n 1) 9090:9090
   ```

   ```bash
   Forwarding from 127.0.0.1:9090 -> 9090
   Forwarding from [::1]:9090 -> 9090
   ```

7. Then go to [http://localhost:9090/targets](http://localhost:9090/targets) to see the Retina Pods being discovered and scraped:

   ![alt text](img/prometheus-retina-pods.png)

## Configuring Grafana

In the Azure Portal, find your Grafana instance. Click on the Grafana Endpoint URL, then follow [Configuring Grafana](./configuring-grafana.md).
