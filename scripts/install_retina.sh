set -o xtrace
set -e

echo "Install Retina on cluster using helm"

helm install retina ./deploy/manifests/controller/helm/retina/\
 --namespace kube-system \
 --dependency-update \
 --values $5/values.yaml \
 --set image.repository=$1 \
 --set operator.repository=$2 \
 --set image.initRepository=$3 \
 --set image.tag=$4 \
 --set operator.tag=$4 \

sleep 20s

kubectl get pods -n kube-system -o wide -l app=retina
# wait for retina pods to be ready
kubectl -n kube-system wait --for=condition=ready --timeout=600s pod -l app=retina # windows server 2019 is really slow
kubectl get pods -n kube-system -o wide -l app=retina
kubectl describe pods -n kube-system -l app=retina

# Check if the profile is advanced. The operator isn't needed for basic profile.
if [ "$6" = "adv" ]; then
    kubectl -n kube-system wait --for=condition=ready --timeout=600s pod -l app=retina-operator # windows server 2019 is really slow
    kubectl get pods -n kube-system -o wide -l app=retina-operator
    kubectl describe pods -n kube-system -l app=retina-operator
fi

# This applies CRDs corresponding to the integration test profile.
# For example, when we are running the adv profile, we will apply test/profiles/advanced/crd/metrics_config_crd.yaml
if [ -d "$5/crd" ] && [ "$(ls -A $5/crd | grep .yaml)" ]; then
    kubectl apply -f $5/crd/
fi
