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
    if [[ -f ${GOPATH}/src/github.com/codeready-toolchain/toolchain-cicd/${OLM_SETUP_FILE} ]]; then
        source ${GOPATH}/src/github.com/codeready-toolchain/toolchain-cicd/${OLM_SETUP_FILE}
    else
        source /dev/stdin <<< "$(curl -sSL https://raw.githubusercontent.com/${OWNER_AND_BRANCH_LOCATION}/${OLM_SETUP_FILE})"
    fi
fi

# read argument to get project root dir
read_arguments $@
set -ex
echo "arguments read"

# setup version variables based on commits so they can be used for generation process
setup_version_variables_based_on_commits

echo "versions configured"

# generate manifests
check_main_and_embedded_repos_and_generate_manifests $@ --next-version ${NEXT_CSV_VERSION}
