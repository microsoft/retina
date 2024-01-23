set -o xtrace
set -e

# upgrade helm with new value file
helm upgrade retina ./deploy/manifests/controller/helm/retina/\
 --namespace kube-system \
 --dependency-update \
 --values test/profiles/$5/values.yaml \
 --set image.repository=$1 \
 --set operator.repository=$2 \
 --set image.initRepository=$3 \
 --set image.tag=$4 \
 --set operator.tag=$4 \

 sleep 30s

# show configmap
kubectl get configmap -n kube-system retina-config -o yaml

# wait for retina pods to be ready
kubectl -n kube-system wait --for=condition=ready --timeout=600s pod -l app=retina
kubectl get pods -n kube-system -o wide -l app=retina
