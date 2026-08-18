package mscfb

import (
	"bytes"
	"encoding/binary"
	"os"
	"runtime"
	"strings"
	"testing"
)

// A crafted header can declare a huge number of mini FAT sectors. Prior to
// bounding numMiniFatSectors, setMiniStream allocated make([]uint32, n) for the
// attacker-controlled n, so a tiny file could force a multi-gigabyte allocation
// (memory amplification / OOM). Regenerate a minimal case by patching a valid
// sample's header and confirm New rejects it instead of allocating.
func TestMiniFatSectorsBound(t *testing.T) {
	b, err := os.ReadFile(testXls)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(b))
	copy(buf, b)
	// numFatSectors (offset 44) is only used in header sanity comparisons and is
	// never itself used to size an allocation; make it large so the malicious
	// mini FAT count passes the "exceeds FAT sectors" check.
	binary.LittleEndian.PutUint32(buf[44:48], 0x08000000)
	// numMiniFatSectors (offset 64): request ~1 billion mini FAT sectors, which
	// would allocate ~4 GB via make([]uint32, n) without a bound.
	binary.LittleEndian.PutUint32(buf[64:68], 1_000_000_000)

	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	_, err = New(bytes.NewReader(buf))
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	if err == nil {
		t.Fatal("expected error for oversized numMiniFatSectors, got nil")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("expected mini FAT size limit error, got: %v", err)
	}
	if delta := m1.TotalAlloc - m0.TotalAlloc; delta > 64<<20 {
		t.Fatalf("excessive allocation from crafted numMiniFatSectors: %d bytes", delta)
	}
}
