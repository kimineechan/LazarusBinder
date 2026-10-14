package pecheck

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// ExecutionLevel reads the actual RT_MANIFEST/1 resource used by Windows.
// It does not load or execute the inspected program.
func ExecutionLevel(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	parsed, err := Parse(file)
	if err != nil {
		return "", err
	}
	var directory pe.DataDirectory
	switch optional := parsed.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		directory = optional.DataDirectory[2]
	case *pe.OptionalHeader64:
		directory = optional.DataDirectory[2]
	}
	readRVA := func(address, size uint32) ([]byte, error) {
		if size > 32<<20 {
			return nil, fmt.Errorf("resource data is too large")
		}
		for _, section := range parsed.Sections {
			start := uint64(section.VirtualAddress)
			if uint64(address) >= start && uint64(address)+uint64(size) <= start+uint64(section.Size) {
				data := make([]byte, size)
				_, e := file.ReadAt(data, int64(section.Offset)+int64(uint64(address)-start))
				return data, e
			}
		}
		return nil, fmt.Errorf("resource points outside the executable")
	}
	root, err := readRVA(directory.VirtualAddress, directory.Size)
	if err != nil {
		return "", err
	}
	entries := func(offset uint32) ([]byte, error) {
		if uint64(offset)+16 > uint64(len(root)) {
			return nil, fmt.Errorf("invalid resource table")
		}
		count := uint64(binary.LittleEndian.Uint16(root[offset+12:])) + uint64(binary.LittleEndian.Uint16(root[offset+14:]))
		if count > 4096 || uint64(offset)+16+count*8 > uint64(len(root)) {
			return nil, fmt.Errorf("invalid resource entries")
		}
		return root[uint64(offset)+16 : uint64(offset)+16+count*8], nil
	}
	find := func(offset, id uint32) (uint32, error) {
		data, e := entries(offset)
		if e != nil {
			return 0, e
		}
		for i := 0; i < len(data); i += 8 {
			if binary.LittleEndian.Uint32(data[i:]) == id {
				return binary.LittleEndian.Uint32(data[i+4:]), nil
			}
		}
		return 0, fmt.Errorf("manifest resource was not found")
	}
	kind, err := find(0, 24)
	if err != nil {
		return "", err
	}
	if kind&0x80000000 == 0 {
		return "", fmt.Errorf("invalid manifest type directory")
	}
	name, err := find(kind&0x7fffffff, 1)
	if err != nil {
		return "", err
	}
	if name&0x80000000 == 0 {
		return "", fmt.Errorf("invalid manifest name directory")
	}
	languages, err := entries(name & 0x7fffffff)
	if err != nil {
		return "", err
	}
	if len(languages) < 8 {
		return "", fmt.Errorf("manifest has no language entry")
	}
	leaf := binary.LittleEndian.Uint32(languages[4:])
	for i := 0; i < len(languages); i += 8 {
		if binary.LittleEndian.Uint32(languages[i:]) == 0 {
			leaf = binary.LittleEndian.Uint32(languages[i+4:])
			break
		}
	}
	if leaf&0x80000000 != 0 || uint64(leaf)+16 > uint64(len(root)) {
		return "", fmt.Errorf("invalid manifest data entry")
	}
	address, size := binary.LittleEndian.Uint32(root[leaf:]), binary.LittleEndian.Uint32(root[leaf+4:])
	if size > 1<<20 {
		return "", fmt.Errorf("manifest exceeds the size limit")
	}
	data, err := readRVA(address, size)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	level := ""
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
		if tag, ok := token.(xml.StartElement); ok && tag.Name.Local == "requestedExecutionLevel" && tag.Name.Space == "urn:schemas-microsoft-com:asm.v3" {
			if level != "" {
				return "", fmt.Errorf("duplicate requested execution level")
			}
			for _, attribute := range tag.Attr {
				if attribute.Name.Local == "level" {
					level = attribute.Value
				}
				if attribute.Name.Local == "uiAccess" && attribute.Value != "false" {
					return "", fmt.Errorf("unexpected UI access setting")
				}
			}
		}
	}
	if level == "" {
		return "", fmt.Errorf("requested execution level is missing")
	}
	return level, nil
}
