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
	"io"
	"os"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/richardlehane/msoleps/types"
)

//objectType types
const (
	unknown     uint8 = 0x0 // this means unallocated - typically zeroed dir entries
	storage     uint8 = 0x1 // this means dir
	stream      uint8 = 0x2 // this means file
	rootStorage uint8 = 0x5 // this means root
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
type File struct {
	Name    string
	Initial uint16 // the first character in the name (identifies special streams such as MSOLEPS property sets)
	Path    []string
	Size    uint64     // size of stream
	stream  [][2]int64 // contains file offsets for the current stream and lengths
	*directoryEntryFields
	r *Reader
}

type fileInfo struct{ *File }

func (fi fileInfo) Name() string { return fi.File.Name }
func (fi fileInfo) Size() int64 {
	if fi.objectType != stream {
		return 0
	}
	return int64(fi.File.Size)
}
func (fi fileInfo) IsDir() bool        { return fi.Mode().IsDir() }
func (fi fileInfo) ModTime() time.Time { return fi.Modified() }
func (fi fileInfo) Mode() os.FileMode  { return fi.File.Mode() }
func (fi fileInfo) Sys() interface{}   { return nil }

func (f *File) FileInfo() os.FileInfo {
	return fileInfo{f}
}

func (f *File) ID() string {
	return f.clsid.String()
}

func (f *File) Created() time.Time {
	return f.create.Time()
}

func (f *File) Modified() time.Time {
	return f.modify.Time()
}

func (f *File) Mode() os.FileMode {
	if f.objectType != stream {
		return os.ModeDir | 0777
	}
	return 0666
}

func (f *File) Read(b []byte) (n int, err error) {
	if f.objectType != stream || f.Size < 1 {
		return 0, io.EOF
	}
	// set the stream if hasn't been done yet
	if f.stream == nil {
		var mini bool
		if f.Size < miniStreamCutoffSize {
			mini = true
		}
		str, err := f.r.stream(f.startingSectorLoc, f.Size, mini)
		if err != nil {
			return 0, err
		}
		f.stream = str
	}
	// now do the read
	str, sz := f.popStream(len(b))
	var idx int64
	var i int
	for _, v := range str {
		jdx := idx + v[1]
		if idx < 0 || jdx < idx || jdx > int64(len(b)) {
			return 0, ErrRead
		}
		j, err := f.r.ra.ReadAt(b[idx:jdx], v[0])
		i = i + j
		if err != nil {
			return i, ErrRead
		}
		idx += v[1]
	}
	if sz < len(b) {
		return sz, io.EOF
	}
	return sz, nil
}

func (r *Reader) setDirEntries() error {
	c := 20
	if r.header.numDirectorySectors > 0 {
		c = int(r.header.numDirectorySectors)
	}
	fs := make([]*File, 0, c)
	num := int(sectorSize / 128)
	sn := r.header.directorySectorLoc
	for sn != endOfChain {
		off := r.fileOffset(sn, false)
		buf, err := r.readAt(off, int(sectorSize))
		if err != nil {
			return ErrRead
		}
		for i := 0; i < num; i++ {
			f := &File{r: r}
			f.directoryEntryFields = makeDirEntry(buf[i*128:])
			if f.directoryEntryFields.objectType != unknown {
				fixFile(r.header.majorVersion, f)
				fs = append(fs, f)
			}
		}
		if nsn, err := r.findNext(sn, false); err != nil {
			return err
		} else {
			sn = nsn
		}
	}
	r.File = fs
	return nil
}

func fixFile(v uint16, f *File) {
	fixName(f)
	// if the MSCFB major version is 4, then this can be a uint64 otherwise is a uint32 and the least signficant bits can contain junk
	if v > 3 {
		f.Size = binary.LittleEndian.Uint64(f.streamSize[:])
	} else {
		f.Size = uint64(binary.LittleEndian.Uint32(f.streamSize[:4]))
	}
}

func fixName(f *File) {
	nlen := 0
	if f.nameLength > 2 {
		// The length MUST be a multiple of 2, and include the terminating null character in the count.
		nlen = int(f.nameLength/2 - 1)
	} else if f.nameLength > 0 {
		nlen = 1
	}
	if nlen > 0 {
		f.Initial = f.rawName[0]
		slen := 0
		if !unicode.IsPrint(rune(f.Initial)) {
			slen = 1
		}
		f.Name = string(utf16.Decode(f.rawName[slen:nlen]))
	}
}

func (r *Reader) traverse() error {
	r.indexes = make([]int, len(r.File))
	var idx int
	var recurse func(i int, path []string)
	var err error
	recurse = func(i int, path []string) {
		if i < 0 || i >= len(r.File) {
			err = ErrBadDir
			return
		}
		file := r.File[i]
		if file.leftSibID != noStream {
			recurse(int(file.leftSibID), path)
		}
		r.indexes[idx] = i
		file.Path = path
		idx++
		if file.childID != noStream {
			if i > 0 {
				recurse(int(file.childID), append(path, file.Name))
			} else {
				recurse(int(file.childID), path)
			}
		}
		if file.rightSibID != noStream {
			recurse(int(file.rightSibID), path)
		}
		return
	}
	recurse(0, []string{})
	return err
}
