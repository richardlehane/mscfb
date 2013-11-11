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

import "unicode/utf16"

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
	RawName           [32]uint16 //64 bytes, unicode string encoded in UTF-16. If root, "Root Entry\0" w
	NameLength        uint16     //2 bytes
	ObjectType        uint8      //1 byte Must be one of the types specified above
	Color             uint8      //1 byte Must be 0x00 RED or 0x01 BLACK
	LeftSibID         uint32     //4 bytes, Dir? Stream ID of left sibling, if none set to NOSTREAM
	RightSibID        uint32     //4 bytes, Dir? Stream ID of right sibling, if none set to NOSTREAM
	ChildID           uint32     //4 bytes, Dir? Stream ID of child object, if none set to NOSTREAM
	CLSID             [16]byte   // Contains an object class GUID (must be set to zeroes for stream object)
	StateBits         [4]byte    // user-defined flags for storage object
	CreateDate        [8]byte    //Windows FILETIME structure in UTC
	ModifiedDate      [8]byte    //Windows FILETIME structure in UTC
	StartingSectorLoc uint32     // if a stream object, first sector location. If root, first sector of ministream
	StreamSize        uint64     // if a stream, size of user-defined data. If root, size of ministream
}

// Represents a DirectoryEntry
type DirectoryEntry struct {
	Name     string
	Path     []string // to create full path to file
	Dir      bool     //isDir?
	Creation string
	Modified string
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
			entry := new(DirectoryEntry)
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
	for i, v := range entries {
		nlen := 0
		if v.NameLength > 2 {
			nlen = int(v.NameLength/2 - 1)
		} else if v.NameLength > 0 {
			nlen = 1
		}
		if nlen > 0 {
			entries[i].Name = string(utf16.Decode(v.RawName[:nlen]))
		}
	}
	r.entries = entries
	return nil
}
