#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
CUTOVER_TEMP=

cleanup() {
  if [ -n "$CUTOVER_TEMP" ] && [ -d "$CUTOVER_TEMP" ]; then
    rm -rf -- "$CUTOVER_TEMP"
  fi
}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

run_entrypoint() {
  entrypoint=$1
  output_prefix=$2
  shift 2
  set +e
  HOME="$CUTOVER_TEMP/home" \
    XDG_CONFIG_HOME="$CUTOVER_TEMP/config" \
    XDG_STATE_HOME="$CUTOVER_TEMP/state" \
    PATH="$CUTOVER_TEMP/bin:$PATH" \
    "$entrypoint" "$@" \
    >"$output_prefix.stdout" 2>"$output_prefix.stderr"
  ENTRYPOINT_STATUS=$?
  set -e
}

assert_entrypoint_help() {
  entrypoint=$1
  output_prefix=$2
  run_entrypoint "$entrypoint" "$output_prefix" --help
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "$entrypoint help exited $ENTRYPOINT_STATUS: $(cat "$output_prefix.stderr")"
  grep -F 'Usage: ./install.sh <command> [options]' "$output_prefix.stdout" >/dev/null ||
    fail "$entrypoint help omitted compatibility usage"
  [ ! -s "$output_prefix.stderr" ] ||
    fail "$entrypoint help wrote stderr: $(cat "$output_prefix.stderr")"
}

run_wrapper_contract() {
  if grep -E '^[[:space:]]*(source|\.)[[:space:]]' "$REPOSITORY/install.sh" >/dev/null; then
    fail "install.sh still loads the Bash management implementation"
  fi
  if grep -E '(^|[;&|[:space:]])(curl|wget)([[:space:]]|$)|git[[:space:]]+clone|go[[:space:]]+run|command[[:space:]]+-v[[:space:]]+oaw' \
    "$REPOSITORY/install.sh" >/dev/null; then
    fail "install.sh downloads, builds, or searches PATH for executable code"
  fi

  release_dir=$CUTOVER_TEMP/release
  mkdir -p "$release_dir" "$CUTOVER_TEMP/home" "$CUTOVER_TEMP/config" \
    "$CUTOVER_TEMP/state" "$CUTOVER_TEMP/bin"
  cp "$REPOSITORY/install.sh" "$release_dir/install.sh"
  chmod 755 "$release_dir/install.sh"
  (cd "$REPOSITORY" && go build -o "$release_dir/oaw" ./cmd/oaw)

  assert_entrypoint_help "$release_dir/oaw" "$CUTOVER_TEMP/direct-help"
  assert_entrypoint_help "$release_dir/install.sh" "$CUTOVER_TEMP/wrapper-help"
  cmp -s "$CUTOVER_TEMP/direct-help.stdout" "$CUTOVER_TEMP/wrapper-help.stdout" ||
    fail "wrapper help differs from the colocated binary"

  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/check" check --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper check exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/check.stderr")"
  grep -F 'installed claude: not-installed' "$CUTOVER_TEMP/check.stdout" >/dev/null ||
    fail "wrapper did not forward check arguments"
  [ ! -e "$CUTOVER_TEMP/state/open-agent-workflow/runtime" ] ||
    fail "wrapper check created Runtime State"

  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/install" install --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper install exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/install.stderr")"
  [ -f "$CUTOVER_TEMP/state/open-agent-workflow/installations/user.state" ] ||
    fail "wrapper install did not create Install State"
  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/update" update --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper update exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/update.stderr")"
  grep -F 'oaw: unchanged: claude' "$CUTOVER_TEMP/update.stdout" >/dev/null ||
    fail "wrapper did not forward update arguments"
  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/uninstall" uninstall --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper uninstall exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/uninstall.stderr")"
  [ ! -e "$CUTOVER_TEMP/state/open-agent-workflow/installations/user.state" ] ||
    fail "wrapper uninstall left Install State"

  missing_dir=$CUTOVER_TEMP/missing
  mkdir -p "$missing_dir"
  cp "$REPOSITORY/install.sh" "$missing_dir/install.sh"
  chmod 755 "$missing_dir/install.sh"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'touch "$PATH_EXECUTED_SENTINEL"' \
    >"$CUTOVER_TEMP/bin/oaw"
  chmod 755 "$CUTOVER_TEMP/bin/oaw"
  PATH_EXECUTED_SENTINEL=$CUTOVER_TEMP/path-oaw-executed
  export PATH_EXECUTED_SENTINEL
  run_entrypoint "$missing_dir/install.sh" "$CUTOVER_TEMP/missing" --help
  [ "$ENTRYPOINT_STATUS" -eq 70 ] ||
    fail "missing sibling binary exited $ENTRYPOINT_STATUS instead of 70"
  [ ! -e "$PATH_EXECUTED_SENTINEL" ] ||
    fail "wrapper executed an oaw binary from PATH"

  cp "$release_dir/oaw" "$missing_dir/oaw"
  chmod 644 "$missing_dir/oaw"
  run_entrypoint "$missing_dir/install.sh" "$CUTOVER_TEMP/non-executable" --help
  [ "$ENTRYPOINT_STATUS" -eq 70 ] ||
    fail "non-executable sibling exited $ENTRYPOINT_STATUS instead of 70"

  rm -f "$missing_dir/oaw"
  cp "$release_dir/oaw" "$missing_dir/oaw.exe"
  chmod 755 "$missing_dir/oaw.exe"
  assert_entrypoint_help "$missing_dir/install.sh" "$CUTOVER_TEMP/exe-help"

  pass "install.sh is an offline colocated-binary compatibility wrapper"
}

