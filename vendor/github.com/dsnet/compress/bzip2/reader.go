// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/prefix"
)

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     prefixReader
	err    error
	level  int    // The current compression level
	rdHdr  bool   // Have we read the stream header?
	blkCRC uint32 // CRC-32 IEEE of each block
	endCRC uint32 // Checksum of all blocks using bzip2's custom method

	mtf moveToFront
	bwt burrowsWheelerTransform
	rle runLengthEncoding
}

type ReaderConfig struct {
	_ struct{} // Blank field to prevent unkeyed struct literals
}

func NewReader(r io.Reader, conf *ReaderConfig) (*Reader, error) {
	zr := new(Reader)
	zr.Reset(r)
	return zr, nil
}

func (zr *Reader) Reset(r io.Reader) {
	*zr = Reader{
		rd:  zr.rd,
		mtf: zr.mtf,
		bwt: zr.bwt,
		rle: zr.rle,
	}
	zr.rd.Init(r)
	return
}

func (zr *Reader) Read(buf []byte) (int, error) {
	for {
		cnt, _ := zr.rle.Read(buf)
		if cnt > 0 {
			zr.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if zr.err != nil {
			return 0, zr.err
		}

		// Read the next chunk.
		zr.rd.Offset = zr.InputOffset
		func() {
			defer errRecover(&zr.err)
			if !zr.rdHdr {
				// Read stream header.
				if zr.rd.ReadBitsBE64(16) != hdrMagic {
					panic(ErrCorrupt)
				}
				if ver := zr.rd.ReadBitsBE64(8); ver != 'h' {
					if ver == '0' {
						panic(ErrDeprecated)
					}
					panic(ErrCorrupt)
				}
				lvl := int(zr.rd.ReadBitsBE64(8)) - '0'
				if lvl < BestSpeed || lvl > BestCompression {
					panic(ErrCorrupt)
				}
				zr.level = lvl
				zr.rdHdr = true
			}
			buf := zr.decodeBlock()
			zr.rle.Init(buf)
		}()
		var err error
		if zr.InputOffset, err = zr.rd.Flush(); err != nil {
			zr.err = err
		}
		if zr.err != nil {
			if zr.err == internal.ErrInvalid {
				zr.err = ErrCorrupt
			}
			return 0, zr.err
		}
	}
}

func (zr *Reader) Close() error {
	if zr.err == io.EOF || zr.err == ErrClosed {
		zr.rle.Init(nil) // Make sure future reads fail
		zr.err = ErrClosed
		return nil
	}
	return zr.err // Return the persistent error
}

func (zr *Reader) decodeBlock() []byte {
	if magic := zr.rd.ReadBitsBE64(48); magic != blkMagic {
		if magic == endMagic {
			// TODO(dsnet): Handle multiple bzip2 files back-to-back.
			// TODO(dsnet): Check for block and stream CRC errors.
			panic(io.EOF)
		}
		panic(ErrCorrupt)
	}
	zr.blkCRC = uint32(zr.rd.ReadBitsBE64(32))
	if zr.rd.ReadBitsBE64(1) != 0 {
		panic(ErrDeprecated)
	}

	// Read BWT related fields.
	ptr := int(zr.rd.ReadBitsBE64(24)) // BWT origin pointer

	// Read MTF related fields.
	var dictArr [256]uint8
	dict := dictArr[:0]
	bmapHi := uint16(zr.rd.ReadBits(16))
	for i := 0; i < 256; i, bmapHi = i+16, bmapHi>>1 {
		if bmapHi&1 > 0 {
			bmapLo := uint16(zr.rd.ReadBits(16))
			for j := 0; j < 16; j, bmapLo = j+1, bmapLo>>1 {
				if bmapLo&1 > 0 {
					dict = append(dict, uint8(i+j))
				}
			}
		}
	}

	// Step 1: Prefix encoding.
	syms := zr.decodePrefix(len(dict))

	// Step 2: Move-to-front transform and run-length encoding.
	zr.mtf.Init(dict, zr.level*blockSize)
	buf := zr.mtf.Decode(syms)

	// Step 3: Burrows-Wheeler transformation.
	if ptr >= len(buf) {
		panic(ErrCorrupt)
	}
	zr.bwt.Decode(buf, ptr)

	return buf
}

func (zr *Reader) decodePrefix(numSyms int) (syms []uint16) {
	numSyms += 2 // Remove 0 symbol, add RUNA, RUNB, and EOF symbols
	if numSyms < 3 {
		panic(ErrCorrupt) // Not possible to encode EOF marker
	}

	// Read information about the trees and tree selectors.
	var mtf internal.MoveToFront
	numTrees := int(zr.rd.ReadBitsBE64(3))
	if numTrees < minNumTrees || numTrees > maxNumTrees {
		panic(ErrCorrupt)
	}
	numSels := int(zr.rd.ReadBitsBE64(15))
	treeSels := make([]uint8, numSels)
	for i := range treeSels {
		sym, ok := zr.rd.TryReadSymbol(&decSel)
		if !ok {
			sym = zr.rd.ReadSymbol(&decSel)
		}
		if int(sym) >= numTrees {
			panic(ErrCorrupt)
		}
		treeSels[i] = uint8(sym)
	}
	mtf.Decode(treeSels)

	// Initialize prefix codes.
	var codes2D [maxNumTrees][maxNumSyms]prefix.PrefixCode
	var codes1D [maxNumTrees]prefix.PrefixCodes
	var trees1D [maxNumTrees]prefix.Decoder
	for i := range codes2D[:numTrees] {
		pc := codes2D[i][:numSyms]
		for j := range pc {
			pc[j].Sym = uint32(j)
		}
		codes1D[i] = pc
	}
	zr.rd.ReadPrefixCodes(codes1D[:numTrees], trees1D[:numTrees])

	// Read prefix encoded symbols of compressed data.
	var tree *prefix.Decoder
	var blkLen, selIdx int
	for {
		if blkLen == 0 {
			blkLen = numBlockSyms
			if selIdx >= len(treeSels) {
				panic(ErrCorrupt)
			}
			tree = &trees1D[treeSels[selIdx]]
			selIdx++
		}
		blkLen--
		sym, ok := zr.rd.TryReadSymbol(tree)
		if !ok {
			sym = zr.rd.ReadSymbol(tree)
		}

		if int(sym) == numSyms-1 {
			break // EOF marker
		}
		if int(sym) >= numSyms {
			panic(ErrCorrupt) // Invalid symbol used
		}
		if len(syms) >= zr.level*blockSize {
			panic(ErrCorrupt) // Block is too large
		}
		syms = append(syms, uint16(sym))
	}
	return syms
}
