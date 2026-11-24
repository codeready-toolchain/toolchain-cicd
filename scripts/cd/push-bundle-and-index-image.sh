#!/usr/bin/env bash

# get abs path of the directory where this script is located
# this way the source of the local file can work independently of where the script is executed from.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# use the olm-setup as the source
OLM_SETUP_FILE=scripts/cd/olm-setup.sh
LOCAL_OLM_SETUP_FILE=${SCRIPT_DIR}/olm-setup.sh
OWNER_AND_BRANCH_LOCATION=${OWNER_AND_BRANCH_LOCATION:-codeready-toolchain/toolchain-cicd/master}

if [[ -f ${LOCAL_OLM_SETUP_FILE} ]]; then
    echo "sourcing local olm setup file ${LOCAL_OLM_SETUP_FILE}"
    source ${LOCAL_OLM_SETUP_FILE}
else
    echo "sourcing remote olm setup file"
    source /dev/stdin <<< "$(curl -sSL https://raw.githubusercontent.com/${OWNER_AND_BRANCH_LOCATION}/${OLM_SETUP_FILE})"
fi

# read argument to get project root dir
read_arguments $@

# if the main repo is specified then reconfigure the variables so the project root points to the temp directory
if [[ -n "${MAIN_REPO_URL}"  ]]; then
    REPO_NAME_WITH_GIT=$(basename $(echo ${MAIN_REPO_URL}))
    OTHER_REPO_PATH=${OTHER_REPO_ROOT_DIR}/${REPO_NAME_WITH_GIT%.*}
    read_arguments $@ -pr ${OTHER_REPO_PATH}
fi

# retrieve the current version
CSV_LOCATION="${MANIFESTS_DIR}/*clusterserviceversion.yaml"
CURRENT_VERSION=`grep "^  version: " ${CSV_LOCATION} | awk '{print $2}'`

# set the image names variables
BUNDLE_IMAGE=quay.io/${QUAY_NAMESPACE_TO_PUSH}/${PRJ_NAME}-bundle:${BUNDLE_TAG:-${CURRENT_VERSION}}
INDEX_IMAGE=quay.io/${QUAY_NAMESPACE_TO_PUSH}/${INDEX_IMAGE_NAME}:latest

if [[ -n "${INDEX_IMAGE_TAG}" ]]; then
    INDEX_IMAGE=quay.io/${QUAY_NAMESPACE_TO_PUSH}/${INDEX_IMAGE_NAME}:${INDEX_IMAGE_TAG}
fi
FROM_INDEX_IMAGE="${INDEX_IMAGE}"

# get the current version that is in the "replaces" clause of the CSV
REPLACE_VERSION=`grep "^  replaces: " ${CSV_LOCATION} | awk '{print $2}'`
if [[ -z ${REPLACE_VERSION} ]]; then
    FROM_INDEX_IMAGE=""
fi

cd ${PRJ_ROOT_DIR}

echo "building & pushing operator bundle image ${BUNDLE_IMAGE}..."

${IMAGE_BUILDER} build --platform ${IMAGE_PLATFORM} -f bundle.Dockerfile -t ${BUNDLE_IMAGE} .
${IMAGE_BUILDER} push ${BUNDLE_IMAGE}

if [[ ${IMAGE_BUILDER} == "podman" ]]; then
    PULL_TOOL_PARAM="--pull-tool podman"
fi


if [[ -z ${GITHUB_ACTIONS} ]]; then
    ${IMAGE_BUILDER} image rm quay.io/operator-framework/upstream-opm-builder:latest || true
fi

echo "modifying & pushing operator index image ${INDEX_IMAGE}..."
TEMP_INDEX_DOCKERFILE=`mktemp`
if [[ -n ${FROM_INDEX_IMAGE} ]] && [[ `${IMAGE_BUILDER} pull "${FROM_INDEX_IMAGE}"` ]]; then
    opm index add --generate --out-dockerfile "${TEMP_INDEX_DOCKERFILE}" --bundles "${BUNDLE_IMAGE}" --build-tool ${IMAGE_BUILDER} --tag ${INDEX_IMAGE} --from-index ${FROM_INDEX_IMAGE} ${PULL_TOOL_PARAM}
else
    opm index add --generate --out-dockerfile "${TEMP_INDEX_DOCKERFILE}" --bundles "${BUNDLE_IMAGE}" --build-tool ${IMAGE_BUILDER} --tag ${INDEX_IMAGE} ${PULL_TOOL_PARAM}
fi

${IMAGE_BUILDER} build -f "${TEMP_INDEX_DOCKERFILE}" --platform "${IMAGE_PLATFORM}" -t "${INDEX_IMAGE}" .
${IMAGE_BUILDER} push "${INDEX_IMAGE}"

cd "${CURRENT_DIR}"