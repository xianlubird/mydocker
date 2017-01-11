// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"github.com/dsnet/compress/bzip2/internal/sais"
)

// The Burrows-Wheeler Transform implementation used here is based on the
// Suffix Array by Induced Sorting (SA-IS) methodology by Nong, Zhang, and Chan.
// This implementation uses the sais algorithm originally written by Yuta Mori.
//
// The SA-IS algorithm runs in O(n) and outputs a Suffix Array. There is a
// mathematical relationship between Suffix Arrays and the Burrows-Wheeler
// Transform, such that a SA can be converted to a BWT in O(n) time.
//
// References:
//	http://www.hpl.hp.com/techreports/Compaq-DEC/SRC-RR-124.pdf
//	https://github.com/cscott/compressjs/blob/master/lib/BWT.js
//	https://www.quora.com/How-can-I-optimize-burrows-wheeler-transform-and-inverse-transform-to-work-in-O-n-time-O-n-space
type burrowsWheelerTransform struct {
	// TODO(dsnet): Reduce memory allocations by caching slices.
}

func (bwt *burrowsWheelerTransform) Encode(buf []byte) (ptr int) {
	if len(buf) == 0 {
		return -1
	}

	// TODO(dsnet): Find a way to avoid the duplicate input string trick.

	// Step 1: Concatenate the input string to itself so that we can use the
	// suffix array algorithm for bzip2's variant of BWT.
	n := len(buf)
	t := append(buf, buf...)
	sa := make([]int, 2*n)
	buf2 := t[n:]

	// Step 2: Compute the suffix array (SA). The input string, t, will not be
	// modified, while the results will be written to the output, sa.
	sais.ComputeSA(t, sa)

	// Step 3: Convert the SA to a BWT. Since ComputeSA does not mutate the
	// input, we have two copies of the input; in buf and buf2. Thus, we write
	// the transformation to buf, while using buf2.
	var j int
	for _, i := range sa {
		if i < n {
			if i == 0 {
				ptr = j
				i = n
			}
			buf[j] = buf2[i-1]
			j++
		}
	}
	return ptr
}

func (bwt *burrowsWheelerTransform) Decode(buf []byte, ptr int) {
	if len(buf) == 0 {
		return
	}

	// Step 1: Compute the C array, where C[ch] reports the total number of
	// characters that precede the character ch in the alphabet.
	var c [256]int
	for _, v := range buf {
		c[v]++
	}
	var sum int
	for i, v := range c {
		sum += v
		c[i] = sum - v
	}

	// Step 2: Compute the P permutation, where P[ptr] contains the index of the
	// first byte and the index to the pointer to the index of the next byte.
	p := make([]uint32, len(buf))
	for i := range buf {
		b := buf[i]
		p[c[b]] = uint32(i)
		c[b]++
	}

	// Step 3: Follow each pointer in P to the next byte, starting with the
	// origin pointer.
	buf2 := make([]byte, len(buf))
	tPos := p[ptr]
	for i := range p {
		buf2[i] = buf[tPos]
		tPos = p[tPos]
	}
	copy(buf, buf2)
}
