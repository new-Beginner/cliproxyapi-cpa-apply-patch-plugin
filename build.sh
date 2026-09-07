#!/usr/bin/env bash
set -euo pipefail

echo "========================================================"
echo "Building CLIProxyAPI Apply Patch Plugin (apply_patch)"
echo "========================================================"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

echo "Running go mod tidy..."
go mod tidy

TARGET_OS="${GOOS:-windows}"
TARGET_ARCH="${GOARCH:-amd64}"

if [ "${TARGET_OS}" = "windows" ]; then
    OUT_NAME="apply_patch.dll"
    export CGO_ENABLED=1
    if [ "$(uname -s)" != "MINGW"* ] && [ "$(uname -s)" != "MSYS"* ]; then
        export CC="x86_64-w64-mingw32-gcc"
    fi
elif [ "${TARGET_OS}" = "darwin" ]; then
    OUT_NAME="apply_patch.dylib"
    export CGO_ENABLED=1
else
    OUT_NAME="apply_patch.so"
    export CGO_ENABLED=1
fi

echo "Compiling ${OUT_NAME} for ${TARGET_OS}/${TARGET_ARCH}..."
go build -buildmode=c-shared -ldflags="-s -w" -o "${OUT_NAME}" .

rm -f apply_patch.h

echo ""
echo "[SUCCESS] ${OUT_NAME} built successfully!"
echo ""
