#!/bin/bash

HOME="/data"
cd ${HOME}

set -eou pipefail

export CAPK_GUEST_K8S_VERSION="{{ .capk_guest_k8s_version }}"
export CLUSTER_NAME="{{ .cluster_name }}"
export WORKER_MACHINE_COUNT="{{ .worker_machine_count }}"
export WORKER_MACHINE_CPU="{{ .worker_machine_cpu }}"
export WORKER_MACHINE_MEMORY="{{ .worker_machine_memory }}"
export KUBERNETES_VERSION="v${CAPK_GUEST_K8S_VERSION}"
export NODE_VM_IMAGE_TEMPLATE="quay.io/capk/ubuntu-2204-container-disk:v${CAPK_GUEST_K8S_VERSION}"



export NATS_SUCCESS_MESSAGE="Task Completed Successfully"
export NATS_FAILURE_MESSAGE="Task Failed"
export SOCKETS=1
export THREADS=1
export CONTROL_PLANE_MACHINE_COUNT=3

ADMIN_CLUSTER_KUBECONFIG_STRING='{{.admin_cluster_kubeconfig_string}}'
INFRA_CSI_VERSION=v0.1.0
TENANT_CSI_VERSION=v0.1.0
INFRA_STORAGE_CLASS_NAME=hvl
INFRA_SNAPSHOT_CLASS_NAME=longhorn-snapshot
CLUSTER_NAMESPACE="{{ .cluster_namespace }}"
WORKLOAD_KUBECONFIG=""
CILIUM_VERSION=1.17.5
# Logging setup
exec > >(tee -a /data/create-script.log) 2>&1
SHIPPER_FILE=/data/create-script.log nats-logger &

rollback() {
    log "ERROR" "Rolling back cluster creation process."
    export KUBECONFIG=$ADMIN_CLUSTER_KUBECONFIG || true
    kubectl delete cluster $CLUSTER_NAME -n ${CLUSTER_NAMESPACE} || true
    sleep 30s
    kubectl delete ns $CLUSTER_NAMESPACE || true
    log "INFO" "Rollback completed."
}

function finish {
    result=$?
    if [ $result -ne 0 ]; then
        rollback || true
        log "ERROR" "Cluster Creation: $NATS_FAILURE_MESSAGE !!!"
    else
        # Cluster Created Successfully
        log "INFO" "Cluster Creation: $NATS_SUCCESS_MESSAGE !!!"
    fi
    sleep 10

    exit $result
}

trap finish EXIT

timestamp() {
    date +"%Y/%m/%d %T"
}

