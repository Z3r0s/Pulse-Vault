package vault

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxCarrierSize is the largest image/video prefix we will copy (4 GiB).
	MaxCarrierSize = 4 << 30
	eocdMinSize    = 22
	eocdMaxComment = 65535
)

// PeekCarrierPrefix reports how many leading non-ZIP bytes a container has.
// This does not require the vault password. A regular .pulsevault returns 0.
func PeekCarrierPrefix(path string) (int64, error) {
	return zipLocalHeaderOffset(path)
}

func refuseSymlinkDest(path string) error {
	lst, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if lst.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing to use a symbolic link as the vault path", ErrVault)
	}
	return nil
}

func validateCarrier(path string) error {
	lst, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: carrier file not found", ErrVault)
		}
		return fmt.Errorf("%w: cannot stat carrier: %v", ErrVault, err)
	}
	if lst.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing to use a symbolic link as carrier", ErrVault)
	}
	if !lst.Mode().IsRegular() {
		return fmt.Errorf("%w: carrier must be a regular file", ErrVault)
	}
	if lst.Size() == 0 {
		return fmt.Errorf("%w: carrier file is empty", ErrVault)
	}
	if lst.Size() > MaxCarrierSize {
		return fmt.Errorf("%w: carrier file is too large (max 4 GiB)", ErrVault)
	}
	return nil
}

func samePath(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(fa, fb)
	}
	absA, err1 := filepath.Abs(a)
	absB, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
}

// find where the zip starts (after the picture). scan EOCD from the end
// so a random PK in the image doesn't fool us.
func zipLocalHeaderOffset(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := st.Size()
	if size < eocdMinSize {
		return 0, nil
	}
	if off, ok, err := zipPrefixFromEOCD(f, size); err != nil {
		return 0, err
	} else if ok {
		return off, nil
	}
	return zipPrefixScanLocalHeader(f, size)
}

func zipPrefixFromEOCD(f *os.File, size int64) (int64, bool, error) {
	bufSize := int64(eocdMinSize + eocdMaxComment)
	if bufSize > size {
		bufSize = size
	}
	buf := make([]byte, bufSize)
	start := size - bufSize
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return 0, false, err
	}
	for i := len(buf) - eocdMinSize; i >= 0; i-- {
		if buf[i] != 'P' || buf[i+1] != 'K' || buf[i+2] != 0x05 || buf[i+3] != 0x06 {
			continue
		}
		commentLen := int(binary.LittleEndian.Uint16(buf[i+20 : i+22]))
		eocdOff := start + int64(i)
		if eocdOff+int64(eocdMinSize)+int64(commentLen) != size {
			continue
		}
		cdSize := binary.LittleEndian.Uint32(buf[i+12 : i+16])
		cdOff := binary.LittleEndian.Uint32(buf[i+16 : i+20])
		if cdSize == 0xffffffff || cdOff == 0xffffffff {
			return zipPrefixZIP64(f, eocdOff)
		}
		actualCD := eocdOff - int64(cdSize)
		prefix := actualCD - int64(cdOff)
		if prefix < 0 || prefix > eocdOff {
			return 0, false, nil
		}
		return prefix, true, nil
	}
	return 0, false, nil
}

func zipPrefixZIP64(f *os.File, eocdOff int64) (int64, bool, error) {
	const locatorSize = 20
	if eocdOff < locatorSize {
		return 0, false, nil
	}
	loc := make([]byte, locatorSize)
	if _, err := f.ReadAt(loc, eocdOff-locatorSize); err != nil {
		return 0, false, err
	}
	if loc[0] != 'P' || loc[1] != 'K' || loc[2] != 0x06 || loc[3] != 0x07 {
		return 0, false, nil
	}
	zip64Off := int64(binary.LittleEndian.Uint64(loc[8:16]))
	if zip64Off < 0 {
		return 0, false, nil
	}
	hdr := make([]byte, 56)
	if _, err := f.ReadAt(hdr, zip64Off); err != nil {
		return 0, false, err
	}
	if hdr[0] != 'P' || hdr[1] != 'K' || hdr[2] != 0x06 || hdr[3] != 0x06 {
		return 0, false, nil
	}
	cdSize := int64(binary.LittleEndian.Uint64(hdr[40:48]))
	cdOff := int64(binary.LittleEndian.Uint64(hdr[48:56]))
	actualCD := zip64Off - cdSize
	prefix := actualCD - cdOff
	if prefix < 0 {
		return 0, false, nil
	}
	return prefix, true, nil
}

func zipPrefixScanLocalHeader(f *os.File, size int64) (int64, error) {
	const marker = "PK\x03\x04"
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	buf := make([]byte, 64*1024)
	carry := []byte{}
	var offset int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			window := append(append([]byte{}, carry...), buf[:n]...)
			if idx := bytes.Index(window, []byte(marker)); idx >= 0 {
				return offset - int64(len(carry)) + int64(idx), nil
			}
			if len(window) > len(marker)-1 {
				carry = append([]byte{}, window[len(window)-(len(marker)-1):]...)
			} else {
				carry = append([]byte{}, window...)
			}
			offset += int64(n)
		}
		if errors.Is(readErr, io.EOF) || offset >= size {
			return 0, nil
		}
		if readErr != nil {
			return 0, readErr
		}
	}
}
