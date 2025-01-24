# Windows plugins

1. Cordon all windows nodes. Until the below selector is added, needed so helm install isn't blocked.
2. Install Linux Retina helm chart.

    `helm install retina ./deploy/standard/manifests/controller/helm/retina/ --namespace kube-system`

3. Uncordon the Windows and nodes.

4. Apply nodeSelector patch to prometheus-node-exporter

    ```bash
    kubectl patch ds/retina-prometheus-node-exporter -n kube-system --patch "$(cat ./windows/manifests/node-selector-patch.yaml)"
    kubectl patch deployment/retina-kube-prometheus-sta-operator -n kube-system --patch "$(cat ./windows/manifests/node-selector-patch.yaml)"
    kubectl patch deployment/retina-kube-state-metrics -n kube-system --patch "$(cat ./windows/manifests/node-selector-patch.yaml)"
    kubectl patch statefulset/prometheus-retina-kube-prometheus-sta-prometheus -n kube-system --patch "$(cat ./windows/manifests/node-selector-patch.yaml)"
    kubectl patch statefulset/alertmanager-retina-kube-prometheus-sta-alertmanager -n kube-system --patch "$(cat ./windows/manifests/node-selector-patch.yaml)"
    ```

5. Build (and push) the Retina windows image. (Don't forget to `az acr login`)

    `make retina-image-win-push`

6. Modify `./windows/manifests/windows.yaml` with the image tag you just pushed

7. Apply the Windows manifest

    `k apply -f ./windows/manifests/windows.yaml`
