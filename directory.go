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
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/richardlehane/msoleps/types"
)

//objectType types
const (
	unknown     uint8 = 0x0
	storage     uint8 = 0x1
	stream      uint8 = 0x2
	rootStorage uint8 = 0x5
)

// color flags
const (
	red   uint8 = 0x0
	black uint8 = 0x1
)

type directoryEntryFields struct {
	RawName           [32]uint16     //64 bytes, unicode string encoded in UTF-16. If root, "Root Entry\0" w
	NameLength        uint16         //2 bytes
	ObjectType        uint8          //1 byte Must be one of the types specified above
	Color             uint8          //1 byte Must be 0x00 RED or 0x01 BLACK
	LeftSibID         uint32         //4 bytes, Dir? Stream ID of left sibling, if none set to NOSTREAM
	RightSibID        uint32         //4 bytes, Dir? Stream ID of right sibling, if none set to NOSTREAM
	ChildID           uint32         //4 bytes, Dir? Stream ID of child object, if none set to NOSTREAM
	CLSID             types.Guid     // Contains an object class GUID (must be set to zeroes for stream object)
	StateBits         [4]byte        // user-defined flags for storage object
	Create            types.FileTime // Windows FILETIME structure
	Modify            types.FileTime // Windows FILETIME structure
	StartingSectorLoc uint32         // if a stream object, first sector location. If root, first sector of ministream
	StreamSize        uint64         // if a stream, size of user-defined data. If root, size of ministream
}

// Represents a DirectoryEntry
type DirectoryEntry struct {
	Name     string
	Initial  uint16 // the first character in the name (identifies special streams such as MSOLEPS property sets)
	Path     []string
	fn       dirFixer // to allow mocking in test
	Stream   bool     // does the storage object have a stream?
	Children bool     // does the storage object have children?
	Created  time.Time
	Modified time.Time
	ID       string // hex dump of the CLSID
	*directoryEntryFields
}

func (r *Reader) setDirEntries() error {
	c := 20
	if r.header.NumDirectorySectors > 0 {
		c = int(r.header.NumDirectorySectors)
	}
	entries := make([]*DirectoryEntry, 0, c)
	num := int(sectorSize / 128)
	sn := r.header.DirectorySectorLoc
	for sn != endOfChain {
		for i := 0; i < num; i++ {
			off := r.fileOffset(sn, false) + int64(128*i)
			entry := &DirectoryEntry{fn: fixDir}
			entry.directoryEntryFields = new(directoryEntryFields)
			if err := r.binaryReadAt(off, entry.directoryEntryFields); err != nil {
				return err
			}
			if entry.directoryEntryFields.ObjectType != unknown {
				entries = append(entries, entry)
			}
		}
		if nsn, err := r.findNext(sn, false); err != nil {
			return err
		} else {
			sn = nsn
		}
	}
	r.entries = entries
	return nil
}

type dirFixer func(e *DirectoryEntry)

func fixDir(e *DirectoryEntry) {
	fixName(e)
	e.Created = e.Create.Time()
	e.Modified = e.Modify.Time()
	e.ID = e.CLSID.String()
}

func fixName(e *DirectoryEntry) {
	nlen := 0
	if e.NameLength > 2 {
		nlen = int(e.NameLength/2 - 1)
	} else if e.NameLength > 0 {
		nlen = 1
	}
	if nlen > 0 {
		e.Initial = e.RawName[0]
		slen := 0
		if !unicode.IsPrint(rune(e.Initial)) {
			slen = 1
		}
		e.Name = string(utf16.Decode(e.RawName[slen:nlen]))
	}
}

func (r *Reader) traverse(i, d int) chan [2]int {
	c := make(chan [2]int)
	var recurse func(i, d int)
	recurse = func(i, d int) {
		if i < 0 || i > len(r.entries)-1 {
			// signal error
			c <- [2]int{-2, -2}
		}
		entry := r.entries[i]
		if entry.LeftSibID != noStream {
			recurse(int(entry.LeftSibID), d)
		}
		c <- [2]int{i, d}
		if entry.ChildID != noStream {
			d++
			recurse(int(entry.ChildID), d)
		}
		if entry.RightSibID != noStream {
			recurse(int(entry.RightSibID), d)
		}
		return
	}
	go func() {
		recurse(0, 0)
		// signal EOF
		c <- [2]int{-1, -1}
	}()
	return c
}