log() {
    local type="$1"
    local msg="$2"
    local script_name=${0##*/}
    echo "$(timestamp) [$script_name] [$type] $msg"
}

retry() {
    local retries="$1"
    shift
    local count=0
    local wait=5
    until "$@"; do
        exit="$?"
        if [ $count -lt $retries ]; then
            log "INFO" "Attempt $count/$retries. Command exited with exit_code: $exit. Retrying after $wait seconds..."
            sleep $wait
        else
            log "ERROR" "Command failed in all $retries attempts with exit_code: $exit. Stopping further attempts."
            return $exit
        fi
        count=$(($count + 1))
    done
    return 0
}

write_ADMIN_CLUSTER_kubeconfig_string() {
    log "INFO" "Writing Admin cluster kubeconfig string."
    echo "$ADMIN_CLUSTER_KUBECONFIG_STRING" >admin-cluster-kubeconfig.yaml
    export KUBECONFIG=admin-cluster-kubeconfig.yaml
    export ADMIN_CLUSTER_KUBECONFIG=${KUBECONFIG}
}

create_workload_cluster() {
    log "INFO" "Creating Workload cluster."
    local cmnd="clusterctl generate cluster"
    retry 5 ${cmnd} ${CLUSTER_NAME} -n $CLUSTER_NAMESPACE --from /home/assets/template.yaml >cluster.yaml
    kubectl create ns $CLUSTER_NAMESPACE --kubeconfig=${KUBECONFIG} || true
    cmnd="kubectl apply -f cluster.yaml -n ${CLUSTER_NAMESPACE}"
    retry 5 ${cmnd}

    log "INFO" "Waiting for cluster to be ready."
    kubectl wait --for=condition=ready cluster --all -n $CLUSTER_NAMESPACE --timeout=20m
    sleep 1m
    kubectl wait --for=condition=Ready machines --all -n $CLUSTER_NAMESPACE --timeout=30m
    log "INFO" "Cluster ${CLUSTER_NAME} created successfully."
}
generate_kubeconfig() {
    log "INFO" "Generating kubeconfig."
    local cmnd="clusterctl get kubeconfig"
    retry 5 ${cmnd} ${CLUSTER_NAME} -n ${CLUSTER_NAMESPACE} --kubeconfig=${ADMIN_CLUSTER_KUBECONFIG} >$HOME/cluster.kubeconfig
    WORKLOAD_KUBECONFIG=$HOME/cluster.kubeconfig
}
install_cni() {
    helm repo add cilium https://helm.cilium.io/
    helm repo update cilium
    local cmd="helm install --kubeconfig=${WORKLOAD_KUBECONFIG} cilium cilium/cilium --version $CILIUM_VERSION --namespace kube-system"
    if [ "$WORKER_MACHINE_COUNT" -eq 1 ]; then
        cmd="${cmd} --set operator.replicas=1"
    fi
    retry 5 ${cmd}
    sleep 1m
    retry 5 kubectl --kubeconfig=${WORKLOAD_KUBECONFIG} wait --for=condition=ready pods --all -A --timeout=2m
    log "INFO" "Successfully installed CNI"
}
install_csi() {
    log "INFO" "Installing csi...."
    cat <<EOF >storage-class-inforce.yaml
tenant:
  storageClassEnforcement:
    allowList:
      - ${INFRA_STORAGE_CLASS_NAME}
    allowAll: false
    allowDefault: false
    storageSnapshotMapping:
      - volumeSnapshotClasses:
          - ${INFRA_SNAPSHOT_CLASS_NAME}
        storageClasses:
          - ${INFRA_STORAGE_CLASS_NAME}
EOF
    local cmnd="helm upgrade -i kubevirt-infra-csi-driver oci://ghcr.io/appscode-charts/kubevirt-infra-csi-driver -n ${CLUSTER_NAMESPACE} --create-namespace \
    --version=${INFRA_CSI_VERSION} --set tenant.kubeconfig=$(cat $HOME/cluster.kubeconfig | base64 -w0) --set tenant.labels=csi-driver/cluster=${CLUSTER_NAME} \
    --set tenant.namespace=${CLUSTER_NAMESPACE} -f storage-class-inforce.yaml"

    retry 5 ${cmnd} --kubeconfig=${ADMIN_CLUSTER_KUBECONFIG}

    local cmnd="helm upgrade -i kubevirt-tenant-csi-driver oci://ghcr.io/appscode-charts/kubevirt-tenant-csi-driver -n kubevirt-csi-driver --create-namespace \
    --version=${TENANT_CSI_VERSION}  --set tenant.namespace=${CLUSTER_NAMESPACE} \
    --set tenant.labels=csi-driver/cluster=${CLUSTER_NAME} \
    --set infra.storageClassName=${INFRA_STORAGE_CLASS_NAME} \
    --set infra.snapshotClassName=${INFRA_SNAPSHOT_CLASS_NAME}"

    retry 5 ${cmnd} --kubeconfig=${WORKLOAD_KUBECONFIG}

}
init() {
    log "INFO" "Starting Cluster Creation Script."
    write_ADMIN_CLUSTER_kubeconfig_string
    create_workload_cluster
    generate_kubeconfig
    install_cni
    install_csi
}

init
