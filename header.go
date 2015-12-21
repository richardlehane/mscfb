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

const lenHeader int = 8 + 16 + 10 + 6 + 12 + 8 + 16 + 109*4

type headerFields struct {
	signature           uint64
	_                   [16]byte    //CLSID - ignore, must be null
	minorVersion        uint16      //Version number for non-breaking changes. This field SHOULD be set to 0x003E if the major version field is either 0x0003 or 0x0004.
	majorVersion        uint16      //Version number for breaking changes. This field MUST be set to either 0x0003 (version 3) or 0x0004 (version 4).
	_                   [2]byte     //byte order - ignore, must be little endian
	sectorSize          uint16      //This field MUST be set to 0x0009, or 0x000c, depending on the Major Version field. This field specifies the sector size of the compound file as a power of 2. If Major Version is 3, then the Sector Shift MUST be 0x0009, specifying a sector size of 512 bytes. If Major Version is 4, then the Sector Shift MUST be 0x000C, specifying a sector size of 4096 bytes.
	_                   [2]byte     // ministream sector size - ignore, must be 64 bytes
	_                   [6]byte     // reserved - ignore, not used
	numDirectorySectors uint32      //This integer field contains the count of the number of directory sectors in the compound file. If Major Version is 3, then the Number of Directory Sectors MUST be zero. This field is not supported for version 3 compound files.
	numFatSectors       uint32      //This integer field contains the count of the number of FAT sectors in the compound file.
	directorySectorLoc  uint32      //This integer field contains the starting sector number for the directory stream.
	_                   [4]byte     // transaction - ignore, not used
	_                   [4]byte     // mini stream size cutooff - ignore, must be 4096 bytes
	miniFatSectorLoc    uint32      //This integer field contains the starting sector number for the mini FAT.
	numMiniFatSectors   uint32      //This integer field contains the count of the number of mini FAT sectors in the compound file.
	difatSectorLoc      uint32      //This integer field contains the starting sector number for the DIFAT.
	numDifatSectors     uint32      //This integer field contains the count of the number of DIFAT sectors in the compound file.
	initialDifats       [109]uint32 //The first 109 difat sectors are included in the header
}

func makeHeader(b []byte) *headerFields {
	h := &headerFields{}
	h.signature = binary.LittleEndian.Uint64(b[:8])
	h.minorVersion = binary.LittleEndian.Uint16(b[24:26])
	h.majorVersion = binary.LittleEndian.Uint16(b[26:28])
	h.sectorSize = binary.LittleEndian.Uint16(b[30:32])
	h.numDirectorySectors = binary.LittleEndian.Uint32(b[40:44])
	h.numFatSectors = binary.LittleEndian.Uint32(b[44:48])
	h.directorySectorLoc = binary.LittleEndian.Uint32(b[48:52])
	h.miniFatSectorLoc = binary.LittleEndian.Uint32(b[60:64])
	h.numMiniFatSectors = binary.LittleEndian.Uint32(b[64:68])
	h.difatSectorLoc = binary.LittleEndian.Uint32(b[68:72])
	h.numDifatSectors = binary.LittleEndian.Uint32(b[72:76])
	var idx int
	for i := 76; i < 512; i = i + 4 {
		h.initialDifats[idx] = binary.LittleEndian.Uint32(b[i : i+4])
		idx++
	}
	return h
}

type header struct {
	*headerFields
	difats         []uint32
	miniFatLocs    []uint32
	miniStreamLocs []uint32 // chain of sectors containing the ministream
}

func (r *Reader) setHeader() error {
	buf, err := r.readAt(0, lenHeader)
	if err != nil {
		return err
	}
	r.header = &header{headerFields: makeHeader(buf)}
	// sanity check - check signature
	if r.header.signature != signature {
		return Error{ErrFormat, "bad signature", int64(r.header.signature)}
	}
	// check for legal sector size
	if r.header.sectorSize == 0x0009 || r.header.sectorSize == 0x000c {
		setSectorSize(r.header.sectorSize)
	} else {
		return Error{ErrFormat, "illegal sector size", int64(r.header.sectorSize)}
	}
	// check for DIFAT overflow
	if r.header.numDifatSectors > 0 {
		sz := (sectorSize / 4) - 1
		if int(r.header.numDifatSectors*sz+109) < 0 {
			return Error{ErrFormat, "DIFAT int overflow", int64(r.header.numDifatSectors)}
		}
		if r.header.numDifatSectors*sz+109 > r.header.numFatSectors+sz {
			return Error{ErrFormat, "num DIFATs exceeds FAT sectors", int64(r.header.numDifatSectors)}
		}
	}
	// check for mini FAT overflow
	if r.header.numMiniFatSectors > 0 {
		if int(sectorSize/4*r.header.numMiniFatSectors) < 0 {
			return Error{ErrFormat, "mini FAT int overflow", int64(r.header.numMiniFatSectors)}
		}
		if r.header.numMiniFatSectors > r.header.numFatSectors*(sectorSize/miniStreamSectorSize) {
			return Error{ErrFormat, "num mini FATs exceeds FAT sectors", int64(r.header.numFatSectors)}
		}
	}
	return nil
}

func (r *Reader) setDifats() error {
	r.header.difats = r.header.initialDifats[:]
	// return early if no extra DIFAT sectors
	if r.header.numDifatSectors == 0 {
		return nil
	}
	sz := (sectorSize / 4) - 1
	n := make([]uint32, 109, r.header.numDifatSectors*sz+109)
	copy(n, r.header.difats)
	r.header.difats = n
	off := r.header.difatSectorLoc
	for i := 0; i < int(r.header.numDifatSectors); i++ {
		buf, err := r.readAt(fileOffset(off), int(sectorSize))
		if err != nil {
			return Error{ErrFormat, "error setting DIFAT(" + err.Error() + ")", int64(off)}
		}
		for j := 0; j < int(sz); j++ {
			r.header.difats = append(r.header.difats, binary.LittleEndian.Uint32(buf[j*4:j*4+4]))
		}
		off = binary.LittleEndian.Uint32(buf[len(buf)-4:])
	}
	return nil
}

// set the ministream FAT and sector slices in the header
func (r *Reader) setMiniStream() error {
	// do nothing if there is no ministream
	if r.File[0].startingSectorLoc == endOfChain || r.header.miniFatSectorLoc == endOfChain || r.header.numMiniFatSectors == 0 {
		return nil
	}
	// build a slice of minifat sectors (akin to the DIFAT slice)
	c := int(r.header.numMiniFatSectors)
	r.header.miniFatLocs = make([]uint32, c)
	r.header.miniFatLocs[0] = r.header.miniFatSectorLoc
	for i := 1; i < c; i++ {
		loc, err := r.findNext(r.header.miniFatLocs[i-1], false)
		if err != nil {
			return Error{ErrFormat, "setting mini stream (" + err.Error() + ")", int64(r.header.miniFatLocs[i-1])}
		}
		r.header.miniFatLocs[i] = loc
	}
	// build a slice of ministream sectors
	c = int(sectorSize / 4 * r.header.numMiniFatSectors)
	r.header.miniStreamLocs = make([]uint32, 0, c)
	sn := r.File[0].startingSectorLoc
	var err error
	for sn != endOfChain {
		r.header.miniStreamLocs = append(r.header.miniStreamLocs, sn)
		sn, err = r.findNext(sn, false)
		if err != nil {
			return Error{ErrFormat, "setting mini stream (" + err.Error() + ")", int64(sn)}
		}
	}
	return nil
}
