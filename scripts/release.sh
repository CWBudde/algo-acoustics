#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || printf unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
TARGET_OS="${GOOS:-$(go env GOOS)}"
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"

if [[ ! $VERSION =~ ^[0-9A-Za-z][0-9A-Za-z._+-]*$ ]]; then
	echo "VERSION contains characters that are unsafe in artifact names: $VERSION" >&2
	exit 2
fi

if [[ ! $TARGET_OS =~ ^[0-9a-z]+$ || ! $TARGET_ARCH =~ ^[0-9a-z]+$ ]]; then
	echo "GOOS and GOARCH must contain only lowercase letters and digits" >&2
	exit 2
fi

if [[ ! $COMMIT =~ ^[0-9A-Za-z._+-]+$ || $BUILD_DATE =~ [[:space:]] ]]; then
	echo "COMMIT and BUILD_DATE must be safe linker values without whitespace" >&2
	exit 2
fi

export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git -C "$ROOT_DIR" show -s --format=%ct HEAD 2>/dev/null || date +%s)}"

readonly METADATA_PACKAGE="github.com/cwbudde/algo-acoustics/internal/buildinfo"
readonly LDFLAGS="-s -w -X ${METADATA_PACKAGE}.Version=${VERSION} -X ${METADATA_PACKAGE}.Commit=${COMMIT} -X ${METADATA_PACKAGE}.BuildDate=${BUILD_DATE}"

checksum_file() {
	local file=$1
	local directory
	local filename
	directory="$(dirname "$file")"
	filename="$(basename "$file")"

	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$directory" && sha256sum "$filename" >"${filename}.sha256")
	else
		(cd "$directory" && shasum -a 256 "$filename" >"${filename}.sha256")
	fi
}

archive_directory() {
	local source_dir=$1
	local archive_base=$2
	local parent_dir
	local source_name
	parent_dir="$(dirname "$source_dir")"
	source_name="$(basename "$source_dir")"

	if tar --help 2>&1 | grep -q -- '--sort'; then
		tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner \
			-C "$parent_dir" -czf "${archive_base}.tar.gz" "$source_name"
	else
		tar -C "$parent_dir" -czf "${archive_base}.tar.gz" "$source_name"
	fi
	(
		cd "$parent_dir"
		TZ=UTC zip -X -q -r "${archive_base}.zip" "$source_name"
	)
	checksum_file "${archive_base}.tar.gz"
	checksum_file "${archive_base}.zip"
}

copy_distribution_context() {
	local destination=$1
	cp "$ROOT_DIR/README.md" "$ROOT_DIR/LICENSE" "$destination/"
	cp -R "$ROOT_DIR/docs" "$ROOT_DIR/examples" "$destination/"
	mkdir -p "$destination/testdata"
	cp -R "$ROOT_DIR/testdata/rooms" "$ROOT_DIR/testdata/interop" "$destination/testdata/"
}

release_cli() {
	local package_name="algo-acoustics-${VERSION}-${TARGET_OS}-${TARGET_ARCH}"
	local stage="$DIST_DIR/.stage/$package_name"
	local executable_suffix=""
	if [[ $TARGET_OS == windows ]]; then
		executable_suffix=".exe"
	fi

	rm -rf "$stage"
	mkdir -p "$stage/bin"
	for command in roomir roomplot roombench; do
		CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" \
			go build -trimpath -buildvcs=false -ldflags="$LDFLAGS" \
			-o "$stage/bin/${command}${executable_suffix}" "$ROOT_DIR/cmd/$command"
	done
	copy_distribution_context "$stage"
	archive_directory "$stage" "$DIST_DIR/$package_name"
}

release_web() {
	local package_name="algo-acoustics-web-${VERSION}"
	local stage="$DIST_DIR/.stage/$package_name"

	rm -rf "$stage"
	mkdir -p "$stage"
	BUILD_LDFLAGS="$LDFLAGS" "$ROOT_DIR/web/build-wasm.sh"
	cp -R "$ROOT_DIR/web/." "$stage/"
	rm -rf "$stage/wasm"
	rm -f "$stage/build-wasm.sh" "$stage"/*.test.mjs
	archive_directory "$stage" "$DIST_DIR/$package_name"
}

release_regression() {
	local package_name="algo-acoustics-regression-${VERSION}-${TARGET_OS}-${TARGET_ARCH}"
	local stage="$DIST_DIR/.stage/$package_name"
	local executable_suffix=""
	if [[ $TARGET_OS == windows ]]; then
		executable_suffix=".exe"
	fi

	rm -rf "$stage"
	mkdir -p "$stage/bin" "$stage/testdata"
	CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" \
		go build -trimpath -buildvcs=false -ldflags="$LDFLAGS" \
		-o "$stage/bin/roombench${executable_suffix}" "$ROOT_DIR/cmd/roombench"
	cp -R "$ROOT_DIR/testdata/regression" "$ROOT_DIR/testdata/rooms" "$stage/testdata/"
	cp "$ROOT_DIR/scripts/release-regression.md" "$stage/README.md"
	archive_directory "$stage" "$DIST_DIR/$package_name"
}

mkdir -p "$DIST_DIR/.stage"

case "${1:-all}" in
	cli)
		release_cli
		;;
	web)
		release_web
		;;
	regression)
		release_regression
		;;
	all)
		release_cli
		release_web
		release_regression
		;;
	*)
		echo "usage: $0 [all|cli|web|regression]" >&2
		exit 2
		;;
esac

echo "Release artifacts written to $DIST_DIR"
