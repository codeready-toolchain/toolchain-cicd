#!/usr/bin/env bash

set -e

user_help () {
    echo "Creates ToolchainCluster"
    echo "options:"
    echo "-t, --type            joining cluster type (host or member)"
    echo "-tn, --type-name      the type name of the joining cluster (host, member or e2e)"
    echo "-tc, --target-cluster the name of the cluster it should join to - applicable only together with '--sandbox-config' param (host, member1, member2,...)"
    echo "-mn, --member-ns      namespace where member-operator is running"
    echo "-hn, --host-ns        namespace where host-operator is running"
    echo "-s,  --single-cluster running both operators on single cluster"
    echo "-mm, --multi-member   enables deploying multiple members in a single cluster, provide a unique id that will be used as a suffix for additional member cluster names"
    echo "-kc, --kube-config    kubeconfig for managing multiple clusters"
    echo "-sc, --sandbox-config sandbox config file for managing Dev Sandbox instance - applicable only together with '--target-cluster' param"
    echo "-le, --lets-encrypt   use let's encrypt certificate"
    exit 0
}

login_to_cluster() {
    if [[ ${SINGLE_CLUSTER} != "true" ]]; then
      if [[ -z ${KUBECONFIG_FILE} ]] && [[ -z ${SANDBOX_CONFIG} ]]; then
        echo "Please specify the path to kube config file using the parameter --kube-config"
        echo "or specify SA tokens to be used when reaching operators using the parameters --host-token and --member-token"
      elif [[ -n ${KUBECONFIG_FILE} ]]; then
        oc config use-context "$1-admin"
      else
        REGISTER_SERVER_API=$(yq -r .\"$1\".serverAPI ${SANDBOX_CONFIG})
        SANDBOX_SA_TOKEN=$(yq -r .\"$1\".token ${SANDBOX_CONFIG})
        OC_ADDITIONAL_PARAMS="--token=${SANDBOX_SA_TOKEN} --server=${REGISTER_SERVER_API}"
      fi
    fi
}

create_service_account() {
# we need to delete the bindings since we cannot change the roleRef of the existing bindings
if [[ -n `oc get rolebinding ${SA_NAME} 2>/dev/null` ]]; then
    oc delete rolebinding ${SA_NAME} -n ${OPERATOR_NS} ${OC_ADDITIONAL_PARAMS}
fi

cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
EOF

if [[ ${JOINING_CLUSTER_TYPE} == "host" ]]; then
    cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
rules:
- apiGroups:
  - toolchain.dev.openshift.com
  resources:
  - "bannedusers"
  - "masteruserrecords"
  - "notifications"
  - "nstemplatetiers"
  - "spaces"
  - "spacebindings"
  - "tiertemplates"
  - "toolchainconfigs"
  - "toolchainclusters"
  - "toolchainstatuses"
  - "usersignups"
  - "usertiers"
  - "proxyplugins"
  verbs:
  - "*"
EOF
else
    CLUSTER_ROLE_NAME=${SA_NAME}-${OPERATOR_NS}-toolchaincluster
    # we need to delete the binding since we cannot change the roleRef of the existing binding
    if [[ -n `oc get ClusterRoleBinding ${CLUSTER_ROLE_NAME} ${OC_ADDITIONAL_PARAMS} 2>/dev/null` ]]; then
      oc delete ClusterRoleBinding ${CLUSTER_ROLE_NAME} ${OC_ADDITIONAL_PARAMS}
    fi
    # Additional permissions within user namespace are specified as part of namespace templates. eg. https://github.com/codeready-toolchain/host-operator/blob/0e292ef3fedea2a839e6800bfee635c4db41f088/deploy/templates/nstemplatetiers/appstudio/ns_appstudio.yaml#L19-L53
    cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
rules:
- apiGroups:
  - toolchain.dev.openshift.com
  resources:
  - "idlers"
  - "nstemplatesets"
  - "memberoperatorconfigs"
  - "memberstatuses"
  - "toolchainclusters"
  - "useraccounts"
  verbs:
  - "*"
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${CLUSTER_ROLE_NAME}
rules:
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups: [""]
  resources: ["users", "groups"]
  verbs: ["impersonate"]
- apiGroups:
  - toolchain.dev.openshift.com
  resources:
  - "spacerequests"
  verbs:
  - "*"
- apiGroups:
  - toolchain.dev.openshift.com
  resources:
  - spacerequests/finalizers
  verbs:
  - update
- apiGroups:
  - toolchain.dev.openshift.com
  resources:
  - spacerequests/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - route.openshift.io
  resources:
  - routes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - "namespaces"
  verbs:
  - "get"
  - "list"
  - "watch"
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${CLUSTER_ROLE_NAME}
subjects:
- kind: ServiceAccount
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
roleRef:
  kind: ClusterRole
  name: ${CLUSTER_ROLE_NAME}
  apiGroup: rbac.authorization.k8s.io
EOF
fi

cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
subjects:
- kind: ServiceAccount
  name: ${SA_NAME}
roleRef:
  kind: Role
  name: ${SA_NAME}
  apiGroup: rbac.authorization.k8s.io
EOF
}

create_service_account_e2e() {
CLUSTER_ROLE_BINDING_NAME=${SA_NAME}-${OPERATOR_NS}
# we need to delete the binding since we cannot change the roleRef of the existing binding
if [[ -n `oc get ClusterRoleBinding ${CLUSTER_ROLE_BINDING_NAME} 2>/dev/null` ]]; then
    oc delete ClusterRoleBinding ${CLUSTER_ROLE_BINDING_NAME} ${OC_ADDITIONAL_PARAMS}
fi
echo "Creating SA ${SA_NAME}"
cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
EOF

echo "Creating ClusterRoleBinding ${CLUSTER_ROLE_BINDING_NAME}"
cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ${CLUSTER_ROLE_BINDING_NAME}
subjects:
- kind: ServiceAccount
  name: ${SA_NAME}
  namespace: ${OPERATOR_NS}
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
EOF

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
            -tn|--type-name)
                shift
                JOINING_CLUSTER_TYPE_NAME=$1
                shift
                ;;
            -tc|--target-cluster)
                shift
                TARGET_CLUSTER_NAME=$1
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
            -kc|--kube-config)
                shift
                KUBECONFIG_FILE=$1
                shift
                ;;
            -sc|--sandbox-config)
                shift
                SANDBOX_CONFIG=$1
                shift
                ;;
            -s|--single-cluster)
                SINGLE_CLUSTER=true
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
if [[ -n ${SANDBOX_CONFIG} ]]; then
    CLUSTER_JOIN_TO=${TARGET_CLUSTER_NAME}
