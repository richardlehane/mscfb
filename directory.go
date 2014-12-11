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

import (
	"encoding/binary"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/richardlehane/msoleps/types"
)

//objectType types
const (
	unknown     uint8 = 0x0 // this means unallocated - typically zeroed dir entries
	storage     uint8 = 0x1
	stream      uint8 = 0x2
	rootStorage uint8 = 0x5
)

// color flags
const (
	red   uint8 = 0x0
	black uint8 = 0x1
)

const lenDirEntry int = 64 + 4*4 + 16 + 4 + 8*2 + 4 + 8

type directoryEntryFields struct {
	rawName           [32]uint16     //64 bytes, unicode string encoded in UTF-16. If root, "Root Entry\0" w
	nameLength        uint16         //2 bytes
	objectType        uint8          //1 byte Must be one of the types specified above
	color             uint8          //1 byte Must be 0x00 RED or 0x01 BLACK
	leftSibID         uint32         //4 bytes, Dir? Stream ID of left sibling, if none set to NOSTREAM
	rightSibID        uint32         //4 bytes, Dir? Stream ID of right sibling, if none set to NOSTREAM
	childID           uint32         //4 bytes, Dir? Stream ID of child object, if none set to NOSTREAM
	clsid             types.Guid     // Contains an object class GUID (must be set to zeroes for stream object)
	stateBits         [4]byte        // user-defined flags for storage object
	create            types.FileTime // Windows FILETIME structure
	modify            types.FileTime // Windows FILETIME structure
	startingSectorLoc uint32         // if a stream object, first sector location. If root, first sector of ministream
	streamSize        [8]byte        // if a stream, size of user-defined data. If root, size of ministream
}

func makeDirEntry(b []byte) *directoryEntryFields {
	d := &directoryEntryFields{}
	for i := range d.rawName {
		d.rawName[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	d.nameLength = binary.LittleEndian.Uint16(b[64:66])
	d.objectType = uint8(b[66])
	d.color = uint8(b[67])
	d.leftSibID = binary.LittleEndian.Uint32(b[68:72])
	d.rightSibID = binary.LittleEndian.Uint32(b[72:76])
	d.childID = binary.LittleEndian.Uint32(b[76:80])
	d.clsid = types.MustGuid(b[80:96])
	copy(d.stateBits[:], b[96:100])
	d.create = types.MustFileTime(b[100:108])
	d.modify = types.MustFileTime(b[108:116])
	d.startingSectorLoc = binary.LittleEndian.Uint32(b[116:120])
	copy(d.streamSize[:], b[120:128])
	return d
}

// Represents a DirectoryEntry
type DirectoryEntry struct {
	Name    string
	Initial uint16 // the first character in the name (identifies special streams such as MSOLEPS property sets)
	Path    []string
	fn      dirFixer // to allow mocking in test
	Stream  bool     // does the entry have a stream?
	Size    uint64   // size of stream
	*directoryEntryFields
}

func (d *DirectoryEntry) ID() string {
	return d.clsid.String()
}

func (d *DirectoryEntry) Created() time.Time {
	return d.create.Time()
}

func (d *DirectoryEntry) Modified() time.Time {
	return d.modify.Time()
}

func (r *Reader) setDirEntries() error {
	c := 20
	if r.header.numDirectorySectors > 0 {
		c = int(r.header.numDirectorySectors)
	}
	entries := make([]*DirectoryEntry, 0, c)
	num := int(sectorSize / 128)
	sn := r.header.directorySectorLoc
	for sn != endOfChain {
		off := r.fileOffset(sn, false)
		buf, err := r.readAt(off, int(sectorSize))
		if err != nil {
			return ErrRead
		}
		for i := 0; i < num; i++ {
			entry := &DirectoryEntry{fn: fixDir(r.header.majorVersion)}
			entry.directoryEntryFields = makeDirEntry(buf[i*128:])
			if entry.directoryEntryFields.objectType != unknown {
				entries = append(entries, entry)
			}
		}
		if nsn, err := r.findNext(sn, false); err != nil {
			return err
		} else {
			sn = nsn
		}
	}
	r.Entries = entries
	return nil
}

type dirFixer func(e *DirectoryEntry)

func fixDir(v uint16) dirFixer {
	return func(e *DirectoryEntry) {
		fixName(e)
		// if the MSCFB major version is 4, then this can be a uint64 otherwise is a uint32 and the least signficant bits can contain junk
		if v > 3 {
			e.Size = binary.LittleEndian.Uint64(e.streamSize[:])
		} else {
			e.Size = uint64(binary.LittleEndian.Uint32(e.streamSize[:4]))
		}
		if e.objectType == stream && e.startingSectorLoc <= maxRegSect && e.Size > 0 {
			e.Stream = true
		}
	}
}

func fixName(e *DirectoryEntry) {
	nlen := 0
	if e.nameLength > 2 {
		// The length MUST be a multiple of 2, and include the terminating null character in the count.
		nlen = int(e.nameLength/2 - 1)
	} else if e.nameLength > 0 {
		nlen = 1
	}
	if nlen > 0 {
		e.Initial = e.rawName[0]
		slen := 0
		if !unicode.IsPrint(rune(e.Initial)) {
			slen = 1
		}
		e.Name = string(utf16.Decode(e.rawName[slen:nlen]))
	}
}

func (r *Reader) traverse() error {
	r.indexes = make([]int, len(r.Entries))
	var idx int
	var recurse func(i int, path []string)
	var err error
	recurse = func(i int, path []string) {
		if i < 0 || i > len(r.Entries)-1 {
			err = ErrBadDir
			return
		}
		entry := r.Entries[i]
		if entry.leftSibID != noStream {
			recurse(int(entry.leftSibID), path)
		}
		entry.fn(entry)
		r.indexes[idx] = i
		entry.Path = path
		idx++
		if entry.childID != noStream {
			if i > 0 {
				recurse(int(entry.childID), append(path, entry.Name))
			} else {
				recurse(int(entry.childID), path)
			}
		}
		if entry.rightSibID != noStream {
			recurse(int(entry.rightSibID), path)
		}
		return
	}
	recurse(0, []string{})
	return err
}
