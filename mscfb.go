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

// Package mscfb implements a reader for Microsoft's Compound File Binary File Format (http://msdn.microsoft.com/en-us/library/dd942138.aspx).
//
// The Compound File Binary File Format is also known as the Object Linking and Embedding (OLE) or Component Object Model (COM) format and was used by many
// early MS software such as MS Office.
//
// Example:
//   file, _ := os.Open("test/test.doc")
//   defer file.Close()
//   doc, err := mscfb.New(file)
//   if err != nil {
//     log.Fatal(err)
//	 }
//	 for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
//     buf := make([]byte, 512)
//     i, _ := doc.Read(buf)
//     if i > 0 {
//       fmt.Println(buf[:i])
//     }
//     fmt.Println(entry.Name)
//   }
package mscfb

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

var (
	ErrFormat = errors.New("mscfb: not a valid compound file")
	ErrRead   = errors.New("mscfb: error reading compound file")
	ErrBadDir = errors.New("mscfb: error traversing directory structure")
	ErrSeek   = errors.New("mscfb: error calculating offset")
)

var sectorSize uint32

func setSectorSize(ss uint16) {
	sectorSize = uint32(1 << ss)
}

const (
	signature            uint64 = 0xE11AB1A1E011CFD0
	miniStreamSectorSize uint32 = 64
	miniStreamCutoffSize uint64 = 4096
	dirEntrySize         uint32 = 128 //128 bytes
)

const (
	maxRegSect     uint32 = 0xFFFFFFFA // Maximum regular sector number
	difatSect      uint32 = 0xFFFFFFFC //Specifies a DIFAT sector in the FAT
	fatSect        uint32 = 0xFFFFFFFD // Specifies a FAT sector in the FAT
	endOfChain     uint32 = 0xFFFFFFFE // End of linked chain of sectors
	freeSect       uint32 = 0xFFFFFFFF // Speficies unallocated sector in the FAT, Mini FAT or DIFAT
	maxRegStreamID uint32 = 0xFFFFFFFA // maximum regular stream ID
	noStream       uint32 = 0xFFFFFFFF // empty pointer
)

func (r *Reader) readAt(offset int64, length int) ([]byte, error) {
	if r.slicer {
		b, err := r.ra.(Slicer).Slice(int(offset), length)
		if err != nil {
			return nil, ErrRead
		}
		return b, nil
	}
	if length > len(r.buf) {
		return nil, ErrRead
	}
	if _, err := r.ra.ReadAt(r.buf[:length], offset); err != nil {
		return nil, ErrRead
	}
	return r.buf[:length], nil
}

func (r *Reader) fileOffset(sn uint32, mini bool) int64 {
	if mini {
		num := sectorSize / 64
		sec := sn / num
		dif := sn % num
		return int64((r.header.miniStreamLocs[sec]+1)*sectorSize + dif*64)
	}
	return int64((sn + 1) * sectorSize)
}

// check the FAT sector for the next sector in a chain
func (r *Reader) findNext(sn uint32, mini bool) (uint32, error) {
	entries := sectorSize / 4
	index := int(sn / entries) // find position in DIFAT or minifat array
	var sect uint32
	if mini {
		if index < 0 || index >= len(r.header.miniFatLocs) {
			return 0, ErrSeek
		}
		sect = r.header.miniFatLocs[index]
	} else {
		if index < 0 || index >= len(r.header.difats) {
			return 0, ErrSeek
		}
		sect = r.header.difats[index]
	}
	fatIndex := sn % entries // find position within FAT or MiniFAT sector
	offset := r.fileOffset(sect, false)
	offset += int64(fatIndex * 4)
	buf, err := r.readAt(offset, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}

// Reader provides sequential access to the contents of a compound file
type Reader struct {
	slicer  bool
	buf     []byte
	header  *header
	File    []*File
	entry   int
	indexes []int
	ra      io.ReaderAt
}

func New(ra io.ReaderAt) (*Reader, error) {
	r := &Reader{ra: ra}
	if _, ok := ra.(Slicer); ok {
		r.slicer = true
	} else {
		r.buf = make([]byte, lenHeader)
	}
	if err := r.setHeader(); err != nil {
		return nil, err
	}
	// resize the buffer to 4096 if sector size isn't 512
	if !r.slicer && int(sectorSize) > len(r.buf) {
		r.buf = make([]byte, sectorSize)
	}
	if err := r.setDifats(); err != nil {
		return nil, err
	}
	if err := r.setDirEntries(); err != nil {
		return nil, err
	}
	if err := r.setMiniStream(); err != nil {
		return nil, err
	}
	if err := r.traverse(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reader) ID() string {
	return r.File[0].ID()
}

func (r *Reader) Created() time.Time {
	return r.File[0].Created()
}

func (r *Reader) Modified() time.Time {
	return r.File[0].Modified()
}

func (r *Reader) Next() (*File, error) {
	r.entry++
	if r.entry >= len(r.File) {
		return nil, io.EOF
	}
	return r.File[r.indexes[r.entry]], nil
}

func (r *Reader) Read(b []byte) (n int, err error) {
	if r.entry >= len(r.File) {
		return 0, io.EOF
	}
	return r.File[r.indexes[r.entry]].Read(b)
}

// API change - this func will be removed (syncronised with next major release of siegfried)
func (r *Reader) Quit() error {
	return nil
}

// Slicer interface enables MSCFB to avoid copying bytes by getting a byte slice directly from the underlying reader
type Slicer interface {
	Slice(offset int, length int) ([]byte, error)
}
