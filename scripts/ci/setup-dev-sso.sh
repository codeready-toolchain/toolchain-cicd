#!/usr/bin/env bash

user_help () {
    echo "Deploy in cluster keycloak and configure registration service to use it."
    echo "options:"
    echo "-sn, --ss-ns  Builds and pushes the operator to quay"
    echo "-h,  --help              To show this help text"
    echo ""
    exit 0
}

read_arguments() {
    if [[ $# -lt 2 ]]
    then
        echo "There are missing parameters"
        user_help
    fi

    while test $# -gt 0; do
           case "$1" in
                -h|--help)
                    user_help
                    ;;
                -sn|--sso-ns)
                    shift
                    DEV_SSO_NS=$1
                    shift
                    ;;
                *)
                   echo "$1 is not a recognized flag!" >> /dev/stderr
                   user_help
                   exit -1
                   ;;
          esac
    done
}
check_commands()
{
    for cmd in "$@"
    do
        check_command "$cmd"
    done
}

check_command()
{
    command -v "$1" > /dev/null && return 0

    printf "please install '%s' before running this script\n" "$1"
    exit 1
}

read_arguments $@

if [[ -n "${CI}" ]]; then
    set -ex
else
    set -e
fi


check_commands yq oc base64 openssl

# Install rhsso operator
SUBSCRIPTION_NAME=DEV_SSO_NS
printf "installing RH SSO operator\n"
    INSTALL_RHSSO="apiVersion: v1
kind: Namespace
metadata:
  name: ${DEV_SSO_NS}
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: og-rhsso
  namespace: ${DEV_SSO_NS}
spec:
  targetNamespaces:
  - ${DEV_SSO_NS}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${SUBSCRIPTION_NAME}
  namespace: ${DEV_SSO_NS}
spec:
  channel: stable
  name: rhsso-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic"
    echo "objects to be created in order to install operator"
    cat <<EOF | oc apply -f -
${INSTALL_RHSSO}
EOF

source wait-until-is-installed.sh "-crd keycloak.org -cs '' -n ${DEV_SSO_NS} -s ${SUBSCRIPTION_NAME}"

printf "installing dev Keycloak in namespace ${DEV_SSO_NS}\n"
export KEYCLOAK_SECRET=$(openssl rand -base64 32)
    INSTALL_KEYCLOAK="apiVersion: keycloak.org/v1alpha1
kind: Keycloak
metadata:
  name: sandbox-dev
  namespace: ${DEV_SSO_NS}
  labels:
    sso-toolchain: sandbox-dev
spec:
  externalAccess:
    enabled: true
  instances: 1
---
 apiVersion: keycloak.org/v1alpha1
 kind: KeycloakRealm
 metadata:
   name: sandbox-dev
   namespace: ${DEV_SSO_NS}
 spec:
   instanceSelector:
     matchLabels:
       sso-toolchain: sandbox-dev
   realm:
     id: sandbox-dev
     realm: sandbox-dev
     displayName: Sandbox Dev In-cluster Keycloak
     accessTokenLifespan: 7200
     accessTokenLifespanForImplicitFlow: 900
     enabled: true
     sslRequired: none
     registrationAllowed: false
     registrationEmailAsUsername: false
     rememberMe: false
     verifyEmail: false
     loginWithEmailAllowed: true
     duplicateEmailsAllowed: false
     resetPasswordAllowed: false
     editUsernameAllowed: false
     bruteForceProtected: false
     permanentLockout: false
     maxFailureWaitSeconds: 900
     minimumQuickLoginWaitSeconds: 60
     waitIncrementSeconds: 60
     quickLoginCheckMilliSeconds: 1000
     maxDeltaTimeSeconds: 43200
     failureFactor: 30
     clients:
       - id: 9a5018a7-5f92-40c9-b8f1-63f53bc32a68
         clientId: sandbox-public
         surrogateAuthRequired: false
         enabled: true
         clientAuthenticatorType: client-secret
         redirectUris:
           - '*'
         webOrigins:
           - '*'
         notBefore: 0
         bearerOnly: false
         consentRequired: false
         standardFlowEnabled: true
         implicitFlowEnabled: false
         directAccessGrantsEnabled: true
         serviceAccountsEnabled: false
         publicClient: true
         frontchannelLogout: false
         protocol: openid-connect
         protocolMappers: []
         attributes: {}
         authenticationFlowBindingOverrides: {}
         fullScopeAllowed: true
         nodeReRegistrationTimeout: -1
         defaultClientScopes: []
         optionalClientScopes: []
     clientScopes: []
     defaultDefaultClientScopes: []
     smtpServer: {}
     loginTheme: rh-sso
     eventsEnabled: false
     eventsListeners:
       - jboss-logging
     enabledEventTypes: []
     adminEventsEnabled: false
     adminEventsDetailsEnabled: false
     identityProviders: []
     identityProviderMappers: []
     internationalizationEnabled: false
     supportedLocales: []
     authenticationFlows: []
     authenticatorConfig: []
     userManagedAccessAllowed: false
     users:
       - credentials:
           - type: password
             value: user1
         email: user1@user.us
         emailVerified: true
         enabled: true
         firstName: user1
         id: user1
         username: user1
         clientRoles: {}"
    echo "objects to be created in order to install operator"
    cat <<EOF | oc apply -f -
