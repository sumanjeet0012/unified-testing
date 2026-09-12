#!/usr/bin/env bash
# Docker registry layer cache for hole-punch CI (GHCR).
#
# CI runners are ephemeral, so without this every run cold-rebuilds all
# images (~45 min: rust cargo + nim compile). This file shadows the `docker`
# command with a shell function that adds registry cache flags to `docker
# build` invocations only; every other subcommand passes through untouched.
# It is sourced by hole-punch/run.sh, so no other suite is affected.
#
# Opt-in via environment (the composite action sets these; local runs
# unaffected unless exported manually):
#   DOCKER_CACHE_ENABLED=true   pull per-image cache refs, use as build source
#   DOCKER_CACHE_PUSH=true      push refreshed cache refs after successful
#                               builds (daily/schedule runs; fork PRs use
#                               read-only tokens so push safely no-ops there)
#   DOCKER_CACHE_REPO=ghcr.io/libp2p/unified-testing-ci-cache
#   (single repo, one tag per image)
# Every cache operation is best-effort: any failure silently falls back to a
# plain build, so caching can never break a build.

docker_cache_ref_for_image() {
  local image_name="$1"
  local repo="${DOCKER_CACHE_REPO:-ghcr.io/libp2p/unified-testing-ci-cache}"
  local tag
  tag=$(printf '%s' "${image_name}" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9_.-]/-/g' | cut -c1-100)
  [ -z "${tag}" ] && tag="image"
  printf '%s:%s' "${repo}" "${tag}"
}

# Extracts -t/--tag <image> from a `docker build` argument list (last wins).
docker_cache_image_from_args() {
  local prev=""
  local img=""
  for arg in "$@"; do
    if [ "${prev}" = "-t" ] || [ "${prev}" = "--tag" ]; then
      img="${arg}"
    fi
    case "${arg}" in
      -t=*|--tag=*) img="${arg#*=}" ;;
    esac
    prev="${arg}"
  done
  printf '%s' "${img}"
}

docker() {
  if [ "${1:-}" != "build" ]; then
    command docker "$@"
    return $?
  fi
  shift
  if [ "${DOCKER_CACHE_ENABLED:-false}" != "true" ]; then
    command docker build "$@"
    return $?
  fi
  local image_name cache_ref
  image_name=$(docker_cache_image_from_args "$@")
  if [ -z "${image_name}" ]; then
    command docker build "$@"
    return $?
  fi
  cache_ref=$(docker_cache_ref_for_image "${image_name}")
  local cache_from_args=()
  # Only reference cache that exists (a missing ref fails some builders).
  if command docker manifest inspect "${cache_ref}" >/dev/null 2>&1; then
    echo "→ Using registry cache ${cache_ref}" >&2
    cache_from_args=(--cache-from "type=registry,ref=${cache_ref}")
  else
    echo "→ No registry cache for ${image_name} (cold build)" >&2
  fi
  # buildx with mode=max caches multi-stage layers (rust chef/nim builds);
  # plain `docker build` can only reuse final-stage layers via inline metadata.
  if [ "${DOCKER_CACHE_PUSH:-false}" = "true" ] && command docker buildx version >/dev/null 2>&1; then
    if command docker buildx build --load \
        "${cache_from_args[@]}" \
        --cache-to "type=registry,ref=${cache_ref},mode=max" \
        "$@"; then
      return 0
    fi
    echo "→ buildx cached build failed, retrying plain build" >&2
  fi
  if ! BUILDKIT_INLINE_CACHE=1 command docker build "${cache_from_args[@]}" "$@"; then
    return 1
  fi
  return 0
}
