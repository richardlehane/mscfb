package mscfb

import (
	"io"
	"os"
	"testing"
)

var (
	testDoc = "test/test.doc"
	testXls = "test/test.xls"
	testPpt = "test/test.ppt"
	testMsg = "test/test.msg"
	entries = []*DirectoryEntry{
		&DirectoryEntry{Name: "Root Node",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: 1},
		},
		&DirectoryEntry{Name: "Alpha",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: 2, ChildID: noStream},
		},
		&DirectoryEntry{Name: "Bravo",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: 3, ChildID: 5},
		},
		&DirectoryEntry{Name: "Charlie",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: 7},
		},
		&DirectoryEntry{Name: "Delta",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: noStream},
		},
		&DirectoryEntry{Name: "Echo",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: 4, RightSibID: 6, ChildID: 9},
		},
		&DirectoryEntry{Name: "Foxtrot",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: noStream},
		},
		&DirectoryEntry{Name: "Golf",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: 10},
		},
		&DirectoryEntry{Name: "Hotel",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: noStream},
		},
		&DirectoryEntry{Name: "Indigo",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: 8, RightSibID: noStream, ChildID: 11},
		},
		&DirectoryEntry{Name: "Jello",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: noStream},
		},
		&DirectoryEntry{Name: "Kilo",
			fn:                   mockFN,
			directoryEntryFields: &directoryEntryFields{LeftSibID: noStream, RightSibID: noStream, ChildID: noStream},
		},
	}
	expect  = []int{0, 1, 2, 4, 5, 8, 9, 11, 6, 3, 7, 10}
	expectd = []int{0, 1, 1, 2, 2, 3, 3, 4, 3, 2, 3, 4}
)

func equals(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func mockFN(e *DirectoryEntry) {}

func empty(sl []byte) bool {
	for _, v := range sl {
		if v != 0 {
			return false
		}
	}
	return true
}

func testFile(t *testing.T, path string) {
	file, _ := os.Open(path)
	defer file.Close()
	doc, err := NewReader(file)
	if err != nil {
		t.Errorf("Error opening file; Returns error: ", err)
	}
	for entry, _ := doc.Next(); entry != nil; entry, _ = doc.Next() {
		buf := make([]byte, 512)
		_, err := doc.Read(buf)
		if err != nil && err != ErrNoStream && err != io.EOF {
			t.Errorf("Error reading entry name, %v", entry.Name)
		}
		if len(entry.Name) < 1 {
			t.Errorf("Error reading entry name")
		}
	}

}

func TestTraverse(t *testing.T) {
	r := new(Reader)
	r.entries = entries
	r.iter = r.traverse(0, 0)
	r.path = make([]string, 0, 5)
	indexes := make([]int, 0)
	depths := make([]int, 0)
	for {
		e := <-r.iter
		i, d := e[0], e[1]
		if i < 0 {
			break
		}
		indexes = append(indexes, i)
		depths = append(depths, d)
	}
	if !equals(indexes, expect) {
		t.Errorf("Error traversing, bad index: %v; expecting: %v", indexes, expect)
	}
	if !equals(depths, expectd) {
		t.Errorf("Error traversing, bad depths: %v; expecting: %v", depths, expect)
	}
}

func TestWord(t *testing.T) {
	testFile(t, testDoc)
}

func TestXls(t *testing.T) {
	testFile(t, testXls)
}

func TestPpt(t *testing.T) {
	testFile(t, testPpt)
}

func TestMsg(t *testing.T) {
	testFile(t, testMsg)
}
