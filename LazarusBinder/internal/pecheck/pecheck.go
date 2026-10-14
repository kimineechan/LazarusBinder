// Package pecheck bounds untrusted PE metadata before using the standard parser.
package pecheck

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

func Parse(file *os.File) (*pe.File, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() < 64 || stat.Size() > 1<<30 {
		return nil, fmt.Errorf("select a regular Windows file no larger than 1 GiB")
	}
	var dos [64]byte
	if _, err = file.ReadAt(dos[:], 0); err != nil {
		return nil, err
	}
	if string(dos[:2]) != "MZ" {
		return nil, fmt.Errorf("missing Windows executable header")
	}
	offset := int64(binary.LittleEndian.Uint32(dos[60:]))
	if offset < 64 || offset > stat.Size()-24 {
		return nil, fmt.Errorf("invalid PE header location")
	}
	var head [24]byte
	if _, err = file.ReadAt(head[:], offset); err != nil {
		return nil, err
	}
	if string(head[:4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("missing PE signature")
	}
	sections := uint64(binary.LittleEndian.Uint16(head[6:8]))
	optionalSize := uint64(binary.LittleEndian.Uint16(head[20:22]))
	if sections == 0 || sections > 256 || optionalSize > 4096 || uint64(offset)+24+optionalSize+sections*40 > uint64(stat.Size()) {
		return nil, fmt.Errorf("invalid or excessive PE section metadata")
	}
	symbolOffset := uint64(binary.LittleEndian.Uint32(head[12:16]))
	symbolCount := uint64(binary.LittleEndian.Uint32(head[16:20]))
	if symbolCount > 1<<20 {
		return nil, fmt.Errorf("PE symbol table is too large")
	}
	if symbolOffset != 0 {
		stringOffset := symbolOffset + symbolCount*18
		if stringOffset+4 > uint64(stat.Size()) {
			return nil, fmt.Errorf("invalid PE symbol table bounds")
		}
		var sizeBytes [4]byte
		if _, err = file.ReadAt(sizeBytes[:], int64(stringOffset)); err != nil {
			return nil, err
		}
		length := uint64(binary.LittleEndian.Uint32(sizeBytes[:]))
		if length < 4 || length > 32<<20 || stringOffset+length > uint64(stat.Size()) {
			return nil, fmt.Errorf("invalid or excessive PE string table")
		}
	}
	// These tools need headers and section locations only. Do not decode arbitrary
	// COFF symbols or long symbol names from an executable being inspected.
	result := &pe.File{}
	if err := binary.Read(bytes.NewReader(head[4:]), binary.LittleEndian, &result.FileHeader); err != nil {
		return nil, err
	}
	optional := make([]byte, optionalSize)
	if _, err = file.ReadAt(optional, offset+24); err != nil {
		return nil, err
	}
	if len(optional) < 2 {
		return nil, fmt.Errorf("missing optional PE header")
	}
	padded := make([]byte, 240)
	copy(padded, optional)
	switch binary.LittleEndian.Uint16(optional[:2]) {
	case 0x10b:
		if optionalSize < 96 {
			return nil, fmt.Errorf("truncated PE32 header")
		}
		h := &pe.OptionalHeader32{}
		if err := binary.Read(bytes.NewReader(padded), binary.LittleEndian, h); err != nil {
			return nil, err
		}
		if uint64(h.NumberOfRvaAndSizes)*8 > optionalSize-96 {
			return nil, fmt.Errorf("invalid PE32 data directory count")
		}
		result.OptionalHeader = h
	case 0x20b:
		if optionalSize < 112 {
			return nil, fmt.Errorf("truncated PE32+ header")
		}
		h := &pe.OptionalHeader64{}
		if err := binary.Read(bytes.NewReader(padded), binary.LittleEndian, h); err != nil {
			return nil, err
		}
		if uint64(h.NumberOfRvaAndSizes)*8 > optionalSize-112 {
			return nil, fmt.Errorf("invalid PE32+ data directory count")
		}
		result.OptionalHeader = h
	default:
		return nil, fmt.Errorf("unsupported PE optional header")
	}
	for i := uint64(0); i < sections; i++ {
		var raw [40]byte
		if _, err = file.ReadAt(raw[:], offset+24+int64(optionalSize+i*40)); err != nil {
			return nil, err
		}
		h := pe.SectionHeader{Name: string(bytes.TrimRight(raw[:8], "\x00")), VirtualSize: binary.LittleEndian.Uint32(raw[8:12]), VirtualAddress: binary.LittleEndian.Uint32(raw[12:16]), Size: binary.LittleEndian.Uint32(raw[16:20]), Offset: binary.LittleEndian.Uint32(raw[20:24]), Characteristics: binary.LittleEndian.Uint32(raw[36:40])}
		if uint64(h.Offset)+uint64(h.Size) > uint64(stat.Size()) {
			return nil, fmt.Errorf("section data points outside the file")
		}
		result.Sections = append(result.Sections, &pe.Section{SectionHeader: h})
	}
	return result, nil
}
