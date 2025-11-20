#! /bin/bash

## Paths
export ROOT_DEBUG_DIR=/Users/jparrill/RedHat/RedHat_Engineering/hypershift/bugs/OCPBUGS-65713
export HYP_REPO=/Users/jparrill/RedHat/RedHat_Engineering/hypershift/repos/hypershift
export KUBECONFIG=/Users/jparrill/RedHat/RedHat_Engineering/hypershift/hosted_clusters/jparrill-jparrill-dev/kubeconfig

## Hosted Cluster
export HC_NS=clusters
export HC_NAME=jparrill-hosted
export HCP_NS=$HC_NS-$HC_NAME
export OCP_RELEASE=quay.io/openshift-release-dev/ocp-release:4.19.19-x86_64
export TOKEN_SECRET=$(KUBECONFIG=$KUBECONFIG oc get secret -n $HCP_NS | grep token | grep $HC_NAME | cut -f1 -d" ")

## Files
export DEBUG_DIR=$ROOT_DEBUG_DIR/debug-ignition
export FEATURE_GATE_FILE_MANIFEST=$ROOT_DEBUG_DIR/feature-gate.yaml
export OS_RELEASE_FILE=$ROOT_DEBUG_DIR/os-release

go run $HYP_REPO/ignition-server/main.go run-local-ign-provider \
    --namespace=$HCP_NS \
    --image=$OCP_RELEASE \
    --token-secret=$TOKEN_SECRET \
    --feature-gate-manifest=$FEATURE_GATE_FILE_MANIFEST \
    --dir=$DEBUG_DIR \
    --os-release-file=$OS_RELEASE_FILE
