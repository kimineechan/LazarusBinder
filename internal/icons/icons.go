// Package icons reads EXE/DLL resources as data, never executing the selected file.
package icons

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image/png"
	"lazarusbinder/internal/pecheck"
	"os"
	"sort"
	"unicode/utf16"
)

const maxIconBytes = 32 << 20

type Image struct {
	Header [12]byte
	Data   []byte
}
type Group struct {
	Label string
	Data  []byte
	Sizes string
}

func ParseICO(data []byte) ([]Image, error) {
	if len(data) < 6 || len(data) > maxIconBytes || binary.LittleEndian.Uint16(data[:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, fmt.Errorf("select a valid .ico file")
	}
	n := int(binary.LittleEndian.Uint16(data[4:6]))
	if n == 0 || n > 256 || len(data) < 6+n*16 {
		return nil, fmt.Errorf("invalid icon directory")
	}
	result := make([]Image, 0, n)
	for i := 0; i < n; i++ {
		entry := data[6+i*16 : 6+(i+1)*16]
		size := uint64(binary.LittleEndian.Uint32(entry[8:12]))
		offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
		if size == 0 || offset < uint64(6+n*16) || offset+size > uint64(len(data)) {
			return nil, fmt.Errorf("invalid icon image bounds")
		}
		blob := data[offset : offset+size]
		if len(blob) >= 8 && bytes.Equal(blob[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			config, err := png.DecodeConfig(bytes.NewReader(blob))
			if err != nil || config.Width < 1 || config.Width > 256 || config.Height < 1 || config.Height > 256 {
				return nil, fmt.Errorf("invalid PNG icon image")
			}
		} else {
			if len(blob) < 12 {
				return nil, fmt.Errorf("invalid bitmap icon image")
			}
			header := binary.LittleEndian.Uint32(blob[:4])
			if header != 12 && (header < 40 || uint64(header) > uint64(len(blob))) {
				return nil, fmt.Errorf("unsupported bitmap icon header")
			}
		}
		var image Image
		copy(image.Header[:], entry[:12])
		image.Data = blob
		result = append(result, image)
	}
	return result, nil
}

func ReadICO(path string) ([]Image, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if stat.Size() > maxIconBytes {
		return nil, fmt.Errorf("icon file is too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseICO(data)
}

type resource struct {
	name string
	id   uint32
	lang uint32
	data []byte
}

// Extract reconstructs complete multi-resolution ICOs from RT_GROUP_ICON/RT_ICON.
func Extract(path string) ([]Group, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	f, err := pecheck.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("not a Windows EXE or DLL: %w", err)
	}
	var rva, size uint32
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		rva, size = h.DataDirectory[2].VirtualAddress, h.DataDirectory[2].Size
	case *pe.OptionalHeader64:
		rva, size = h.DataDirectory[2].VirtualAddress, h.DataDirectory[2].Size
	default:
		return nil, fmt.Errorf("missing executable headers")
	}
	if rva == 0 || size == 0 {
		return nil, fmt.Errorf("this file contains no embedded icons")
	}
	if size > 128<<20 {
		return nil, fmt.Errorf("resource directory is too large")
	}
	readRVA := func(address, length uint32) ([]byte, error) {
		for _, section := range f.Sections {
			start := uint64(section.VirtualAddress)
			if uint64(address) >= start && uint64(address)+uint64(length) <= start+uint64(section.Size) {
				if length > maxIconBytes {
					return nil, fmt.Errorf("resource image is too large")
				}
				data := make([]byte, length)
				_, e := file.ReadAt(data, int64(section.Offset)+int64(uint64(address)-start))
				return data, e
			}
		}
		return nil, fmt.Errorf("resource points outside the executable")
	}
	var root []byte
	for _, section := range f.Sections {
		if uint64(rva) >= uint64(section.VirtualAddress) && uint64(rva)+uint64(size) <= uint64(section.VirtualAddress)+uint64(section.Size) {
			root = make([]byte, size)
			_, err = file.ReadAt(root, int64(section.Offset)+int64(rva-section.VirtualAddress))
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if root == nil {
		return nil, fmt.Errorf("invalid resource directory location")
	}
	slice := func(offset uint32, length uint64) ([]byte, error) {
		if uint64(offset)+length > uint64(len(root)) {
			return nil, fmt.Errorf("invalid resource directory bounds")
		}
		return root[uint64(offset) : uint64(offset)+length], nil
	}
	nameOf := func(value uint32) (string, error) {
		if value&0x80000000 == 0 {
			return fmt.Sprintf("%d", value), nil
		}
		offset := value & 0x7fffffff
		head, e := slice(offset, 2)
		if e != nil {
			return "", e
		}
		n := uint64(binary.LittleEndian.Uint16(head))
		if n > 512 {
			return "", fmt.Errorf("resource name is too long")
		}
		data, e := slice(offset+2, n*2)
		if e != nil {
			return "", e
		}
		units := make([]uint16, n)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		return string(utf16.Decode(units)), nil
	}
	var components, groups []resource
	var resourceBytes uint64
	visited := 0
	var walk func(uint32, int, uint32, string, uint32) error
	walk = func(offset uint32, depth int, kind uint32, name string, id uint32) error {
		visited++
		if visited > 8192 || depth > 2 {
			return fmt.Errorf("resource tree is too complex")
		}
		header, e := slice(offset, 16)
		if e != nil {
			return e
		}
		n := uint64(binary.LittleEndian.Uint16(header[12:14])) + uint64(binary.LittleEndian.Uint16(header[14:16]))
		if n > 4096 {
			return fmt.Errorf("resource table is too large")
		}
		entries, e := slice(offset+16, n*8)
		if e != nil {
			return e
		}
		for i := uint64(0); i < n; i++ {
			entry := entries[i*8 : i*8+8]
			key := binary.LittleEndian.Uint32(entry[:4])
			target := binary.LittleEndian.Uint32(entry[4:])
			nextKind, nextName, nextID := kind, name, id
			if depth == 0 {
				if key != 3 && key != 14 {
					continue
				}
				nextKind = key
			}
			if depth == 1 {
				nextName, e = nameOf(key)
				if e != nil {
					return e
				}
				nextID = key
			}
			if depth < 2 {
				if target&0x80000000 == 0 {
					return fmt.Errorf("invalid resource tree depth")
				}
				if e = walk(target&0x7fffffff, depth+1, nextKind, nextName, nextID); e != nil {
					return e
				}
			} else {
				if target&0x80000000 != 0 || key&0x80000000 != 0 {
					return fmt.Errorf("invalid resource language entry")
				}
				dataEntry, e := slice(target, 16)
				if e != nil {
					return e
				}
				blobSize := binary.LittleEndian.Uint32(dataEntry[4:])
				resourceBytes += uint64(blobSize)
				if resourceBytes > 128<<20 {
					return fmt.Errorf("total icon resources exceed the 128 MiB limit")
				}
				blob, e := readRVA(binary.LittleEndian.Uint32(dataEntry), blobSize)
				if e != nil {
					return e
				}
				r := resource{name: nextName, id: nextID, lang: key, data: blob}
				if nextKind == 3 {
					components = append(components, r)
				} else {
					groups = append(groups, r)
				}
			}
		}
		return nil
	}
	if err = walk(0, 0, 0, "", 0); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("this file contains no icon groups")
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].id != groups[j].id {
			return groups[i].id < groups[j].id
		}
		return groups[i].lang < groups[j].lang
	})
	var result []Group
	var outputBytes int
	for _, g := range groups {
		if len(g.data) < 6 || binary.LittleEndian.Uint16(g.data) != 0 || binary.LittleEndian.Uint16(g.data[2:]) != 1 {
			return nil, fmt.Errorf("invalid icon group")
		}
		n := int(binary.LittleEndian.Uint16(g.data[4:]))
		if n == 0 || n > 256 || len(g.data) < 6+n*14 {
			return nil, fmt.Errorf("invalid icon group entries")
		}
		ico := make([]byte, 6+n*16)
		copy(ico[:6], g.data[:6])
		sizes := ""
		for i := 0; i < n; i++ {
			entry := g.data[6+i*14 : 6+(i+1)*14]
			id := uint32(binary.LittleEndian.Uint16(entry[12:14]))
			var blob []byte
			for _, c := range components {
				if c.id == id && c.lang == g.lang {
					blob = c.data
					break
				}
			}
			if blob == nil {
				for _, c := range components {
					if c.id == id && c.lang == 0 {
						blob = c.data
						break
					}
				}
			}
			if blob == nil {
				for _, c := range components {
					if c.id == id {
						blob = c.data
						break
					}
				}
			}
			if blob == nil || uint64(len(blob)) != uint64(binary.LittleEndian.Uint32(entry[8:12])) {
				return nil, fmt.Errorf("missing or mismatched icon component")
			}
			if len(ico)+len(blob) > maxIconBytes {
				return nil, fmt.Errorf("icon group is too large")
			}
			out := ico[6+i*16 : 6+(i+1)*16]
			copy(out[:12], entry[:12])
			binary.LittleEndian.PutUint32(out[12:], uint32(len(ico)))
			ico = append(ico, blob...)
			w, h := int(entry[0]), int(entry[1])
			if w == 0 {
				w = 256
			}
			if h == 0 {
				h = 256
			}
			if i > 0 {
				sizes += ", "
			}
			sizes += fmt.Sprintf("%dx%d", w, h)
		}
		if _, e := ParseICO(ico); e != nil {
			return nil, e
		}
		outputBytes += len(ico)
		if outputBytes > 128<<20 {
			return nil, fmt.Errorf("exported icon groups exceed the 128 MiB limit")
		}
		result = append(result, Group{Label: fmt.Sprintf("Group %s / language %d", g.name, g.lang), Data: ico, Sizes: sizes})
	}
	return result, nil
}
