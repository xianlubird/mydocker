#!/usr/bin/env bash
# Copyright 2015, Joe Tsai. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE.md file.

set -e

PKGDIR="github.com/dsnet/compress"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $DIR

# Setup a temporary GOPATH.
TMPDIR=$(mktemp -d 2>/dev/null || mktemp -d -t tmp)
mkdir -p $TMPDIR/src/$(dirname $PKGDIR)
ln -s $DIR $TMPDIR/src/$PKGDIR
function finish { rm -rf $TMPDIR; }
trap finish EXIT

env GOPATH=$TMPDIR:$GOPATH go run internal/tool/bench/main.go $@
