#!/usr/bin/env bash

set -e

user_help () {
    echo "Creates ToolchainCluster"
    echo "options:"
    echo "-t, --type                  joining cluster type (host or member)"
    echo "-mn, --member-ns            namespace where member-operator is running"
    echo "-hn, --host-ns              namespace where host-operator is running"
    echo "-mm, --multi-member         enables deploying multiple members in a single cluster, provide a unique id that will be used as a suffix for additional member cluster names"
    echo "-hk, --host-kubeconfig      kubeconfig of the host cluster"
    echo "-mk, --member-kubeconfig    kubeconfig of the member cluster"
    echo "-le, --lets-encrypt         use let's encrypt certificate"
    exit 0
}

login_to_cluster() {
  if [[ -n "${MEMBER_KUBECONFIG_FILE}" ]]; then
    OC_ADDITIONAL_PARAMS="--kubeconfig=${MEMBER_KUBECONFIG_FILE}"
  fi
  if [[ "${1}" == "host" ]] && [[ -n "${HOST_KUBECONFIG_FILE}" ]]; then
    OC_ADDITIONAL_PARAMS="--kubeconfig=${HOST_KUBECONFIG_FILE}"
  fi
}

wait_for_service_account() {
NEXT_WAIT_TIME=0
while [[ -z `oc get sa ${SA_NAME} -n ${OPERATOR_NS} ${OC_ADDITIONAL_PARAMS} 2>/dev/null || true` ]]; do
    if [[ ${NEXT_WAIT_TIME} -eq 300 ]]; then
       echo "reached timeout of waiting for the ServiceAccount ${SA_NAME} in namespace ${OPERATOR_NS} ... The SA should be deployed by the toolchaincluster_resource controller."
       exit 1
    fi
    echo "$(( NEXT_WAIT_TIME++ )). attempt (out of 300) of waiting for ServiceAccount ${SA_NAME} in namespace ${OPERATOR_NS}."
    sleep 1
done
}

if [[ $# -lt 2 ]]
then
    user_help
fi

while test $# -gt 0; do
       case "$1" in
            -h|--help)
                user_help
                ;;
            -t|--type)
                shift
                JOINING_CLUSTER_TYPE=$1
                shift
                ;;
            -mn|--member-ns)
                shift
                MEMBER_OPERATOR_NS=$1
                shift
                ;;
            -hn|--host-ns)
                shift
                HOST_OPERATOR_NS=$1
                shift
                ;;
            -hk|--host-kubeconfig)
                shift
                HOST_KUBECONFIG_FILE=$1
                shift
                ;;
            -mk|--member-kubeconfig)
                shift
                MEMBER_KUBECONFIG_FILE=$1
                shift
                ;;
            -mm|--multi-member)
                shift
                MULTI_MEMBER=$1
                shift
                ;;
            -le|--lets-encrypt)
                LETS_ENCRYPT=true
                shift
                ;;
            *)
               echo "$1 is not a recognized flag!"
               user_help
               exit -1
               ;;
      esac
done

CLUSTER_JOIN_TO="host"
if [[ ${JOINING_CLUSTER_TYPE} == "host" ]]; then
  CLUSTER_JOIN_TO="member"
fi

# We need this to configurable to work with dynamic namespaces from end to end tests
HOST_OPERATOR_NS=${HOST_OPERATOR_NS:-toolchain-host-operator}
MEMBER_OPERATOR_NS=${MEMBER_OPERATOR_NS:-toolchain-member-operator}

OPERATOR_NS=${MEMBER_OPERATOR_NS}
CLUSTER_JOIN_TO_OPERATOR_NS=${HOST_OPERATOR_NS}
if [[ ${JOINING_CLUSTER_TYPE} == "host" ]]; then
  OPERATOR_NS=${HOST_OPERATOR_NS}
  CLUSTER_JOIN_TO_OPERATOR_NS=${MEMBER_OPERATOR_NS}
fi

echo ${OPERATOR_NS}
echo ${CLUSTER_JOIN_TO_OPERATOR_NS}

login_to_cluster ${JOINING_CLUSTER_TYPE}


SA_NAME="toolchaincluster-${JOINING_CLUSTER_TYPE}"
wait_for_service_account

echo "Getting ${JOINING_CLUSTER_TYPE} SA token"
SA_TOKEN=$(oc create token ${SA_NAME} --duration 87600h -n ${OPERATOR_NS} ${OC_ADDITIONAL_PARAMS})

echo "SA token retrieved"
if [[ ${LETS_ENCRYPT} == "true" ]]; then
    echo "Using let's encrypt certificate"
else
    INSECURE_PARAM="  disabledTLSValidations:
    - '*'"
fi

echo "Fetching information about the clusters"
API_ENDPOINT=`oc get infrastructure cluster -o jsonpath='{.status.apiServerURL}' ${OC_ADDITIONAL_PARAMS}`
echo "API endpoint retrieved: ${API_ENDPOINT}"
# The regexp below extracts the domain name from the API server URL, taking everything after "//" until a ":" or "/" (or end of line) is reached.
# The "api." prefix is removed from the domain if present. E.g. "https://api.server.domain.net:6443" -> "server.domain.net".
JOINING_CLUSTER_NAME=`echo "${API_ENDPOINT}" | sed 's/^[^/]*\/\/\([^:/]*\)\(:.*\)\{0,1\}\(\/.*\)\{0,1\}$/\1/' | sed 's/^api\.//'`
echo "Joining cluster name: ${JOINING_CLUSTER_NAME}"

login_to_cluster ${CLUSTER_JOIN_TO}

