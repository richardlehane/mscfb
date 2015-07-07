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
			locs = append(locs[:x+1], locs[x+2:]...)
		} else {
			x += 1
		}
	}
	return locs
}

func truncate(locs [][2]int64, sz uint64) [][2]int64 {
	remainder := int64(len(locs))*locs[0][1] - int64(sz)
	locs[len(locs)-1][1] = locs[len(locs)-1][1] - remainder
	return locs
}

func (r *Reader) stream(sn uint32, sz uint64, mini bool) ([][2]int64, error) {
	var l int
	var s int64
	if mini {
		l = int(sz)/64 + 1
		s = 64
	} else {
		l = int(uint32(sz)/sectorSize) + 1
		s = int64(sectorSize)
	}
	chain := make([][2]int64, 0, l)
	offset, err := r.getOffset(sn, mini)
	if err != nil {
		return nil, err
	}
	for i := 0; i < l; i++ {
		chain = append(chain, [2]int64{offset, s})
		sn, err = r.findNext(sn, mini)
		if err != nil {
			return nil, err
		}
		if sn == endOfChain {
			return compressChain(truncate(chain, sz)), nil
		}
		offset, err = r.getOffset(sn, mini)
		if err != nil {
			return nil, err
		}
	}
	return compressChain(truncate(chain, sz)), nil
}

func (f *File) popStream(sz int) ([][2]int64, int) {
	var total int64
	s := int64(sz)
	for i, v := range f.stream {
		total = total + v[1]
		if s < total {
			dif := total - s
			pop := make([][2]int64, i+1)
			copy(pop, f.stream[:i+1])
			pop[i][1] = pop[i][1] - dif
			f.stream = f.stream[i:]
			f.stream[0][0] = pop[i][0] + pop[i][1]
			f.stream[0][1] = dif
			return pop, sz
		}
	}
	pop := f.stream
	f.stream = [][2]int64{}
	return pop, int(total)
}
