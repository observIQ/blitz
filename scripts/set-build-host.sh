#!/bin/bash
# This script sets BUILD_HOST_OS and BUILD_HOST_ARCH environment variables
# based on the Go build host platform
# Usage: source ./scripts/set-build-host.sh
# or: eval $(./scripts/set-build-host.sh)

# shellcheck disable=SC2034
# These variables are used by GoReleaser when
# injecting build system information into the
# binary's version command.
BUILD_HOST_OS=$(go env GOHOSTOS)
BUILD_HOST_ARCH=$(go env GOHOSTARCH)

export BUILD_HOST_OS
export BUILD_HOST_ARCH