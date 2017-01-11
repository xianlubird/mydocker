// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package prefix implements bit readers and writers that use prefix encoding.
package prefix

import (
	"sort"

	"github.com/dsnet/compress/internal"
)

const (
	countBits  = 5  // Number of bits to store the bit-length of the code
	symbolBits = 27 // Number of bits to store the symbol
	valueBits  = 27 // Number of bits to store the code value

	countMask = (1 << countBits) - 1
)

type PrefixCode struct {
	Sym uint32 // The symbol being mapped
	Cnt uint32 // The number times this symbol is used
	Len uint32 // Bit-length of the prefix code
	Val uint32 // Value of the prefix code (must be in 0..(1<<Len)-1)
}
type PrefixCodes []PrefixCode

type prefixCodesBySymbol []PrefixCode

func (c prefixCodesBySymbol) Len() int           { return len(c) }
func (c prefixCodesBySymbol) Less(i, j int) bool { return c[i].Sym < c[j].Sym }
func (c prefixCodesBySymbol) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type prefixCodesByCount []PrefixCode

func (c prefixCodesByCount) Len() int           { return len(c) }
func (c prefixCodesByCount) Less(i, j int) bool { return c[i].Cnt < c[j].Cnt }
func (c prefixCodesByCount) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

func (pc PrefixCodes) SortBySymbol() { sort.Sort(prefixCodesBySymbol(pc)) }
func (pc PrefixCodes) SortByCount()  { sort.Stable(prefixCodesByCount(pc)) }

// Length computes the total bit-length using the Len and Cnt fields.
func (pc PrefixCodes) Length() (nb uint) {
	for _, c := range pc {
		nb += uint(c.Len * c.Cnt)
	}
	return nb
}

// checkLengths reports whether the codes form a complete prefix tree.
func (pc PrefixCodes) checkLengths() bool {
	sum := 1 << valueBits
	for _, c := range pc {
		sum -= (1 << valueBits) >> uint(c.Len)
	}
	return sum == 0 || len(pc) == 0
}

// checkPrefixes reports whether all codes have non-overlapping prefixes.
func (pc PrefixCodes) checkPrefixes() bool {
	for i, c1 := range pc {
		for j, c2 := range pc {
			mask := uint32(1)<<c1.Len - 1
			if i != j && c1.Len <= c2.Len && c1.Val&mask == c2.Val&mask {
				return false
			}
		}
	}
	return true
}

// checkCanonical reports whether all codes are canonical.
func (pc PrefixCodes) checkCanonical() bool {
	// Rule #1: All codes of a given bit-length are consecutive values.
	var vals [valueBits + 1]PrefixCode
	for _, c := range pc {
		if c.Len > 0 {
			c.Val = internal.ReverseUint32N(c.Val, uint(c.Len))
			if vals[c.Len].Cnt > 0 && vals[c.Len].Val+1 != c.Val {
				return false
			}
			vals[c.Len].Val = c.Val
			vals[c.Len].Cnt++
		}
	}

	// Rule #2: Shorter codes lexicographically precede longer codes.
	var last PrefixCode
	for _, v := range vals {
		if v.Cnt > 0 {
			curVal := v.Val - uint32(v.Cnt) + 1
			if last.Cnt != 0 && last.Val >= curVal {
				return false
			}
			last = v
		}
	}
	return true
}

