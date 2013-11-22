// Copyright 2013 Richard Lehane. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mscfb

import "encoding/binary"

type headerFields struct {
	Signature           uint64
	_                   [16]byte    //CLSID - ignore, must be null
	MinorVersion        uint16      //Version number for non-breaking changes. This field SHOULD be set to 0x003E if the major version field is either 0x0003 or 0x0004.
	MajorVersion        uint16      //Version number for breaking changes. This field MUST be set to either 0x0003 (version 3) or 0x0004 (version 4).
	_                   [2]byte     //byte order - ignore, must be little endian
	SectorSize          uint16      //This field MUST be set to 0x0009, or 0x000c, depending on the Major Version field. This field specifies the sector size of the compound file as a power of 2. If Major Version is 3, then the Sector Shift MUST be 0x0009, specifying a sector size of 512 bytes. If Major Version is 4, then the Sector Shift MUST be 0x000C, specifying a sector size of 4096 bytes.
	_                   [2]byte     // ministream sector size - ignore, must be 64 bytes
	_                   [6]byte     // reserved - ignore, not used
	NumDirectorySectors uint32      //This integer field contains the count of the number of directory sectors in the compound file. If Major Version is 3, then the Number of Directory Sectors MUST be zero. This field is not supported for version 3 compound files.
	NumFatSectors       uint32      //This integer field contains the count of the number of FAT sectors in the compound file.
	DirectorySectorLoc  uint32      //This integer field contains the starting sector number for the directory stream.
	_                   [4]byte     // transaction - ignore, not used
	_                   [4]byte     // mini stream size cutooff - ignore, must be 4096 bytes
	MiniFatSectorLoc    uint32      //This integer field contains the starting sector number for the mini FAT.
	NumMiniFatSectors   uint32      //This integer field contains the count of the number of mini FAT sectors in the compound file.
	DifatSectorLoc      uint32      //This integer field contains the starting sector number for the DIFAT.
	NumDifatSectors     uint32      //This integer field contains the count of the number of DIFAT sectors in the compound file.
	InitialDifats       [109]uint32 //The first 109 difat sectors are included in the header
}

type header struct {
	*headerFields
	difats         []uint32
	miniFatLocs    []uint32
	miniStreamLocs []uint32 // chain of sectors containing the ministream
}

func (h *header) setDifats(r *Reader) error {
	h.difats = h.InitialDifats[:]
	if h.NumDifatSectors > 0 {
		sz := sectorSize / 4
		cap := h.NumDifatSectors*sz + 109
		n := make([]uint32, 109, cap)
		copy(n, h.difats)
		h.difats = n
		off := h.DifatSectorLoc
		for i := 0; i < int(h.NumDifatSectors); i++ {
			buf := make([]uint32, sz)
			if err := r.binaryReadAt(r.fileOffset(off, false), &buf); err != nil {
				return err
			}
			off = buf[sz-1]
			buf = buf[:sz-1]
			h.difats = append(h.difats, buf...)
		}
	}
	return nil
}

func (r *Reader) setHeader() error {
	h := new(header)
	h.headerFields = new(headerFields)
	if err := binary.Read(r.rs, binary.LittleEndian, h.headerFields); err != nil {
		return ErrFormat
	}
	// sanity check - check signature
	if h.Signature != signature {
		return ErrFormat
	}
	setSectorSize(h.SectorSize)
	if err := h.setDifats(r); err != nil {
		return ErrFormat
	}
	r.header = h
	return nil
}