verify_release_checksums() {
  release_output=$1
  if command -v shasum >/dev/null 2>&1; then
    (cd "$release_output" && shasum -a 256 -c SHA256SUMS >/dev/null)
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$release_output" && sha256sum -c SHA256SUMS >/dev/null)
    return
  fi
  fail "no local SHA-256 verification tool is available"
}

assert_archive_contents() {
  archive=$1
  package_name=$2
  binary_name=$3
  actual=$CUTOVER_TEMP/archive-actual
  expected=$CUTOVER_TEMP/archive-expected
  tar -tzf "$archive" | LC_ALL=C sort >"$actual"
  printf '%s\n' \
    "$package_name/" \
    "$package_name/CHANGELOG.md" \
    "$package_name/LICENSE" \
    "$package_name/README-zh.md" \
    "$package_name/README.md" \
    "$package_name/VERSION" \
    "$package_name/install.sh" \
    "$package_name/$binary_name" \
    | LC_ALL=C sort >"$expected"
  if ! cmp -s "$expected" "$actual"; then
    diff -u "$expected" "$actual" >&2 || true
    fail "release archive has unexpected contents: $archive"
  fi
}

run_release_contract() {
  release_output=$CUTOVER_TEMP/dist
  mkdir -p "$release_output"
  if ! bash "$REPOSITORY/scripts/build-release.sh" "$release_output" \
    >"$CUTOVER_TEMP/release.stdout" 2>"$CUTOVER_TEMP/release.stderr"; then
    fail "release build failed: $(cat "$CUTOVER_TEMP/release.stderr")"
  fi

  version=$(tr -d '\r\n' <"$REPOSITORY/VERSION")
  archive_count=0
  for platform in \
    darwin_amd64 darwin_arm64 \
    linux_amd64 linux_arm64 \
    windows_amd64 windows_arm64; do
    package_name=open-agent-workflow_${version}_${platform}
    archive=$release_output/$package_name.tar.gz
    [ -f "$archive" ] || fail "missing release archive: $archive"
    case "$platform" in
      windows_*) binary_name=oaw.exe ;;
      *) binary_name=oaw ;;
    esac
    assert_archive_contents "$archive" "$package_name" "$binary_name"
    archive_extract=$CUTOVER_TEMP/inspect-$platform
    mkdir -p "$archive_extract"
    tar -xzf "$archive" -C "$archive_extract"
    go version -m "$archive_extract/$package_name/$binary_name" >/dev/null 2>&1 ||
      fail "$archive does not contain a Go executable"
    archive_count=$((archive_count + 1))
  done
  [ "$archive_count" -eq 6 ] || fail "release matrix did not contain six archives"
  [ -f "$release_output/SHA256SUMS" ] || fail "release output omitted SHA256SUMS"
  [ "$(find "$release_output" -type f | wc -l | tr -d ' ')" -eq 7 ] ||
    fail "release output contains files outside the six archives and SHA256SUMS"
  verify_release_checksums "$release_output"

  current_platform=$(go env GOOS)_$(go env GOARCH)
  current_package=open-agent-workflow_${version}_${current_platform}
  current_root=$CUTOVER_TEMP/inspect-$current_platform/$current_package
  current_binary=oaw
  if [ "$(go env GOOS)" = windows ]; then
    current_binary=oaw.exe
  fi
  [ -x "$current_root/$current_binary" ] || fail "current-platform binary is not executable"
  [ -x "$current_root/install.sh" ] || fail "current-platform wrapper is not executable"
  "$current_root/$current_binary" --help >"$CUTOVER_TEMP/current-help"
  "$current_root/$current_binary" catalog validate >"$CUTOVER_TEMP/current-catalog"
  grep -F 'catalog valid' "$CUTOVER_TEMP/current-catalog" >/dev/null ||
    fail "current-platform catalog validation failed"
  bash "$current_root/install.sh" --help >"$CUTOVER_TEMP/current-wrapper-help"
  cmp -s "$CUTOVER_TEMP/current-help" "$CUTOVER_TEMP/current-wrapper-help" ||
    fail "released wrapper help differs from released binary help"

  smoke_home=$CUTOVER_TEMP/release-home
  smoke_config=$CUTOVER_TEMP/release-config
  smoke_state=$CUTOVER_TEMP/release-state
  mkdir -p "$smoke_home" "$smoke_config" "$smoke_state"
  HOME="$smoke_home" XDG_CONFIG_HOME="$smoke_config" XDG_STATE_HOME="$smoke_state" \
    "$current_root/$current_binary" check --target claude >"$CUTOVER_TEMP/current-check"
  [ ! -e "$smoke_state/open-agent-workflow/runtime" ] ||
    fail "released check created Runtime State"

  set +e
  bash "$REPOSITORY/scripts/build-release.sh" "$release_output" \
    >"$CUTOVER_TEMP/repeated.stdout" 2>"$CUTOVER_TEMP/repeated.stderr"
  repeated_status=$?
  set -e
  [ "$repeated_status" -ne 0 ] || fail "release builder overwrote existing artifacts"

  wsl_arch=$(go env GOARCH)
  case "$wsl_arch" in
    amd64|arm64) ;;
    *) fail "WSL release smoke has no archive for host architecture: $wsl_arch" ;;
  esac
  linux_archive=$release_output/open-agent-workflow_${version}_linux_${wsl_arch}.tar.gz
  set +e
  bash "$REPOSITORY/scripts/smoke-wsl.sh" "$linux_archive" \
    >"$CUTOVER_TEMP/wsl.stdout" 2>"$CUTOVER_TEMP/wsl.stderr"
  wsl_status=$?
  set -e
  if [ -r /proc/sys/kernel/osrelease ] && grep -qi microsoft /proc/sys/kernel/osrelease; then
    [ "$wsl_status" -eq 0 ] || fail "WSL smoke failed: $(cat "$CUTOVER_TEMP/wsl.stderr")"
    grep -F 'PASS:' "$CUTOVER_TEMP/wsl.stdout" >/dev/null ||
      fail "WSL smoke returned no PASS evidence"
  else
    [ "$wsl_status" -eq 77 ] || fail "non-WSL smoke returned $wsl_status instead of 77"
    grep -F 'SKIP:' "$CUTOVER_TEMP/wsl.stderr" >/dev/null ||
      fail "non-WSL smoke did not report SKIP"
  fi

  pass "release archives are offline, cross-platform, checksummed, and executable"
}

