package mscfb

import (
	"os"
	"testing"
)

var (
	testDoc = "test/test.doc"
	testXls = "test/test.xls"
	testPpt = "test/test.ppt"
)

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
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		buf := make([]byte, 512)
		i, _ := doc.Read(buf)
		if i > 0 {
			if empty(buf) {
				t.Errorf("Error reading entry; slice is empty")
			}
		}
		if len(entry.Name) < 1 {
			t.Errorf("Error reading entry name")
		}
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
