#!/bin/sh
set -eu

REPO="Jurci04/ctrwatch"

# detect arch
case "$(uname -m)" in
  x86_64|amd64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $(uname -m)"; exit 1 ;;
esac

# fetch latest release tag
echo "fetching latest release..."
TAG=$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "failed to fetch latest release"
  exit 1
fi
echo "found $TAG"

# download
URL="https://github.com/$REPO/releases/download/$TAG/ctrwatch_Linux_${ARCH}.tar.gz"
echo "downloading $URL..."
curl -sfL "$URL" -o /tmp/ctrwatch.tar.gz

# extract
cd /tmp
tar xzf ctrwatch.tar.gz
chmod +x ctrwatch

# install
sudo mv ctrwatch /usr/local/bin/ctrwatch
rm -f ctrwatch.tar.gz

echo "installed /usr/local/bin/ctrwatch"
ctrwatch help
