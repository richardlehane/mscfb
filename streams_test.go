package mscfb

import (
	"testing"
)

func equal(a [][2]int64, b [][2]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v[0] != b[i][0] {
			return false
		}
		if v[1] != b[i][1] {
			return false
		}
	}
	return true
}

func TestCompress(t *testing.T) {
	a := [][2]int64{[2]int64{4608, 1024}, [2]int64{5632, 1024}, [2]int64{6656, 1024}, [2]int64{7680, 1024}, [2]int64{8704, 1024}, [2]int64{9728, 1024}, [2]int64{10752, 512}}
	ar := [][2]int64{[2]int64{4608, 6656}}
	a = compressChain(a)
	if !equal(a, ar) {
		t.Errorf("Streams compress fail; Expecting: %v, Got: %v", ar, a)
	}
	b := [][2]int64{[2]int64{4608, 1024}, [2]int64{6656, 1024}, [2]int64{7680, 1024}, [2]int64{8704, 1024}, [2]int64{10752, 512}}
	br := [][2]int64{[2]int64{4608, 1024}, [2]int64{6656, 3072}, [2]int64{10752, 512}}
	b = compressChain(b)
	if !equal(b, br) {
		t.Errorf("Streams compress fail; Expecting: %v, Got: %v", br, b)
	}
}

func TestPopStream(t *testing.T) {
	r := new(Reader)
	r.stream = [][2]int64{[2]int64{50, 500}}
	pop, sz := r.popStream(200)
	if sz != 200 {
		t.Errorf("Streams pop fail: expecting 200, got %d", sz)
	}
	if pop[0][0] != 50 && pop[0][1] != 200 {
		t.Errorf("Streams pop fail: expecting 50, 200, got %d, %d", pop[0], pop[1])
	}
	if r.stream[0][0] != 200 && r.stream[0][1] != 300 {
		t.Errorf("Streams pop fail: expecting 200, 300, got %d, %d", r.stream[0], r.stream[1])
	}
	r.stream = [][2]int64{[2]int64{50, 500}, [2]int64{1000, 600}}
	pop, sz = r.popStream(600)
	if sz != 600 {
		t.Errorf("Streams pop fail: expecting 600, got %d", sz)
	}
	if pop[0][0] != 50 && pop[0][1] != 500 {
		t.Errorf("Streams pop fail: expecting 50, 500, got %d, %d", pop[0], pop[1])
	}
	if pop[1][1] != 1000 && pop[1][1] != 100 {
		t.Errorf("Streams pop fail: expecting 1000, 100, got %d, %d", pop[0], pop[1])
	}
	if r.stream[0][0] != 1100 && r.stream[0][1] != 500 {
		t.Errorf("Streams pop fail: expecting 1100, 500, got %d, %d", r.stream[0], r.stream[1])
	}
}