else
  if [[ ${JOINING_CLUSTER_TYPE} == "host" ]]; then
    CLUSTER_JOIN_TO="member"
  fi
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

JOINING_CLUSTER_TYPE_NAME=${JOINING_CLUSTER_TYPE_NAME:-${JOINING_CLUSTER_TYPE}}

echo ${OPERATOR_NS}
echo ${CLUSTER_JOIN_TO_OPERATOR_NS}

login_to_cluster ${JOINING_CLUSTER_TYPE}

if [[ ${JOINING_CLUSTER_TYPE_NAME} != "e2e" ]]; then
    SA_NAME="toolchaincluster-${JOINING_CLUSTER_TYPE_NAME}${MULTI_MEMBER}"
    create_service_account
else
    SA_NAME="e2e-service-account"
    if [[ ! -z ${MULTI_MEMBER} ]]; then
      SA_NAME="${OPERATOR_NS}"
    fi
    create_service_account_e2e
fi

echo "Getting ${JOINING_CLUSTER_TYPE} SA token"
SA_SECRET=`oc get sa ${SA_NAME} -n ${OPERATOR_NS} -o json ${OC_ADDITIONAL_PARAMS} | jq -r .secrets[].name | { grep token || true; }`
if [[ -n ${SA_SECRET} ]]; then
  echo "SA secret found (OpenShift 4.10 and older): ${SA_SECRET}"
  SA_TOKEN=`oc get secret ${SA_SECRET} -n ${OPERATOR_NS}  -o json ${OC_ADDITIONAL_PARAMS} | jq -r '.data["token"]' | base64 --decode`
else
  SA_SECRET=`oc get sa ${SA_NAME} -n ${OPERATOR_NS} -o json ${OC_ADDITIONAL_PARAMS} | jq -r .secrets[].name | { grep dockercfg -m 1 || true; }`
  echo "SA secret found (OpenShift 4.11 and newer): ${SA_SECRET}"
  SA_TOKEN=`oc get secret ${SA_SECRET} -n ${OPERATOR_NS}  -o json ${OC_ADDITIONAL_PARAMS} | jq -r '.metadata.annotations."openshift.io/token-secret.value"'`

  if [[ -n ${SA_TOKEN} ]]; then
    echo "Token found as annotation openshift.io/token-secret.value"
  else
    echo "Token not found - generating using 'create token' command"
    SA_TOKEN=$(oc create token ${SA_NAME} --duration 876000h -n ${OPERATOR_NS} ${OC_ADDITIONAL_PARAMS})
  fi
