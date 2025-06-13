package utils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	_AT_NULL  = 0
	_AT_ENTRY = 9
)

// EntryPointFromAuxv searches the elf auxiliary vector for the entry point
// address.
// For a description of the auxiliary vector (auxv) format see:
// System V Application Binary Interface, AMD64 Architecture Processor
// Supplement, section 3.4.3.
// System V Application Binary Interface, Intel386 Architecture Processor
// Supplement (fourth edition), section 3-28.
func EntryPointFromAuxv(auxv []byte, ptrSize int) uint64 {
	rd := bytes.NewBuffer(auxv)

	for {
		tag, err := readUintRaw(rd, binary.LittleEndian, ptrSize)
		if err != nil {
			return 0
		}
		val, err := readUintRaw(rd, binary.LittleEndian, ptrSize)
		if err != nil {
			return 0
		}

		switch tag {
		case _AT_NULL:
			return 0
		case _AT_ENTRY:
			return val
		}
	}
}

// readUintRaw reads an integer of ptrSize bytes, with the specified byte order, from reader.
func readUintRaw(reader io.Reader, order binary.ByteOrder, ptrSize int) (uint64, error) {
	switch ptrSize {
	case 4:
		var n uint32
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return uint64(n), nil
	case 8:
		var n uint64
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("not supported ptr size %d", ptrSize)
}

func FindExecutable(path string, pid int) string {
	if path == "" {
		path = fmt.Sprintf("/proc/%d/exe", pid)
	}
	return path
}
