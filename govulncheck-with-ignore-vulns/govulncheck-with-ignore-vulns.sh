#!/usr/bin/env bash

user_help () {
    echo "Runs govulncheck ignoring vulnerabilities from config file"
    echo "options:"
    echo "-cfg, --config-file      Path to the configuration file"
    echo "-h,   --help             To show this help text"
    echo ""
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
                -cfg|--config-file)
                    shift
                    CONFIG_FILE=$1
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

set -e -o pipefail

read_arguments $@

today=$(date -I)

# load vulns to ignore from $CONFIG_FILE using yq
ignoreVulns="$(yq -o=json eval '.ignore' "$CONFIG_FILE")"
echo ignoreVulns: $ignoreVulns

# run govulncheck
echo running govulncheck...
json="$(govulncheck -json ./...)"

# extract vulns reported by govulncheck
vulns="$(jq <<<"$json" -cs '
	(
		map(
			.osv // empty
			| { key: .id, value: . }
		)
		| from_entries
	) as $meta

	| (
		map(.finding) 
		| map(select((.trace[0].function // "") != ""))
		| map({ key: .osv, value: .trace[0].version })
		| from_entries
	) as $found_versions

	| map(
		.finding
		| select((.trace[0].function // "") != "")
		| .osv
	)
	| unique
	| map(
		$meta[.] + { found_in_version: ($found_versions[.] // "N/A") }
	)
')"

echo vulnerabilities reported by govulncheck $vulns

# filtering the vulnerabilities to ignore
filtered="$(jq <<<"$vulns" -c --arg today "$today" --arg ignoreVulns "$ignoreVulns" '
  ($ignoreVulns | fromjson) as $ignore
  | map(select(
      .id as $id
      | $ignore | map(select(.id == $id and .expires > $today)) | length == 0
  ))
')"

echo vulnerabilities filtered $filtered

results="$(jq <<<"$filtered" -r 'map(
  "- \(.id) (\(.database_specific.url))\n\t\(.details | gsub("\n"; "\n\t"))\n\tPackage: \(
    if .affected[0].package.name == "stdlib" then
      .affected[0].ecosystem_specific.imports[0].path // "N/A"
    else
      .affected[0].package.name // "N/A"
    end
  )\n\tFound in: \(
    if .affected[0].package.name == "stdlib" then
      "go@" + ((.found_in_version // "N/A") | sub("^v"; ""))
    else
      .found_in_version // "N/A"
    end
  )\n\tFixed in: \(
    if .affected[0].package.name == "stdlib" then
      "go@" + (.affected[0].ranges[0].events | map(select(.fixed != null)) | .[0].fixed // "N/A")
    else
      .affected[0].ranges[0].events | map(select(.fixed != null)) | .[0].fixed // "N/A"
    end
  )"
) | join("\n\n")')"


if [ -z "$results" ]; then
	printf 'No vulnerabilities found.\n'
	exit 0
else
	printf 'Vulnerabilities found:\n'
	printf '%s\n' "$results"
	exit 1
fi