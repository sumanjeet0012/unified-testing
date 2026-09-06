#!/usr/bin/env bash

set -euo pipefail

[ -d "/opt/homebrew/bin" ] && export PATH="/opt/homebrew/bin:$PATH"

if ! type docker_image_exists &>/dev/null; then
  source "${SCRIPT_LIB_DIR}/lib-image-building.sh"
fi

build_rendezvous_image() {
  local impl_id="$1"
  local force_rebuild="${2:-false}"

  local q=".implementations[] | select(.id == \"${impl_id}\")"
  local image_name build_context
  image_name=$(yq eval "${q} | .imageName" "${IMAGES_YAML}")
  build_context=$(yq eval "${q} | .buildContext // \"\"" "${IMAGES_YAML}")

  if [ "${force_rebuild}" != "true" ] && docker_image_exists "${image_name}"; then
    print_success "${image_name} (already built)"
    return 0
  fi

  print_message "Building ${image_name} from ${build_context}..."
  docker build -t "${image_name}" "${build_context}"
  print_success "${image_name} built"
}

build_rendezvous_images() {
  local force_rebuild="${1:-false}"
  local filter="${2:-}"

  print_header "Building Docker images..."
  indent

  readarray -t impl_ids < <(yq eval '.implementations[].id' "${IMAGES_YAML}")

  for impl_id in "${impl_ids[@]}"; do
    if [ -n "${filter}" ]; then
      local match_found=false
      IFS='|' read -ra FILTER_PATTERNS <<< "${filter}"
      for pattern in "${FILTER_PATTERNS[@]}"; do
        case "${impl_id}" in
          ${pattern}) match_found=true; break ;;
        esac
      done
      if [ "${match_found}" == "false" ]; then
        continue
      fi
    fi

    build_rendezvous_image "${impl_id}" "${force_rebuild}" || {
      unindent
      return 1
    }
  done

  unindent
}
