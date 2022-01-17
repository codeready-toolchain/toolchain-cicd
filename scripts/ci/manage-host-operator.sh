#!/usr/bin/env bash

user_help () {
    echo "Publishes host operator to quay and deploys it to an OpenShift cluster"
    echo "options:"
    echo "-po, --publish-operator  Builds and pushes the operator to quay"
    echo "-qn, --quay-namespace    Quay namespace the images should be pushed to"
    echo "-io, --install-operator  Installs the operator to an OpenShift cluster"
    echo "-hn, --host-namespace    Namespace the operator should be installed to"
    echo "-hr, --host-repo-path    Path to the host operator repo"
    echo "-rr, --reg-repo-path     Path to the registation service repo"
    echo "-ds, --date-suffix       Date suffix to be added to some resources that are created"
    echo "-dl, --deploy-latest     Deploy the latest version of operator"
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
                -po|--publish-operator)
                    shift
                    PUBLISH_OPERATOR=$1
                    shift
                    ;;
                -qn|--quay-namespace)
                    shift
                    QUAY_NAMESPACE=$1
                    shift
                    ;;
                -io|--install-operator)
                    shift
                    INSTALL_OPERATOR=$1
                    shift
                    ;;
                -hn|--host-namespace)
                    shift
                    HOST_NS=$1
                    shift
                    ;;
                -hr|--host-repo-path)
                    shift
                    HOST_REPO_PATH=$1
                    shift
                    ;;
                -rr|--reg-path)
                    shift
                    REG_REPO_PATH=$1
                    shift
                    ;;
                -ds|--date-suffix)
                    shift
                    DATE_SUFFIX=$1
                    shift
                    ;;
                -dl|--deploy-latest)
                    shift
                    DEPLOY_LATEST=$1
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

set -e

read_arguments $@

if [[ -n "${CI}" ]]; then
    set -ex
else
    set -e
fi

MANAGE_OPERATOR_FILE=scripts/ci/manage-operator.sh
OWNER_AND_BRANCH_LOCATION=${OWNER_AND_BRANCH_LOCATION:-codeready-toolchain/toolchain-cicd/master}

if [[ -f ${MANAGE_OPERATOR_FILE} ]]; then
    source ${MANAGE_OPERATOR_FILE}
else
    if [[ -f ${GOPATH}/src/github.com/codeready-toolchain/toolchain-cicd/${MANAGE_OPERATOR_FILE} ]]; then
        source ${GOPATH}/src/github.com/codeready-toolchain/toolchain-cicd/${MANAGE_OPERATOR_FILE}
    else
        source /dev/stdin <<< "$(curl -sSL https://raw.githubusercontent.com/${OWNER_AND_BRANCH_LOCATION}/${MANAGE_OPERATOR_FILE})"
    fi
fi

if [[ DEPLOY_LATEST != "true" ]] && [[ -n "${CI}${REG_REPO_PATH}${HOST_REPO_PATH}" ]]; then
    REPOSITORY_NAME=registration-service
    PROVIDED_REPOSITORY_PATH=${REG_REPO_PATH}
    get_repo
    set_tags

    if [[ ${PUBLISH_OPERATOR} == "true" ]]; then
        push_image
        REG_SERV_IMAGE_LOC=${IMAGE_LOC}
        REG_REPO_PATH=${REPOSITORY_PATH}
    fi


    REPOSITORY_NAME=host-operator
    PROVIDED_REPOSITORY_PATH=${HOST_REPO_PATH}
    get_repo
    set_tags

    if [[ ${PUBLISH_OPERATOR} == "true" ]]; then
        push_image
        OPERATOR_IMAGE_LOC=${IMAGE_LOC}
        make -C ${REPOSITORY_PATH} publish-current-bundle INDEX_IMAGE_TAG=${BUNDLE_AND_INDEX_TAG} BUNDLE_TAG=${BUNDLE_AND_INDEX_TAG} QUAY_NAMESPACE=${QUAY_NAMESPACE} OTHER_REPO_PATH=${REG_REPO_PATH} OTHER_REPO_IMAGE_LOC=${REG_SERV_IMAGE_LOC} IMAGE=${OPERATOR_IMAGE_LOC}
    fi
else
    INDEX_IMAGE_LOC="quay.io/codeready-toolchain/host-operator-index:latest"
fi

if [[ ${INSTALL_OPERATOR} == "true" ]]; then
    OPERATOR_NAME=toolchain-host-operator
    INDEX_IMAGE_NAME=host-operator-index
    NAMESPACE=${HOST_NS}
    EXPECT_CRD=toolchainconfigs.toolchain.dev.openshift.com
    install_operator
fi
