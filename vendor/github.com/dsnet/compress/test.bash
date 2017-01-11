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
	# Run test for a specific package.
	#	./test.bash bzip2 -tags debug
	X=$1
	shift
	env GOPATH=$TMPDIR:$GOPATH go test $@ $PKGDIR/$X
else
	# Run tests for all packages with test files.
	#	./test.bash -tags debug -bench .
	for X in $(find . -type d -not -path '*/\.*' -not -path '*/_*'); do
		RET=$(ls $X/*_test.go &> /dev/null; echo $?)
		if [ $RET -eq 0 ]; then
			env GOPATH=$TMPDIR:$GOPATH go test $@ $PKGDIR/$X
		fi
	done
fi
