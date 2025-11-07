#!/usr/bin/env bash
set -euo pipefail

# Generate man page for blitz into docs/man/blitz.1 (and blitz.1.gz)
# Requirements: go, go-md2man (install via: make install-tools)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DOCS_DIR="${REPO_ROOT}/docs/man"
BIN_DIR="${REPO_ROOT}/dist"
CONFIG_MD="${REPO_ROOT}/docs/configuration.md"
METRICS_MD="${REPO_ROOT}/docs/metrics.md"

if ! command -v go >/dev/null 2>&1; then
  echo "go is required but not found in PATH" >&2
  exit 1
fi

mkdir -p "${DOCS_DIR}"
mkdir -p "${BIN_DIR}"

if ! go tool -n go-md2man >/dev/null 2>&1; then
  echo "go-md2man tool is required but was not found." >&2
  echo "Install it by running: make install-tools" >&2
  exit 1
fi

# Stable date for reproducible gzip
SOURCE_DATE_EPOCH="$(git -C "${REPO_ROOT}" log -1 --format=%ct 2>/dev/null || date +%s)"
export SOURCE_DATE_EPOCH
export LC_ALL=C

OUT_MAN="${DOCS_DIR}/blitz.1"
OUT_MAN_GZ="${DOCS_DIR}/blitz.1.gz"

if [ ! -f "${CONFIG_MD}" ]; then
  echo "Missing ${CONFIG_MD}" >&2
  exit 1
fi
if [ ! -f "${METRICS_MD}" ]; then
  echo "Missing ${METRICS_MD}" >&2
  exit 1
fi

TMP_MD="$(mktemp)"
trap 'rm -f "${TMP_MD}"' EXIT

# Collect component documentation files
GENERATOR_DOCS="${REPO_ROOT}/docs/generator"
OUTPUT_DOCS="${REPO_ROOT}/docs/output"

# Create a combined markdown and append the configuration, metrics, and component documents
# Strip the Table of Contents from configuration.md for man page (TOC is not useful in man pages)
{
  cat "${DOCS_DIR}/blitz.1.md"
  echo
  # Remove TOC section (from "## Table of Contents" to the blank line before "Blitz supports configuration")
  sed '/^## Table of Contents$/,/^Blitz supports configuration/{ /^Blitz supports configuration/!d; }' "${CONFIG_MD}"
  echo
  cat "${METRICS_MD}"
  echo
  echo "# Components"
  echo
  echo "## Generators"
  echo
  # Add generator docs in alphabetical order
  if [ -d "${GENERATOR_DOCS}" ]; then
    while IFS= read -r doc; do
      if [ -f "${doc}" ]; then
        cat "${doc}"
        echo
      fi
    done < <(find "${GENERATOR_DOCS}" -maxdepth 1 -name "*.md" -type f 2>/dev/null | sort)
  fi
  echo
  echo "## Outputs"
  echo
  # Add output docs in alphabetical order
  if [ -d "${OUTPUT_DOCS}" ]; then
    while IFS= read -r doc; do
      if [ -f "${doc}" ]; then
        cat "${doc}"
        echo
      fi
    done < <(find "${OUTPUT_DOCS}" -maxdepth 1 -name "*.md" -type f 2>/dev/null | sort)
  fi
} > "${TMP_MD}"

echo "Generating ${OUT_MAN} from combined markdown..."
go tool go-md2man -in "${TMP_MD}" -out "${OUT_MAN}"

echo "Compressing ${OUT_MAN} -> ${OUT_MAN_GZ} ..."
gzip -c -n "${OUT_MAN}" > "${OUT_MAN_GZ}"

echo "Man page generated at:"
echo "  ${OUT_MAN}"
echo "  ${OUT_MAN_GZ}"

exit 0


