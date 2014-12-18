package mscfb

import (
	"io"
	"os"
	"testing"
)

var (
	novPapPlan  = "test/novpapplan.doc"
	testDoc     = "test/test.doc"
	testXls     = "test/test.xls"
	testPpt     = "test/test.ppt"
	testMsg     = "test/test.msg"
	testEntries = []*File{
		&File{Name: "Root Node",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: 1},
		},
		&File{Name: "Alpha",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: 2, childID: noStream},
		},
		&File{Name: "Bravo",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: 3, childID: 5},
		},
		&File{Name: "Charlie",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: 7},
		},
		&File{Name: "Delta",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: noStream},
		},
		&File{Name: "Echo",
			directoryEntryFields: &directoryEntryFields{leftSibID: 4, rightSibID: 6, childID: 9},
		},
		&File{Name: "Foxtrot",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: noStream},
		},
		&File{Name: "Golf",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: 10},
		},
		&File{Name: "Hotel",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: noStream},
		},
		&File{Name: "Indigo",
			directoryEntryFields: &directoryEntryFields{leftSibID: 8, rightSibID: noStream, childID: 11},
		},
		&File{Name: "Jello",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: noStream},
		},
		&File{Name: "Kilo",
			directoryEntryFields: &directoryEntryFields{leftSibID: noStream, rightSibID: noStream, childID: noStream},
		},
	}
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
	doc, err := New(file)
	if err != nil {
		t.Fatalf("Error opening file; Returns error: %v", err)
	}
	if len(doc.File) < 3 {
		t.Fatalf("Expecting several directory entries, only got %d", len(doc.File))
	}
	for entry, _ := doc.Next(); entry != nil; entry, _ = doc.Next() {
		buf := make([]byte, 512)
		_, err := doc.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("Error reading entry name, %v", entry.Name)
		}
		if len(entry.Name) < 1 {
			t.Errorf("Error reading entry name")
		}
	}

}

func TestTraverse(t *testing.T) {
	r := new(Reader)
	r.File = testEntries
	if r.traverse() != nil {
		t.Error("Error traversing")
	}
	expect := []int{0, 1, 2, 4, 5, 8, 9, 11, 6, 3, 7, 10}
	for i, v := range r.indexes {
		if v != expect[i] {
			t.Errorf("Error traversing: expecting %d at index %d; got %d", expect[i], i, v)
		}
	}
	if r.File[10].Path[0] != "Charlie" {
		t.Errorf("Error traversing: expecting Charlie got %s", r.File[10].Path[0])
	}
	if r.File[10].Path[1] != "Golf" {
		t.Errorf("Error traversing: expecting Golf got %s", r.File[10].Path[1])
	}
}

func TestNovPapPlan(t *testing.T) {
	testFile(t, novPapPlan)
}

func TestWord(t *testing.T) {
	testFile(t, testDoc)
}

func TestMsg(t *testing.T) {
	testFile(t, testMsg)
}

func TestPpt(t *testing.T) {
	testFile(t, testPpt)
}

func TestXls(t *testing.T) {
	testFile(t, testXls)
}
