#!/usr/bin/env bash
# Source this from publish-operators-for-e2e-tests when build-only is true.
#
# The publish scripts keep calling podman push. This puts a podman wrapper
# first on PATH so those calls do not reach Quay:
# - podman push records the image name and does not push
# - podman pull succeeds when that image is already local, so opm can build
#   the index from the bundle that was just built
# - every other podman command runs the real podman
#
# Recorded names are written to ${GITHUB_WORKSPACE}/e2e-images/images.txt.
# The action saves that list into one archive after make returns.

set -euo pipefail

REAL_PODMAN=$(command -v podman)
LIST_DIR="${GITHUB_WORKSPACE}/e2e-images"
rm -rf "${LIST_DIR}"
mkdir -p "${LIST_DIR}"
WRAP_DIR=$(mktemp -d)

# The wrapper is a separate podman executable. Publish makefiles and opm
# find it because this directory is placed first on PATH below.
cat > "${WRAP_DIR}/podman" << EOF
#!/usr/bin/env bash
set -euo pipefail
real="${REAL_PODMAN}"
list="${LIST_DIR}/images.txt"
cmd="\${1:-}"
shift || true
# Record the image instead of pushing. The last argument is the image
# reference (podman push [options] IMAGE). make still thinks the push worked.
if [[ "\${cmd}" == "push" ]]; then
  image="\${@: -1}"
  if [[ -n "\${image}" ]]; then
    printf '%s\n' "\${image}" >> "\${list}"
  fi
  exit 0
fi
# opm runs "podman pull" for the bundle that was just built. That tag is not
# on Quay yet. If it is already local, print its id and succeed. Callers such
# as push-bundle-and-index-image.sh treat empty pull output as a failed pull.
if [[ "\${cmd}" == "pull" ]]; then
  image="\${@: -1}"
  if [[ -n "\${image}" ]] && "\${real}" image exists "\${image}"; then
    "\${real}" image inspect --format '{{.Id}}' "\${image}"
    exit 0
  fi
fi
# build, save, and everything else must use the real podman, not this wrapper.
exec "\${real}" "\${cmd}" "\$@"
EOF
chmod +x "${WRAP_DIR}/podman"
# Later commands in this shell, including make and opm, use the wrapper.
export PATH="${WRAP_DIR}:${PATH}"

# Called after make. Writes one archive containing every recorded image.
save_e2e_image_archive() {
  local list="${LIST_DIR}/images.txt"
  if [[ ! -s "${list}" ]]; then
    echo "no images were recorded for the e2e archive" >&2
    exit 1
  fi
  # Drop blank and repeated lines, then pass each remaining reference as its own argument.
  local images
  mapfile -t images < <(awk 'NF && !seen[$0]++' "${list}")
  "${REAL_PODMAN}" save --multi-image-archive -o "${LIST_DIR}/e2e-images.tar" "${images[@]}"
}
