#!/usr/bin/env bash

# Exit on error
set -ex

user_help () {
    echo "Generate ClusterServiceVersion and additional deployment files for openshift-marketplace"
    echo "options:"
    echo "-pr, --project-root      Path to the root of the project the CSV should be generated for/in"
    echo "-nv, --next-version      Semantic version of the new CSV to be created"
    echo "-ch, --channel           Channel to be used for the CSV in the package manifest"
    echo "-on, --operator-name     Name of the operator - by default it uses toolchain-{repository_name}"
    echo "-mr, --main-repo         URL of the GH repo that should be used as the main repo (for CD). The current repo should be embedded in the main one. The operator bundle should be taken from the main repository (example of the main repo: https://github.com/codeready-toolchain/host-operator)"
    echo "-er, --embedded-repo     URL of the GH repo that should be used as the embedded repo (for CD). The repository should be embedded in the current repo. The operator bundle should be taken from the current repository (example of the embedded repo: https://github.com/codeready-toolchain/registration-service)"
    echo "-orp, --other-repo-path  Path to either embedded repo or main repo - it depends on which is specified. When the parameter is used, then the script won't clone the repo from master, but will use the version from the given path."
    echo "-ori, --other-repo-image Image location of either embedded repo or main repo - it depends on which is specified."
    echo "-qn, --quay-namespace    Specify the quay namespace the CSV should be pushed to - if not used then it uses the one stored in \"\${QUAY_NAMESPACE}\" variable"
    echo "-n,  --namespace         Namespace operator should be installed in"
    echo "-td, --temp-dir          Directory that should be used for storing temporal files - by default '/tmp' is used"
    echo "-ib, --image-builder     Tool to build container images - will be used by opm. One of: [docker, podman] (default "docker")"
    echo "-iin, --index-image-name Name of the index image the bundle image should be added to."
    echo "-iit, --index-image-tag  Tag of the index image the bundle image should be added to."
    echo "-il, --image-location    Image location of the operator binary."
    echo "-bt, --bundle-tag        Tag of the bundle image."
    echo "-fr, --first-release     If set to true, then it will generate CSV without replaces clause."
    echo "-ci, --component-image   The name of the image to be used as a component of this operator."
    echo "-e,  --env               Environment name to be set in operator CSV/deployment."
    echo "-h,  --help              To show this help text"
    echo ""
    additional_help 2>/dev/null || true
    exit 0
}

read_arguments() {
    if [[ $# -lt 2 ]]
    then
        user_help
    fi

    while test $# -gt 0; do
           case "$1" in
                -h|--help)
                    user_help
                    ;;
                -pr|--project-root)
                    shift
                    PRJ_ROOT_DIR=$1
                    shift
                    ;;
                -nv|--next-version)
                    shift
                    NEXT_CSV_VERSION=$1
                    shift
                    ;;
                -ch|--channel)
                    shift
                    CHANNEL=$1
                    shift
                    ;;
                -on|--operator-name)
                    shift
                    SET_OPERATOR_NAME=$1
                    shift
                    ;;
                -mr|--main-repo)
                    shift
                    MAIN_REPO_URL=$1
                    shift
                    ;;
                -er|--embedded-repo)
                    shift
                    EMBEDDED_REPO_URL=$1
                    shift
                    ;;
                -orp|--other-repo-path)
                    shift
                    OTHER_REPO_PATH=$1
                    shift
                    ;;
                -ori|--other-repo-image)
                    shift
                    OTHER_REPO_IMAGE_LOC=$1
                    shift
                    ;;
                -qn|--quay-namespace)
                    shift
                    QUAY_NAMESPACE_TO_PUSH=$1
                    shift
                    ;;
                -n|--namespace)
                    shift
                    NAMESPACE=$1
                    shift
                    ;;
                -td|--temp-dir)
                    shift
                    TEMP_DIR=$1
                    shift
                    ;;
                -ib|--image-builder)
                    shift
                    IMAGE_BUILDER=$1
                    shift
                    ;;
                -iin|--index-image-name)
                    shift
                    INDEX_IMAGE_NAME=$1
                    shift
                    ;;
                -iit|--index-image-tag)
                    shift
                    INDEX_IMAGE_TAG=$1
                    shift
                    ;;
                -il|--image-location)
                    shift
                    IMAGE_LOCATION=$1
                    shift
                    ;;
                -bt|--bundle-tag)
                    shift
                    BUNDLE_TAG=$1
                    shift
                    ;;
                -fr|--first-release)
                    shift
                    FIRST_RELEASE=$1
                    shift
                    ;;
                -ci|--component-image)
                    shift
                    COMPONENT_IMAGE=$1
                    shift
                    ;;
                -e|--env)
                    shift
                    ENV=$1
                    shift
                    ;;
                *)
                   echo "$1 is not a recognized flag!" >> /dev/stderr
                   user_help
                   exit -1
                   ;;
          esac
    done

    if [[ -z ${PRJ_ROOT_DIR} ]]; then
        echo "--project-root parameter is not specified" >> /dev/stderr
        user_help
        exit 1;
    fi

    cd ${PRJ_ROOT_DIR}
    PRJ_ROOT_DIR=${PWD}
    cd - > /dev/null

    if [[ -n "${EMBEDDED_REPO_URL}" ]] && [[ -n "${MAIN_REPO_URL}" ]]; then
        echo "you cannot specify both parameters '--main-repo' and '--embedded-repo' at the same time - use only one" >> /dev/stderr
        user_help
        exit 1
    fi

    if [[ -z ${QUAY_NAMESPACE_TO_PUSH} ]]; then
        QUAY_NAMESPACE_TO_PUSH=${QUAY_NAMESPACE:codeready-toolchain}
    fi

    setup_variables
}