${INSTALL_KEYCLOAK}
EOF

while ! oc get statefulset -n ${DEV_SSO_NS} keycloak &> /dev/null ; do
    printf "waiting for keycloak statefulset in ${DEV_SSO_NS} to be ready...\n"
    sleep 10
done

printf "waiting for keycloak in ${DEV_SSO_NS} to be ready...\n"
ATTEMPT=0
MAX_NUM_ATTEMPTS=100
SLEEP_TIME=1
while [[ -z $(oc get keycloak sandbox-dev -o jsonpath='{.status.ready}' -n ${DEV_SSO_NS} | grep "true" || true) ]]; do
    echo "$(( ATTEMPT++ )). attempt (out of ${MAX_NUM_ATTEMPTS}) of waiting for keycloak sandbox-dev/${DEV_SSO_NS} to be ready"
    sleep ${SLEEP_TIME}
    if [[ ${ATTEMPT} -eq ${MAX_NUM_ATTEMPTS} ]]; then
      echo "reached timeout of waiting for keycloak to be available in the cluster - see following info for debugging:"
      oc get keycloak sandbox-dev -n ${DEV_SSO_NS} -o yaml
    fi
done

BASE_URL=$(oc get ingresses.config.openshift.io/cluster -o jsonpath='{.spec.domain}')
RHSSO_URL="https://keycloak-${DEV_SSO_NS}.$BASE_URL"


oc rollout status statefulset -n ${DEV_SSO_NS} keycloak --timeout 20m

# Configure cluster OAuth authentication for keycloak
oc apply -f - << EOF
apiVersion: v1
kind: Secret
metadata:
  name: openid-client-secret-sandbox
  namespace: openshift-config
stringData:
  clientSecret: ${KEYCLOAK_SECRET}
type: Opaque
EOF

# Certificate used by keycloak is self-signed, we need to import and grant for it
oc get secrets -n openshift-ingress-operator router-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/ca.crt
oc create configmap ca-config-map --from-file="ca.crt=/tmp/ca.crt" -n openshift-config || true

# Patch
oc patch oauths.config.openshift.io/cluster --type=merge --patch-file=/dev/stdin << EOF
spec:
  identityProviders:
  - mappingMethod: lookup
    name: rhd
    openID:
      ca:
        name: ca-config-map
      claims:
        preferredUsername:
        - preferred_username
      clientID: sandbox
      clientSecret:
        name: openid-client-secret-sandbox
      issuer: ${RHSSO_URL}/auth/realms/sandbox-dev
    type: OpenID
EOF

## Configure toolchain to use the internal keycloak
oc patch ToolchainConfig/config -n toolchain-host-operator --type=merge --patch-file=/dev/stdin << EOF
spec:
  host:
    registrationService:
      auth:
        authClientConfigRaw: '{
                  "realm": "sandbox-dev",
                  "auth-server-url": "$RHSSO_URL/auth",
                  "ssl-required": "none",
                  "resource": "sandbox-public",
                  "clientId": "sandbox-public",
                  "public-client": true,
                  "confidential-port": 0
                }'
        authClientLibraryURL: $RHSSO_URL/auth/js/keycloak.js
        authClientPublicKeysURL: $RHSSO_URL/auth/realms/sandbox-dev/protocol/openid-connect/certs
EOF

# Restart the registration-service to ensure the new configuration is used
oc delete pods -n toolchain-host-operator --selector=name=registration-service

KEYCLOAK_ADMIN_PASSWORD=$(oc get secrets -n ${DEV_SSO_NS} credential-sandbox-dev -o jsonpath='{.data.ADMIN_PASSWORD}' | base64 -d)
printf "to login into keycloak use user 'admin' and password '%s' at '%s/auth'\n" "$KEYCLOAK_ADMIN_PASSWORD" "$RHSSO_URL"
printf "use user 'user1@user.us' with password 'user1' to login at 'https://registration-service-toolchain-host-operator.$BASE_URL'\n"