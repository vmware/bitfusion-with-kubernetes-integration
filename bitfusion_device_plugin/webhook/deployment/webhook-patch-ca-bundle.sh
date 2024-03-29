#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

CA_BUNDLE=$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}')

if [ -z "${CA_BUNDLE}" ]; then
    CA_BUNDLE=$(kubectl get secrets -o jsonpath="{.items[?(@.metadata.annotations['kubernetes\.io/service-account\.name']=='default')].data.ca\.crt}")
fi


if [ "${K8S_PLATFORM}" == 'tkgi' ]; then
    str=""
    for line in `cat ca.pem | base64`
            do
            str=$str$line
    done
    CA_BUNDLE=$str
fi

export CA_BUNDLE

if command -v envsubst >/dev/null 2>&1; then
    envsubst
else
    sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi
