package rardecode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxSfxSize = 0x100000 // maximum number of bytes to read when searching for RAR signature
	sigPrefix  = "Rar!\x1A\x07"

	fileFmt15 = iota + 1 // Version 1.5 archive file format
	fileFmt50            // Version 5.0 archive file format
)

var (
	errNoSig              = errors.New("rardecode: RAR signature not found")
	errVerMismatch        = errors.New("rardecode: volume version mistmatch")
	errCorruptHeader      = errors.New("rardecode: corrupt block header")
	errCorruptFileHeader  = errors.New("rardecode: corrupt file header")
	errBadHeaderCrc       = errors.New("rardecode: bad header crc")
	errUnknownArc         = errors.New("rardecode: unknown archive version")
	errUnknownDecoder     = errors.New("rardecode: unknown decoder version")
	errUnsupportedDecoder = errors.New("rardecode: unsupported decoder version")
	errArchiveContinues   = errors.New("rardecode: archive continues in next volume")
	errDecoderOutOfData   = errors.New("rardecode: decoder expected more data than is in packed file")

	reNew = regexp.MustCompile(`(?:(\d+)[^\.]+)*(\d+)\D*$`) // for new style rar file naming
	reOld = regexp.MustCompile(`(\d+|[^\d\.]{1,2})$`)       // for old style rar file naming
)

type readBuf []byte

func (b *readBuf) byte() byte {
	v := (*b)[0]
	*b = (*b)[1:]
	return v
}

func (b *readBuf) uint16() uint16 {
	v := uint16((*b)[0]) | uint16((*b)[1])<<8
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := uint32((*b)[0]) | uint32((*b)[1])<<8 | uint32((*b)[2])<<16 | uint32((*b)[3])<<24
	*b = (*b)[4:]
	return v
}

func (b *readBuf) bytes(n int) []byte {
	v := (*b)[:n]
	*b = (*b)[n:]
	return v
}

func (b *readBuf) uvarint() uint64 {
	var x uint64
	var s uint
	for i, n := range *b {
		if n < 0x80 {
			*b = (*b)[i+1:]
			return x | uint64(n)<<s
		}
		x |= uint64(n&0x7f) << s
		s += 7

	}
	// if we run out of bytes, just return 0
	*b = (*b)[len(*b):]
	return 0
}

// readFull wraps io.ReadFull to return io.ErrUnexpectedEOF instead
// of io.EOF when 0 bytes are read.
func readFull(r io.Reader, buf []byte) error {
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// findSig searches for the RAR signature and version at the beginning of a file.
// It searches no more than maxSfxSize bytes.
func findSig(br *bufio.Reader) (int, error) {
	for n := 0; n <= maxSfxSize; {
		b, err := br.ReadSlice(sigPrefix[0])
		n += len(b)
		if err == bufio.ErrBufferFull {
			continue
		} else if err != nil {
			if err == io.EOF {
				err = errNoSig
			}
			return 0, err
		}

		b, err = br.Peek(len(sigPrefix[1:]) + 2)
		if err != nil {
			if err == io.EOF {
				err = errNoSig
			}
			return 0, err
		}
		if !bytes.HasPrefix(b, []byte(sigPrefix[1:])) {
			continue
		}
		b = b[len(sigPrefix)-1:]

		var ver int
		switch {
		case b[0] == 0:
			ver = fileFmt15
		case b[0] == 1 && b[1] == 0:
			ver = fileFmt50
		default:
			continue
		}
		_, _ = br.ReadSlice('\x00')

		return ver, nil
	}
	return 0, errNoSig
}

// volume extends a fileBlockReader to be used across multiple
// files in a multi-volume archive
type volume struct {
	fileBlockReader
	f    *os.File      // current file handle
	br   *bufio.Reader // buffered reader for current volume file
	name string        // current volume name
	num  int           // volume number
	old  bool          // uses old naming scheme
}

// nextVolName updates name to the next filename in the archive.
func (v *volume) nextVolName() {
	var lo, hi int

	dir, file := filepath.Split(v.name)
	if v.num == 0 {
		ext := filepath.Ext(file)
		switch strings.ToLower(ext) {
		case "", ".", ".exe", ".sfx":
			file = file[:len(file)-len(ext)] + ".rar"
		}
		if a, ok := v.fileBlockReader.(*archive15); ok {
			v.old = a.old
		}
	}
	if !v.old {
		m := reNew.FindStringSubmatchIndex(file)
		if m == nil {
			v.old = true
		} else {
			lo = m[2]
			hi = m[3]
			if lo < 0 {
				lo = m[4]
				hi = m[5]
			}
		}
	}
	if v.old {
		m := reOld.FindStringSubmatchIndex(file)
		lo = m[2]
		hi = m[3]
	}
	n, err := strconv.Atoi(file[lo:hi])
	if err != nil {
		n = 0
	} else {
		n++
	}
	vol := fmt.Sprintf("%0"+fmt.Sprint(hi-lo)+"d", n)
	v.name = dir + file[:lo] + vol + file[hi:]
}

func (v *volume) next() (*fileBlockHeader, error) {
	for {
		h, err := v.fileBlockReader.next()
		if err != errArchiveContinues {
			return h, err
		}

		v.f.Close()
		v.nextVolName()
		v.f, err = os.Open(v.name) // Open next volume file
		if err != nil {
			return nil, err
		}
		v.num++
		v.br.Reset(v.f)
		ver, err := findSig(v.br)
		if err != nil {
			return nil, err
		}
		if v.version() != ver {
			return nil, errVerMismatch
		}
		v.reset(v.br) // reset fileBlockReader to use new file
	}
}

func (v *volume) Close() error {
	// may be nil if os.Open fails in next()
	if v.f == nil {
		return nil
	}
	return v.f.Close()
}

func openVolume(name, password string) (*volume, error) {
	var err error
	v := new(volume)
	v.name = name
	v.f, err = os.Open(name)
	if err != nil {
		return nil, err
	}
	v.br = bufio.NewReader(v.f)
	v.fileBlockReader, err = newFileBlockReader(v.br, password)
	if err != nil {
		v.f.Close()
		return nil, err
	}
	return v, nil
}

func newFileBlockReader(r io.Reader, pass string) (fileBlockReader, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	runes := []rune(pass)
	if len(runes) > maxPassword {
		pass = string(runes[:maxPassword])
	}
	ver, err := findSig(br)
	if err != nil {
		return nil, err
	}
	switch ver {
	case fileFmt15:
		return newArchive15(br, pass), nil
	case fileFmt50:
		return newArchive50(br, pass), nil
	}
	return nil, errUnknownArc
}
