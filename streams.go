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

// set the ministream FAT and sector slices in the header
func (r *Reader) setMiniStream() error {
	// do nothing if there is no ministream
	if r.File[0].startingSectorLoc == endOfChain || r.header.miniFatSectorLoc == endOfChain {
		return nil
	}
	// build a slice of minifat sectors (akin to the DIFAT slice)
	c := int(r.header.numMiniFatSectors)
	r.header.miniFatLocs = make([]uint32, c)
	r.header.miniFatLocs[0] = r.header.miniFatSectorLoc
	for i := 1; i < c; i++ {
		loc, err := r.findNext(r.header.miniFatLocs[i-1], false)
		if err != nil {
			return err
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
			return err
		}
	}
	return nil
}

func compressChain(locs [][2]int64) [][2]int64 {
	l := len(locs)
	for i, x := 0, 0; i < l && x+1 < len(locs); i++ {
		if locs[x][0]+locs[x][1] == locs[x+1][0] {
			locs[x][1] = locs[x][1] + locs[x+1][1]
			for j := range locs[x+1 : len(locs)-1] {
				locs[x+1+j] = locs[j+x+2]
			}
			locs = locs[:len(locs)-1]
		} else {
			x += 1
		}
	}
	return locs
}

// return offsets and lengths for read
func (f *File) stream(sz int) ([][2]int64, error) {
	// calculate ministream and sector size
	var mini bool
	if f.Size < miniStreamCutoffSize {
		mini = true
	}
	var l int
	var ss int64
	if mini {
		l = sz/64 + 2
		ss = 64
	} else {
		l = sz/int(sectorSize) + 2
		ss = int64(sectorSize)
	}

	sectors := make([][2]int64, 0, l)
	var i, j int

	// if we have a remainder from a previous read, use it first
	if f.rem > 0 {
		offset, err := f.r.getOffset(f.readSector, mini)
		if err != nil {
			return nil, err
		}
		if ss-f.rem >= int64(sz) {
			sectors = append(sectors, [2]int64{offset + f.rem, int64(sz)})
		} else {
			sectors = append(sectors, [2]int64{offset + f.rem, ss - f.rem})
		}
		if ss-f.rem <= int64(sz) {
			f.rem = 0
			f.readSector, err = f.r.findNext(f.readSector, mini)
			if err != nil {
				return nil, err
			}
			j += int(ss - f.rem)
		} else {
			f.rem += int64(sz)
		}
		if sectors[0][1] == int64(sz) {
			return sectors, nil
		}
		if f.readSector == endOfChain {
			return nil, ErrRead
		}
		i++
	}

	for {
		// emergency brake!
		if i >= cap(sectors) {
			return nil, ErrRead
		}
		// grab the next offset
		offset, err := f.r.getOffset(f.readSector, mini)
		if err != nil {
			return nil, err
		}
		// check if we are at the last sector
		if sz-j < int(ss) {
			sectors = append(sectors, [2]int64{offset, int64(sz - j)})
			f.rem = int64(sz - j)
			return compressChain(sectors), nil
		} else {
			sectors = append(sectors, [2]int64{offset, ss})
			j += int(ss)
			f.readSector, err = f.r.findNext(f.readSector, mini)
			if err != nil {
				return nil, err
			}
			// we might be at the last sector if there is no remainder, if so can return
			if j == sz {
				return compressChain(sectors), nil
			}
		}
		i++
	}
}