CLUSTER_JOIN_TO_API_ENDPOINT=`oc get infrastructure cluster -o jsonpath='{.status.apiServerURL}' ${OC_ADDITIONAL_PARAMS}`
echo "API endpoint of the cluster it is joining to: ${CLUSTER_JOIN_TO_API_ENDPOINT}"
CLUSTER_JOIN_TO_NAME=`echo "${CLUSTER_JOIN_TO_API_ENDPOINT}" | sed 's/^[^/]*\/\/\([^:/]*\)\(:.*\)\{0,1\}\(\/.*\)\{0,1\}$/\1/' | sed 's/^api\.//'`
echo "The cluster name it is joining to: ${CLUSTER_JOIN_TO_NAME}"

echo "Creating ${JOINING_CLUSTER_TYPE} secret"
SECRET_NAME=${SA_NAME}-${OPERATOR_NS}-${JOINING_CLUSTER_NAME}
if [[ -n `oc get secret -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS} | grep ${SECRET_NAME}` ]]; then
    oc delete secret ${SECRET_NAME} -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS}
fi

oc create secret generic ${SECRET_NAME} --from-literal=token="${SA_TOKEN}" --from-literal=kubeconfig="apiVersion: v1
clusters:
- cluster:
    server: ${API_ENDPOINT}
  name: cluster
contexts:
- context:
    cluster: cluster
    namespace: ${OPERATOR_NS}
    user: auth
  name: ctx
current-context: ctx
kind: Config
preferences: {}
users:
- name: auth
  user:
    token: ${SA_TOKEN}" -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS}

# We need to ensure toolchain cluster name length is <= 63 chars, it ends with an alphanumeric character and is unique
# name between member1 and member2.
#
# 1) we concatenate the "fixed cluster name" part  with the unique id e.g:
# member-1
CLUSTERNAME_FIXED_PART="${JOINING_CLUSTER_TYPE}-${MULTI_MEMBER}"
#
# 2) we get the length of the "fixed cluster name" part
# in this case member-1 (length 8 chars)
CLUSTERNAME_LENGTH_TO_REMOVE="${#CLUSTERNAME_FIXED_PART}"
# we calculate up to how many chars we can keep from the cluster name (that could exceed 63 chars length )
# in this case 62-8=54 chars ( we keep in account that we may have to append the id if MULTI_MEMBER is empty)
CLUSTERNAME_LENGTH_TO_KEEP=$((62-CLUSTERNAME_LENGTH_TO_REMOVE))
# Since MULTI_MEMBER variable is appended at the end of kubernetes object name for toolchaincluster resource,
# let's set a "member id" if not provided and for names which il be truncated, so that we are sure that those object names will end with an alphanumerical char.
if [ -z "$MULTI_MEMBER" ] && [ ${#JOINING_CLUSTER_NAME} -ge ${CLUSTERNAME_LENGTH_TO_KEEP} ];
then
      MULTI_MEMBER=1
fi
#
# 3) we remove the extra characters from the "middle" of the name (specifically from the name of the cluster), so that we can ensure the name ends with and alphanumerical character (the MULTI_MEMBER id , which is always set), e.g:
# JOINING_CLUSTER_NAME=a67d9ea16fe1a48dfbfd0526b33ac00c-279e3fade0dc0068.elb.us-east-1.amazonaws.com
# we keep from char index 0 up to char 55 in the cluster name string, removing the substring "-1.amazonaws.com" so that now the toolchain name goes from 79 chars to 63, is unique between member1 and member2 and ends with a alphanumerical character.
# result is TOOLCHAINCLUSTER_NAME=a67d9ea16fe1a48dfbfd0526b33ac00c-279e3fade0dc0068.elb.us-east-1
TOOLCHAINCLUSTER_NAME="${JOINING_CLUSTER_TYPE}-${JOINING_CLUSTER_NAME:0:CLUSTERNAME_LENGTH_TO_KEEP}${MULTI_MEMBER}"

# We need to label the secret with the SA token with the toolchain cluster name so that in the future we can flip the dependency and create
# the toolchain cluster based on the existence of the label.
oc label secret ${SECRET_NAME} -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS} "toolchain.dev.openshift.com/toolchain-cluster=${TOOLCHAINCLUSTER_NAME}"

CLUSTER_JOIN_TO_TYPE_NAME=CLUSTER_JOIN_TO
if [[ ${CLUSTER_JOIN_TO_TYPE_NAME} != "host" ]]; then
    CLUSTER_JOIN_TO_TYPE_NAME="member"
fi

# add cluster role label only for member clusters
CLUSTER_LABEL=""
if [[ ${JOINING_CLUSTER_TYPE} == "member" ]]; then
    CLUSTER_LABEL="cluster-role.toolchain.dev.openshift.com/tenant: ''"
fi

TOOLCHAINCLUSTER_CRD="apiVersion: toolchain.dev.openshift.com/v1alpha1
kind: ToolchainCluster
metadata:
  name: ${TOOLCHAINCLUSTER_NAME}
  namespace: ${CLUSTER_JOIN_TO_OPERATOR_NS}
  labels:
    namespace: ${OPERATOR_NS}
    ownerClusterName: obsolete
    ${CLUSTER_LABEL}
spec:
  apiEndpoint: ${API_ENDPOINT}
  secretRef:
    name: ${SECRET_NAME}
${INSECURE_PARAM}
"

echo "Creating ToolchainCluster representation of ${JOINING_CLUSTER_TYPE} in ${CLUSTER_JOIN_TO}:"
cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
${TOOLCHAINCLUSTER_CRD}
EOF