# Default version var - it has to be out of the function to make it available in help text
DEFAULT_VERSION=0.0.1

setup_variables() {
    # Version vars
    NEXT_CSV_VERSION=${NEXT_CSV_VERSION:-${DEFAULT_VERSION}}

    # Channel to be used
    CHANNEL=${CHANNEL:alpha}

    # Temporal directory
    TEMP_DIR=${TEMP_DIR:-/tmp}
    if [[ "${TEMP_DIR}" != "/tmp" ]]; then
        mkdir -p ${TEMP_DIR} || true
    fi
    OTHER_REPO_ROOT_DIR=${TEMP_DIR}/cd/other-repo

    # Image builder
    IMAGE_BUILDER=${IMAGE_BUILDER:-"docker"}

    # Files and directories related vars
    PRJ_NAME=`basename ${PRJ_ROOT_DIR}`
    OPERATOR_NAME=${SET_OPERATOR_NAME:-toolchain-${PRJ_NAME}}
    MANIFESTS_DIR=${PRJ_ROOT_DIR}/bundle/manifests
    CURRENT_DIR=${PWD}

    export GO111MODULE=on
}

generate_bundle() {
    echo "## Generating operator bundle of project '${PRJ_NAME}' ..."

    make -C ${PRJ_ROOT_DIR} bundle CHANNEL=${CHANNEL} NEXT_VERSION=${NEXT_CSV_VERSION}

    if [[ ${FIRST_RELEASE} != "true" ]] && [[ -n "${REPLACE_CSV_VERSION}" ]]; then
        REPLACE_CLAUSE="replaces: ${OPERATOR_NAME}.v${REPLACE_CSV_VERSION}"
        CSV_SED_REPLACE+=";s/^  version: /  ${REPLACE_CLAUSE}\n  version: /"
    fi

    if [[ -n "${IMAGE_IN_CSV}" ]]; then
        # digest format removed for now as it brought more pain than benefits
        # IMAGE_IN_CSV_DIGEST_FORMAT=`get_digest_format ${IMAGE_IN_CSV}`
        CSV_SED_REPLACE+=";s|REPLACE_IMAGE|${IMAGE_IN_CSV}|g;s|REPLACE_CREATED_AT|$(date -u +%FT%TZ)|g;"
    fi
    if [[ -n "${EMBEDDED_REPO_IMAGE}" ]]; then
        # digest format removed for now as it brought more pain than benefits
        # EMBEDDED_REPO_IMAGE_DIGEST_FORMAT=`get_digest_format ${EMBEDDED_REPO_IMAGE}`
        CSV_SED_REPLACE+=";s|${EMBEDDED_REPO_REPLACEMENT}|${EMBEDDED_REPO_IMAGE}|g;"
    fi
    if [[ ${PRJ_NAME} == "member-operator" ]]; then
        COMPONENT_IMAGE_URL=${COMPONENT_IMAGE:-quay.io/${QUAY_NAMESPACE_TO_PUSH}/member-operator-webhook:${GIT_COMMIT_ID}}
        # digest format removed for now as it brought more pain than benefits
        # COMPONENT_IMAGE_DIGEST_FORMAT=`get_digest_format ${COMPONENT_IMAGE_URL}`
        CSV_SED_REPLACE+=";s|REPLACE_MEMBER_OPERATOR_WEBHOOK_IMAGE|${COMPONENT_IMAGE_URL}|g;"
    fi
    if [[ "${CHANNEL}" == "staging" ]]; then
        CSV_SED_REPLACE+=";s|  annotations:|  annotations:\n    olm.skipRange: '<${NEXT_CSV_VERSION}'|g;"
    fi

    CSV_LOCATION=${MANIFESTS_DIR}/*clusterserviceversion.yaml
    replace_with_sed "${CSV_SED_REPLACE}" "${CSV_LOCATION}"
    if [[ -n "${ENV}" ]]; then
        CONFIG_ENV_FILE=${PRJ_ROOT_DIR}/deploy/env/${ENV}.yaml

        echo "enriching ${CSV_LOCATION} by params defined in ${CONFIG_ENV_FILE}"
        enrich-by-envs-from-yaml ${CSV_LOCATION} ${CONFIG_ENV_FILE}
    fi

    echo "-> Bundle generated."
}

# digest format removed for now as it brought more pain than benefits
#get_digest_format() {
#    IMG=$1
#    IMG_LOC=`echo ${IMG} | cut -d: -f1`
#
#    IMG_ORG=`echo ${IMG_LOC} | awk -F/ '{print $2}'`
#    IMG_NAME=`echo ${IMG_LOC} | awk -F/ '{print $3}'`
#    IMG_TAG=`echo ${IMG} | cut -d: -f2`
#
#    echo "Getting digest of the image ${IMG}" >> /dev/stderr
#
#    while [[ -z ${IMG_DIGEST} || "${IMG_DIGEST}" == "null" ]]; do
#		if [[ ${NEXT_WAIT_TIME} -eq 10 ]]; then
#		   echo " the digest of the image ${IMG} wasn't found" >> /dev/stderr
#		   exit 1
#		fi
#		echo -n "." >> /dev/stderr
#		(( NEXT_WAIT_TIME++ ))
#		sleep 1
#		IMG_DIGEST=`curl https://quay.io/api/v1/repository/${IMG_ORG}/${IMG_NAME} 2>/dev/null | jq -r ".tags.\"${IMG_TAG}\".manifest_digest"`
#	done
#    echo " found: ${IMG_DIGEST}" >> /dev/stderr
#
#    echo ${IMG_LOC}@${IMG_DIGEST}
#}


enrich-by-envs-from-yaml() {
    ENRICHED_CSV="${TEMP_DIR}/${OPERATOR_NAME}_${NEXT_CSV_VERSION}-enriched-file"

    ENRICH_BY_ENVS_FROM_YAML=scripts/enrich-by-envs-from-yaml.sh
    if [[ -f ${ENRICH_BY_ENVS_FROM_YAML} ]]; then
        ${ENRICH_BY_ENVS_FROM_YAML} $@ > ${ENRICHED_CSV}
    else
        if [[ -f ${GOPATH}/src/github.com/codeready-toolchain/api/${ENRICH_BY_ENVS_FROM_YAML} ]]; then
            ${GOPATH}/src/github.com/codeready-toolchain/api/${ENRICH_BY_ENVS_FROM_YAML} $@ > ${ENRICHED_CSV}
        else
            curl -sSL  https://raw.githubusercontent.com/codeready-toolchain/api/master/${ENRICH_BY_ENVS_FROM_YAML} | bash -s -- $@ > ${ENRICHED_CSV}
        fi
    fi
    cat ${ENRICHED_CSV} > $1
}

replace_with_sed() {
    TMP_CSV="${TEMP_DIR}/${OPERATOR_NAME}_${NEXT_CSV_VERSION}_replace-file"
    sed -e "$1" $2 > ${TMP_CSV}
    cat ${TMP_CSV} > $2
    rm -rf ${TMP_CSV}
}

# it takes one boolean parameter - if the other repo (either embedded or main one) should be cloned or not
setup_version_variables_based_on_commits() {
    # setup version and commit variables for the current repo
    GIT_COMMIT_ID=`git --git-dir=${PRJ_ROOT_DIR}/.git --work-tree=${PRJ_ROOT_DIR} rev-parse --short HEAD`
    PREVIOUS_GIT_COMMIT_ID=`git --git-dir=${PRJ_ROOT_DIR}/.git --work-tree=${PRJ_ROOT_DIR} rev-parse --short HEAD^`
    NUMBER_OF_COMMITS=`git --git-dir=${PRJ_ROOT_DIR}/.git --work-tree=${PRJ_ROOT_DIR} rev-list --count HEAD`

    # check if there is main repo or inner repo specified
    if [[ -n "${MAIN_REPO_URL}${EMBEDDED_REPO_URL}" ]]; then
        if [[ -z "${OTHER_REPO_PATH}" ]]; then
            # if there is, then clone the latest version of the repo to ${TEMP_DIR} dir
            if [[ -d ${OTHER_REPO_ROOT_DIR} ]]; then
                rm -rf ${OTHER_REPO_ROOT_DIR}
            fi
            mkdir -p ${OTHER_REPO_ROOT_DIR}
            git -C ${OTHER_REPO_ROOT_DIR} clone ${MAIN_REPO_URL}${EMBEDDED_REPO_URL}

            OTHER_REPO_PATH=${OTHER_REPO_ROOT_DIR}/`basename -s .git $(echo ${MAIN_REPO_URL}${EMBEDDED_REPO_URL})`
        fi


        # and set version and comit variables also for this repo
        OTHER_REPO_GIT_COMMIT_ID=`git --git-dir=${OTHER_REPO_PATH}/.git --work-tree=${OTHER_REPO_PATH} rev-parse --short HEAD`
        OTHER_REPO_NUMBER_OF_COMMITS=`git --git-dir=${OTHER_REPO_PATH}/.git --work-tree=${OTHER_REPO_PATH} rev-list --count HEAD`

        if [[ -n "${MAIN_REPO_URL}"  ]]; then
            # the other repo is main, so the number of commits and commit ID should be specified as the first one
            NEXT_CSV_VERSION="0.0.${OTHER_REPO_NUMBER_OF_COMMITS}-${NUMBER_OF_COMMITS}-commit-${OTHER_REPO_GIT_COMMIT_ID}-${GIT_COMMIT_ID}"
            REPLACE_CSV_VERSION="0.0.${OTHER_REPO_NUMBER_OF_COMMITS}-$((${NUMBER_OF_COMMITS}-1))-commit-${OTHER_REPO_GIT_COMMIT_ID}-${PREVIOUS_GIT_COMMIT_ID}"
        else
            # the other repo is inner, so the number of commits and commit ID should be specified as the second one
            NEXT_CSV_VERSION="0.0.${NUMBER_OF_COMMITS}-${OTHER_REPO_NUMBER_OF_COMMITS}-commit-${GIT_COMMIT_ID}-${OTHER_REPO_GIT_COMMIT_ID}"
            REPLACE_CSV_VERSION="0.0.$((${NUMBER_OF_COMMITS}-1))-${OTHER_REPO_NUMBER_OF_COMMITS}-commit-${PREVIOUS_GIT_COMMIT_ID}-${OTHER_REPO_GIT_COMMIT_ID}"
        fi
    else
        # there is no other repo specified - use the basic version format
        NEXT_CSV_VERSION="0.0.${NUMBER_OF_COMMITS}-commit-${GIT_COMMIT_ID}"
        REPLACE_CSV_VERSION="0.0.$((${NUMBER_OF_COMMITS}-1))-commit-${PREVIOUS_GIT_COMMIT_ID}"
    fi
    echo ${REPLACE_CSV_VERSION} ${NEXT_CSV_VERSION}
}

check_main_and_embedded_repos_and_generate_manifests() {
    #read arguments and setup variables
    read_arguments $@
    setup_variables

    IMAGE_IN_CSV=${IMAGE_LOCATION:-quay.io/${QUAY_NAMESPACE_TO_PUSH}/${PRJ_NAME}:${GIT_COMMIT_ID}}
    # check if there is main repo or inner repo specified
    if [[ -n ${MAIN_REPO_URL}${EMBEDDED_REPO_URL}  ]] && [[ -n ${OTHER_REPO_GIT_COMMIT_ID} ]]; then

        OTHER_REPO_NAME=`basename -s .git $(echo ${MAIN_REPO_URL}${EMBEDDED_REPO_URL})`

        if [[ -n "${MAIN_REPO_URL}"  ]]; then
            EMBEDDED_REPO_IMAGE=${IMAGE_IN_CSV}

            IMAGE_IN_CSV=quay.io/${QUAY_NAMESPACE_TO_PUSH}/${OTHER_REPO_NAME}:${OTHER_REPO_GIT_COMMIT_ID}
            if [[ -n "${OTHER_REPO_IMAGE_LOC}" ]]; then
                IMAGE_IN_CSV=${OTHER_REPO_IMAGE_LOC}
            fi

            EMBEDDED_REPO_REPLACEMENT=REPLACE_$(echo ${PRJ_NAME} | awk '{ print toupper($0) }' | tr '-' '_')_IMAGE
            generate_manifests $@ -pr ${OTHER_REPO_PATH}
        else
            EMBEDDED_REPO_REPLACEMENT=REPLACE_$(echo ${OTHER_REPO_NAME} | awk '{ print toupper($0) }' | tr '-' '_')_IMAGE
            EMBEDDED_REPO_IMAGE=quay.io/${QUAY_NAMESPACE_TO_PUSH}/${OTHER_REPO_NAME}:${OTHER_REPO_GIT_COMMIT_ID}
            if [[ -n "${OTHER_REPO_IMAGE_LOC}" ]]; then
                EMBEDDED_REPO_IMAGE=${OTHER_REPO_IMAGE_LOC}
            fi
            generate_manifests $@
        fi
    else
        generate_manifests $@
    fi
}

generate_manifests() {
    #read arguments and setup variables
    read_arguments $@
    setup_variables

    # generate the bundle
    generate_bundle
}