// GenerateLengths assigns non-zero bit-lengths to all codes. Codes with high
// frequency counts will be assigned shorter codes to reduce bit entropy.
// This function is used primarily by compressors.
//
// The input codes must have the Cnt field populated, be sorted by count, and
// be densely packed. Even if a code has a count of 0, a non-zero bit-length
// will still be assigned.
//
// The result will have the Len field populated. The algorithm used guarantees
// that Len <= maxBits and that it is a complete prefix tree. The resulting
// codes will be sorted by count.
func GenerateLengths(codes PrefixCodes, maxBits uint) error {
	if len(codes) <= 1 {
		if len(codes) == 1 {
			codes[0].Len = 0
		}
		return nil
	}

	// Verify that the codes are in ascending order by count.
	cntLast := codes[0].Cnt
	for _, c := range codes[1:] {
		if c.Cnt < cntLast {
			return internal.ErrInvalid // Non-monotonically increasing
		}
		cntLast = c.Cnt
	}

	// Compute the number of symbols that exist for each bit-length.
	// This uses the Huffman algorithm, and runs in O(n), but assumes that
	// codes is sorted in increasing order of frequency.
	type node struct {
		cnt uint32

		// Either n0 or c0 is set. Either n1 or c1 is set.
		n0, n1 int // Index to child nodes
		c0, c1 *PrefixCode
	}
	var nodeIdx int
	var nodeArr [1024]node // Large enough to handle most cases on the stack
	nodes := nodeArr[:]
	if len(nodes) < len(codes) {
		nodes = make([]node, len(codes)) // Number of internal nodes < number of leaves
	}
	freqs, queue := codes, nodes[:0]
	for len(freqs)+len(queue) > 1 {
		// These are the two smallest nodes at the front of freqs and queue.
		var n node
		if len(queue) == 0 || (len(freqs) > 0 && freqs[0].Cnt <= queue[0].cnt) {
			n.c0, freqs = &freqs[0], freqs[1:]
			n.cnt += n.c0.Cnt
		} else {
			n.cnt += queue[0].cnt
			n.n0 = nodeIdx // nodeIdx is same as &queue[0] - &nodes[0]
			nodeIdx++
			queue = queue[1:]
		}
		if len(queue) == 0 || (len(freqs) > 0 && freqs[0].Cnt <= queue[0].cnt) {
			n.c1, freqs = &freqs[0], freqs[1:]
			n.cnt += n.c1.Cnt
		} else {
			n.cnt += queue[0].cnt
			n.n1 = nodeIdx // nodeIdx is same as &queue[0] - &nodes[0]
			nodeIdx++
			queue = queue[1:]
		}
		queue = append(queue, n)
	}

	// Search the whole binary tree, noting when we hit each leaf node.
	var fixBits bool
	var explore func(int, uint)
	explore = func(rootIdx int, level uint) {
		root := &nodes[rootIdx]
		if root.c0 == nil {
			explore(root.n0, level+1)
		} else {
			fixBits = fixBits || (level > maxBits)
			root.c0.Len = uint32(level)
		}
		if root.c1 == nil {
			explore(root.n1, level+1)
		} else {
			fixBits = fixBits || (level > maxBits)
			root.c1.Len = uint32(level)
		}
	}
	explore(nodeIdx, 1)

	// Fix the bit-lengths if we violate the maxBits requirement.
	if fixBits {
		// Create histogram for number of symbols with each bit-length.
		var symBitsArr [valueBits + 1]uint32
		symBits := symBitsArr[:] // symBits[nb] indicates number of symbols using nb bits
		for _, c := range codes {
			for int(c.Len) >= len(symBits) {
				symBits = append(symBits, 0)
			}
			symBits[c.Len]++
		}

		// Fudge the tree such that the largest bit-length is <= maxBits.
		// This is accomplish by effectively doing a tree rotation. That is, we
		// increase the bit-length of some higher frequency code, so that the
		// bit-lengths of many lower frequency codes can be decreased.
		var treeRotate func(uint)
		treeRotate = func(nb uint) {
			if symBits[nb-1] == 0 {
				treeRotate(nb - 1)
			}
			symBits[nb-1] -= 1 // Push this node to the level below
			symBits[nb] += 3   // This level gets one node from above, two from below
			symBits[nb+1] -= 2 // Push two nodes to the level above
		}
		for i := uint(len(symBits)) - 1; i > maxBits; i-- {
			for symBits[i] > 0 {
				treeRotate(i - 1)
			}
		}

		// Assign bit-lengths to each code. Since codes is sorted in increasing
		// order of frequency, that means that the most frequently used symbols
		// should have the shortest bit-lengths.  Thus, we copy symbols to codes
		// from the back of codes first.
		cs := codes
		for nb, cnt := range symBits {
			if cnt > 0 {
				pos := len(cs) - int(cnt)
				cs2 := cs[pos:]
				for i := range cs2 {
					cs2[i].Len = uint32(nb)
				}
				cs = cs[:pos]
			}
		}
		if len(cs) != 0 {
			panic("not all codes were used up")
		}
	}

	if internal.Debug && !codes.checkLengths() {
		panic("incomplete prefix tree detected")
	}
	return nil
}

// GeneratePrefixes assigns a prefix value to all codes according to the
// bit-lengths. This function is used by both compressors and decompressors.
//
// The input codes must have the Sym and Len fields populated, be sorted by
// symbol and be densely packed. The bit-lengths of each code must be properly
// allocated, such that it forms a complete tree.
//
// The result will have the Val field populated and will produce a canonical
// prefix tree.
func GeneratePrefixes(codes PrefixCodes) error {
	if len(codes) <= 1 {
		if len(codes) == 1 {
			if codes[0].Len != 0 {
				return internal.ErrInvalid
			}
			codes[0].Val = 0
		}
		return nil
	}

	// Compute basic statistics on the symbols.
	var bitCnts [valueBits + 1]uint
	c0 := codes[0]
	bitCnts[c0.Len]++
	minBits, maxBits, symLast := c0.Len, c0.Len, c0.Sym
	for _, c := range codes[1:] {
		if c.Sym <= symLast {
			return internal.ErrInvalid // Non-unique or non-monotonically increasing
		}
		if minBits > c.Len {
			minBits = c.Len
		}
		if maxBits < c.Len {
			maxBits = c.Len
		}
		bitCnts[c.Len]++ // Histogram of bit counts
		symLast = c.Sym  // Keep track of last symbol
	}
	if minBits == 0 {
		return internal.ErrInvalid // Bit-length is too short
	}

	// Compute the next code for a symbol of a given bit length.
	var nextCodes [valueBits + 1]uint
	var code uint
	for i := minBits; i <= maxBits; i++ {
		code <<= 1
		nextCodes[i] = code
		code += bitCnts[i]
	}
	if code != 1<<maxBits {
		return internal.ErrInvalid // Tree is under or over subscribed
	}

	// Assign the code to each symbol.
	for i, c := range codes {
		codes[i].Val = internal.ReverseUint32N(uint32(nextCodes[c.Len]), uint(c.Len))
		nextCodes[c.Len]++
	}

	if internal.Debug && !codes.checkPrefixes() {
		panic("overlapping prefixes detected")
	}
	if internal.Debug && !codes.checkCanonical() {
		panic("non-canonical prefixes detected")
	}
	return nil
}

func allocUint32s(s []uint32, n int) []uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]uint32, n, n*3/2)
}

func extendSliceUint32s(s [][]uint32, n int) [][]uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	ss := make([][]uint32, n, n*3/2)
	copy(ss, s[:cap(s)])
	return ss
}
