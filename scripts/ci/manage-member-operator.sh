#!/usr/bin/env bash

set -ex

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

REPOSITORY_NAME=member-operator
PROVIDED_REPOSITORY_PATH=${MEMBER_REPO_PATH}
get_repo
set_tags

# can be used only when the operator CSV doesn't bundle the environment information, but now we want to build bundle for both operators
#if [[ ${PUBLISH_OPERATOR} == "true" ]] && [[ -n ${BUNDLE_AND_INDEX_TAG} ]]; then
if [[ ${PUBLISH_OPERATOR} == "true" ]]; then
    push_image

    OPERATOR_IMAGE_LOC=${IMAGE_LOC}
    COMPONENT_IMAGE_LOC=$(echo ${IMAGE_LOC} | sed 's/\/member-operator/\/member-operator-webhook/')

    make -C ${REPOSITORY_PATH} publish-current-bundle ENV=${ENVIRONMENT} INDEX_IMAGE_TAG=${BUNDLE_AND_INDEX_TAG} BUNDLE_TAG=${BUNDLE_AND_INDEX_TAG} QUAY_NAMESPACE=${QUAY_NAMESPACE} COMPONENT_IMAGE=${COMPONENT_IMAGE_LOC} IMAGE=${OPERATOR_IMAGE_LOC}
fi

if [[ ${INSTALL_OPERATOR} == "true" ]]; then
#    can be used only when the operator CSV doesn't bundle the environment information, but now we want to build bundle for both operators
#    if [[ -z ${BUNDLE_AND_INDEX_TAG} ]]; then
#        BUNDLE_AND_INDEX_TAG=latest
#        QUAY_NAMESPACE=codeready-toolchain
#    fi

    OPERATOR_NAME=toolchain-member-operator
    INDEX_IMAGE_NAME=member-operator-index
    NAMESPACE=${MEMBER_NS}
    EXPECT_CRD=memberoperatorconfigs.toolchain.dev.openshift.com
    install_operator
    if [[ -n ${MEMBER_NS_2} ]]; then
        NAMESPACE=${MEMBER_NS_2}
        install_operator
    fi
fi
