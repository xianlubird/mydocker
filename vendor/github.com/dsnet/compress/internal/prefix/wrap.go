// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// For some of the common Readers, we wrap and extend them to satisfy the
// compress.BufferedReader interface to improve performance.

type buffer struct {
	*bytes.Buffer
}

type bytesReader struct {
	*bytes.Reader
	limRd   io.LimitedReader
	scratch [512]byte
}

type stringReader struct {
	*strings.Reader
	limRd   io.LimitedReader
	scratch [512]byte
}

func (r *buffer) Buffered() int {
	return r.Len()
}

func (r *buffer) Peek(n int) ([]byte, error) {
	b := r.Bytes()
	if len(b) >= n {
		return b[:n], nil
	}
	return b, io.EOF
}

func (r *buffer) Discard(n int) (int, error) {
	b := r.Next(n)
	if len(b) == n {
		return n, nil
	}
	return len(b), io.EOF
}

func (r *bytesReader) Buffered() int {
	if r.Len() > len(r.scratch) {
		return len(r.scratch)
	}
	return r.Len()
}

func (r *bytesReader) Peek(n int) ([]byte, error) {
	if n > len(r.scratch) {
		return nil, io.ErrShortBuffer
	}
	pos, _ := r.Seek(0, os.SEEK_CUR) // Get current position, never fails
	cnt, err := r.ReadAt(r.scratch[:n], pos)
	return r.scratch[:cnt], err
}

func (r *bytesReader) Discard(n int) (int, error) {
	r.limRd = io.LimitedReader{R: r, N: int64(n)}
	n64, err := io.CopyBuffer(ioutil.Discard, &r.limRd, r.scratch[:])
	if err == nil && n64 < int64(n) {
		err = io.EOF
	}
	return int(n64), err
}

func (r *stringReader) Buffered() int {
	if r.Len() > len(r.scratch) {
		return len(r.scratch)
	}
	return r.Len()
}

func (r *stringReader) Peek(n int) ([]byte, error) {
	if n > len(r.scratch) {
		return nil, io.ErrShortBuffer
	}
	pos, _ := r.Seek(0, os.SEEK_CUR) // Get current position, never fails
	cnt, err := r.ReadAt(r.scratch[:n], pos)
	return r.scratch[:cnt], err
}

func (r *stringReader) Discard(n int) (int, error) {
	r.limRd = io.LimitedReader{R: r, N: int64(n)}
	n64, err := io.CopyBuffer(ioutil.Discard, &r.limRd, r.scratch[:])
	if err == nil && n64 < int64(n) {
		err = io.EOF
	}
	return int(n64), err
}
