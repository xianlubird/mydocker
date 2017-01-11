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

if [ $# -gt 0 ] && [[ ! "$1" =~ -.* ]]; then
	# Build a specific package.
	#	./build.bash bzip2 -tags debug
	X=$1
	shift
	env GOPATH=$TMPDIR:$GOPATH go build $@ $PKGDIR/$X
else
	# Build all packages with Go files.
	#	./build.bash -tags debug -bench .
	for X in $(find . -type d -not -path '*/\.*' -not -path '*/_*' -not -path '*/testdata'); do
		RET=$(ls $X/*.go &> /dev/null; echo $?)
		if [ $RET -eq 0 ]; then
			env GOPATH=$TMPDIR:$GOPATH go build $@ $PKGDIR/$X
		fi
	done
fi