run_docker_contract() {
  release_output=$CUTOVER_TEMP/docker-release-output
  bash "$REPOSITORY/scripts/build-release.sh" "$release_output" \
    >"$CUTOVER_TEMP/docker-release.stdout" \
    2>"$CUTOVER_TEMP/docker-release.stderr" ||
    fail "Docker release build failed: $(cat "$CUTOVER_TEMP/docker-release.stderr")"

  version=$(cat "$REPOSITORY/VERSION")
  set +e
  docker_arch=$(docker version --format '{{.Server.Arch}}' 2>/dev/null)
  docker_arch_status=$?
  set -e
  if [ "$docker_arch_status" -ne 0 ]; then
    docker_arch=$(go env GOARCH)
  fi
  case "$docker_arch" in
    amd64|arm64) ;;
    *)
      printf 'SKIP: Docker Linux release smoke has no archive for architecture: %s\n' \
        "$docker_arch" >&2
      return 0
      ;;
  esac

  linux_archive=$release_output/open-agent-workflow_${version}_linux_${docker_arch}.tar.gz
  set +e
  bash "$REPOSITORY/scripts/smoke-docker.sh" "$linux_archive" \
    >"$CUTOVER_TEMP/docker.stdout" 2>"$CUTOVER_TEMP/docker.stderr"
  docker_status=$?
  set -e
  case "$docker_status" in
    0)
      grep -F 'PASS: Docker Linux release' "$CUTOVER_TEMP/docker.stdout" >/dev/null ||
        fail "Docker smoke returned no PASS evidence"
      pass "Docker Linux release smoke passed"
      ;;
    77)
      grep -F 'SKIP:' "$CUTOVER_TEMP/docker.stderr" >/dev/null ||
        fail "unavailable Docker smoke returned no SKIP evidence"
      printf '%s\n' "$(cat "$CUTOVER_TEMP/docker.stderr")" >&2
      ;;
    *) fail "Docker smoke failed with status $docker_status: $(cat "$CUTOVER_TEMP/docker.stderr")" ;;
  esac
}

trap cleanup EXIT HUP INT TERM
CUTOVER_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-cutover.XXXXXX") ||
  fail "cannot create cutover test directory"

case "${1:-all}" in
  all)
    run_wrapper_contract
    run_release_contract
    run_docker_contract
    ;;
  wrapper) run_wrapper_contract ;;
  release) run_release_contract ;;
  docker) run_docker_contract ;;
  *) fail "unknown cutover test mode: $1" ;;
esac
