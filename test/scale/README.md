# Retina Scale Testing

* example-*.yaml files are used to generate resources for the cluster using `./generate-yamls.sh`. This generates cluster role bindings, deployments, network polcies, and service accounts.

* `create-all.sh` will create all the resources for the cluster.

* `cpu-and-mem.sh` is the script used to track the cpu and mem of retina pods, and node metrics and output the results as a csv in `./results/`.

* `pprof.sh` will get the pprof for retina agents which have mem > 100 Mb and will output the results in `./results/$pod`.

* `restarts.sh` will get the previous logs for retina agents which have restarted and will output the results in `./results`.

* `scrape-metrics.sh` is currently not used, but can be used/modified to go through each retina pod and get logs or metrics.

* `az aks nodepool scale --name <node-pool> --cluster-name <cluster-name> --resource-group <resource-group-name> --node-count 1000` was used to scale to 1k nodes for testing.
