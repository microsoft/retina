#!/bin/bash
numDeployments=50
numNetpolsPerDeployment=1
numNamespaces=1

echo "creating $numDeployments deployments and $numNetpolsPerDeployment netpols per deployment"

mkdir -p deployments
mkdir -p netpols
mkdir -p serviceaccounts
mkdir -p clusterrolebindings
rm deployments/deployment*.yaml
rm netpols/netpol*.yaml
rm serviceaccounts/serviceaccount*.yaml
rm clusterrolebindings/clusterrolebinding*.yaml

for (( k=1; k<=$numNamespaces; k++ )); do
    ## generate clusterrolebinding
    toFile=clusterrolebindings/clusterrolebinding-$k.yaml
    sed "s/nameReplace/test-clusterrolebinding-$k/g" example-clusterrolebinding.yaml > $toFile
    sed -i "s/labelReplaceNamespace/test-ns-$k/g" $toFile
    sed -i "s/labelReplace/label-$i/g" $toFile

    ## generate service accounts
    toFile=serviceaccounts/serviceaccount-$i.yaml
    sed "s/nameReplace/test-other-$i/g" example-serviceaccount.yaml > $toFile
    sed -i "s/labelReplaceNamespace/test-ns-$k/g" $toFile
    sed -i "s/labelReplace/label-$i/g" $toFile
done

for (( i=1; i<=$numDeployments; i++ )); do
    ## generate all netpols for this deployment
    for (( j=1; j<=$numNetpolsPerDeployment; j++ )); do
        toFile=netpols/netpol-$i-$j.yaml
        sed "s/nameReplace/test-netpol-$i-$j/g" example-netpol.yaml > $toFile
        sed -i "s/labelReplace1/label-$i/g" $toFile
        z=$(( i + (j-1)*2 ))
        plus1=$(( z % numDeployments + 1))
        sed -i "s/labelReplace2/label-$plus1/g" $toFile
        plus2=$(( (z+1) % numDeployments + 1))
        sed -i "s/labelReplace3/label-$plus2/g" $toFile
    done

    for (( k=1; k<=$numNamespaces; k++ )); do
        ## generate deployment yaml
        toFile=deployments/deployment-$i.yaml
        sed "s/nameReplace/test-deployment-$i/g" example-deployment.yaml > $toFile
        sed -i "s/labelReplaceNamespace/test-ns-$k/g" $toFile
        sed -i "s/labelReplace/label-$i/g" $toFile
    done
done

echo "done"
