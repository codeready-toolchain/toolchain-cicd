#!/usr/bin/env bash

# Exit on error
set -e

user_help () {
    echo ""
    echo "Usage: enrich-by-envs-from-yaml.sh [path/to/target/yaml/file/to/be/enriched] [path/to/source/yaml/file/containing/configuration/data]"
    echo ""
    echo "enrich-by-envs-from-yaml.sh adds fields that will set up environment variables for a deployment. The variables are taken from the conf yaml file specified as the source."
    echo ""
    echo "Examples:"
    echo "   ./scripts/enrich-by-envs-from-yaml.sh ./path/to/csv.yaml ./path/to/e2e-test.yaml"
    echo "          - Where e2e-test.yaml contain:"
    echo ""
    echo "--- e2e-test.yaml ------------------------------------------------------------"
    echo "registration-service:
  environment: 'e2e-tests'
  auth-client:
    library-url: 'https://sso.prod-preview.openshift.io/auth/js/keycloak.js'"
    echo "------------------------------------------------------------------------------"
    echo ""
    echo "             then the script will append after the first occurrence of 'env:' inside of csv.yaml file:"
    echo ""
    echo "--- csv.yaml ----------------------------------------------------------------------"
    echo " - name: REGISTRATION_SERVICE_AUTH_CLIENT_LIBRARY_URL
   value: https://sso.redhat.com/auth/js/keycloak.js
 - name: REGISTRATION_SERVICE_ENVIRONMENT
   value: prod"
   echo "------------------------------------------------------------------------------------"
    echo ""
    exit 0
}

keys_values_in_path() {
    local LOCATION_PATH="$1"
    local VAR_BASE_NAME="$2"

    FOUND_KEYS=`cat ${SOURCE_YAML_FILE_PATH} | yq "${LOCATION_PATH}" | yq 'keys?'`

    if [[ -z ${FOUND_KEYS} ]]; then
        add_key_value_pair "${LOCATION_PATH}" "${VAR_BASE_NAME}"
    else
        for KEY in `yq '.[]' <<< ${FOUND_KEYS}`;
        do
            KEY_VAR_NAME=$(to_var_name ${KEY})
            VAR_BASE_NAME_WITH_KEY="${VAR_BASE_NAME}_${KEY_VAR_NAME}"
            if [[ -z ${VAR_BASE_NAME} ]]; then
                VAR_BASE_NAME_WITH_KEY="${KEY_VAR_NAME}"
            fi
            keys_values_in_path "${LOCATION_PATH}[${KEY}]" "${VAR_BASE_NAME_WITH_KEY}"
        done
    fi
}

to_var_name() {
    echo ${1} | awk '{ print toupper($0) }' | tr '-' '_'  | sed 's/\"//g'
}

add_key_value_pair() {
    local LOCATION_PATH="$1"
    local VAR_KEY_NAME="$2"

    RESULT+="\n"
    RESULT+="${INDENTATION}- name: ${VAR_KEY_NAME}\n"
    VALUE=`cat ${SOURCE_YAML_FILE_PATH} | yq "${LOCATION_PATH}" | sed -e 's/^"//;s/"$//'`
    RESULT+="${INDENTATION}  value: '${VALUE}'"
}

if [[ -z $1 ]]; then
    echo "The path to the target yaml file is not specified" >> /dev/stderr
    user_help
    exit 1;
fi

if [[ -z $2 ]]; then
    echo "The path to the config yaml file is not specified" >> /dev/stderr
    user_help
    exit 1;
fi

TARGET_YAML_FILE_PATH=$1
SOURCE_YAML_FILE_PATH=$2

if [[ ! -f ${SOURCE_YAML_FILE_PATH} ]]; then
    echo "there is no file found at the path that should point to the yaml file containing configuration data ${SOURCE_YAML_FILE_PATH}" >> /dev/stderr
    cat ${TARGET_YAML_FILE_PATH}
else
    if [[ -z $(command -v yq) ]]; then
        echo "The binary yq is not available. To get the installation instructions please visit https://github.com/kislyuk/yq#installation" >> /dev/stderr
        exit 1;
    fi

    INDENTATION=`grep -m 1 "env:" ${TARGET_YAML_FILE_PATH} | sed 's/env://'`

    keys_values_in_path . ""

    SED_REPLACEMENT="s|env:|env:${RESULT}|"
    sed "${SED_REPLACEMENT}" ${TARGET_YAML_FILE_PATH}
fi