fi
echo "SA token retrieved"
if [[ ${LETS_ENCRYPT} == "true" ]]; then
    echo "Using let's encrypt certificate"
    SA_CA_CRT=`curl https://letsencrypt.org/certs/lets-encrypt-r3.pem | base64 | tr -d '\n'`
else
    echo "Using standard OpenShift certificate"
    SA_CA_CRT=$(oc config view --raw -o json ${OC_ADDITIONAL_PARAMS} | jq ".clusters[] | select(.name==\"$(oc config view -o json ${OC_ADDITIONAL_PARAMS} | jq ".contexts[] | select(.name==\"$(oc config current-context ${OC_ADDITIONAL_PARAMS} 2>/dev/null)\")" | jq -r .context.cluster)\")" | jq -r '.cluster."certificate-authority-data"')
fi

if [[ -n ${SANDBOX_CONFIG} ]]; then
    echo "Using sandbox.yaml file as a config"
    API_ENDPOINT=$(yq -r .\"${JOINING_CLUSTER_TYPE}\".serverAPI ${SANDBOX_CONFIG})
    JOINING_CLUSTER_NAME=$(yq -r .\"${JOINING_CLUSTER_TYPE}\".serverName ${SANDBOX_CONFIG})

    login_to_cluster ${CLUSTER_JOIN_TO}

    CLUSTER_JOIN_TO_NAME=$(yq -r .\"${CLUSTER_JOIN_TO}\".serverName ${SANDBOX_CONFIG})
else
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
fi

echo "Creating ${JOINING_CLUSTER_TYPE} secret"
SECRET_NAME=${SA_NAME}-${JOINING_CLUSTER_NAME}
if [[ -n `oc get secret -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS} | grep ${SECRET_NAME}` ]]; then
    oc delete secret ${SECRET_NAME} -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS}
fi
oc create secret generic ${SECRET_NAME} --from-literal=token="${SA_TOKEN}" --from-literal=ca.crt="${SA_CA_CRT}" -n ${CLUSTER_JOIN_TO_OPERATOR_NS} ${OC_ADDITIONAL_PARAMS}

# We need to ensure toolchain cluster name length is <= 63 chars, it ends with an alphanumeric character and is unique
# name between member1 and member2.
#
# 1) we concatenate the "fixed cluster name" part  with the unique id e.g:
# member-1
CLUSTERNAME_FIXED_PART="${JOINING_CLUSTER_TYPE_NAME}-${MULTI_MEMBER}"
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
TOOLCHAINCLUSTER_NAME="${JOINING_CLUSTER_TYPE_NAME}-${JOINING_CLUSTER_NAME:0:CLUSTERNAME_LENGTH_TO_KEEP}${MULTI_MEMBER}"

CLUSTER_JOIN_TO_TYPE_NAME=CLUSTER_JOIN_TO
if [[ ${CLUSTER_JOIN_TO_TYPE_NAME} != "host" ]]; then
    CLUSTER_JOIN_TO_TYPE_NAME="member"
fi

# add cluster role label only for member clusters
CLUSTER_LABEL=""
if [[ ${JOINING_CLUSTER_TYPE_NAME} == "member" ]]; then
    CLUSTER_LABEL="cluster-role.toolchain.dev.openshift.com/tenant: ''"
fi

TOOLCHAINCLUSTER_CRD="apiVersion: toolchain.dev.openshift.com/v1alpha1
kind: ToolchainCluster
metadata:
  name: ${TOOLCHAINCLUSTER_NAME}
  namespace: ${CLUSTER_JOIN_TO_OPERATOR_NS}
  labels:
    type: ${JOINING_CLUSTER_TYPE_NAME}
    namespace: ${OPERATOR_NS}
    ownerClusterName: obsolete
    ${CLUSTER_LABEL}
spec:
  apiEndpoint: ${API_ENDPOINT}
  caBundle: ${SA_CA_CRT}
  secretRef:
    name: ${SECRET_NAME}
"

echo "Creating ToolchainCluster representation of ${JOINING_CLUSTER_TYPE} in ${CLUSTER_JOIN_TO}:"
cat <<EOF | oc apply ${OC_ADDITIONAL_PARAMS} -f -
${TOOLCHAINCLUSTER_CRD}
EOF