#!/bin/sh
set -ex

# I am fetching flow from github releases.
#
# To install latest version:
# ./get-flow.sh
#
# To install by specific version:
# ./get-flow.sh v0.1.50

TAR_FILE="flow.tar.gz"
RELEASES_URL="https://github.com/flow-lab/flow/releases"
DEST="."

SEM_VERSION="latest"
if [[ ! -z "$1" ]]; then
  SEM_VERSION="$1"
fi

get_version() {
  curl --fail -sL -o /dev/null -w %{url_effective} "$RELEASES_URL/$SEM_VERSION" |
    rev |
    cut -f1 -d'/'|
    rev
}

download() {
  test -z "$VERSION" && VERSION="$(get_version)"
  test -z "$VERSION" && {
    echo "Unable to get flow version." >&2
    exit 1
  }
  rm -f "$TAR_FILE"
  curl -s -L -o "$TAR_FILE" \
    "$RELEASES_URL/download/$VERSION/flow_$(uname -s)_$(uname -m).tar.gz"
}

download
tar -xf "$TAR_FILE" -C "$DEST"
