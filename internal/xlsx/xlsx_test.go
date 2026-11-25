package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func sheetOf(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			body, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return string(body)
		}
	}
	t.Fatalf("missing part %s", name)
	return ""
}

func TestWriteProducesWorkbookParts(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Sheet{
		{Name: "players", Header: []string{"name", "kills"}, Rows: [][]Cell{
			{Text("KSCERATO"), Number(47)},
			{Text("A&B <test>"), Number(1.5)},
		}},
		{Name: "rounds", Header: []string{"round"}, Rows: [][]Cell{{Number(1)}}},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	data := buf.Bytes()
	for _, part := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if got := sheetOf(t, data, part); got == "" {
			t.Fatalf("empty part %s", part)
		}
	}
	wb := sheetOf(t, data, "xl/workbook.xml")
	if !strings.Contains(wb, `name="players"`) || !strings.Contains(wb, `name="rounds"`) {
		t.Fatalf("workbook sheets: %s", wb)
	}
	s1 := sheetOf(t, data, "xl/worksheets/sheet1.xml")
	if !strings.Contains(s1, `r="A1" t="inlineStr" s="1"`) || !strings.Contains(s1, "name") {
		t.Fatalf("header row: %s", s1)
	}
	if !strings.Contains(s1, `<c r="B2"><v>47</v></c>`) {
		t.Fatalf("numeric cell: %s", s1)
	}
	if !strings.Contains(s1, "A&amp;B &lt;test&gt;") {
		t.Fatalf("escaped text: %s", s1)
	}
	if !strings.Contains(s1, `<v>1.5</v>`) {
		t.Fatalf("float cell: %s", s1)
	}
}

func TestWriteRejectsEmptyWorkbook(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, nil); err == nil {
		t.Fatal("expected error for empty workbook")
	}
}

func TestSheetNameIsSanitised(t *testing.T) {
	if got := sheetName("a/b:c", 0); got != "a-b-c" {
		t.Fatalf("got %q", got)
	}
	if got := sheetName("", 2); got != "Sheet3" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("x", 40)
	if got := sheetName(long, 0); len([]rune(got)) != 31 {
		t.Fatalf("len %d", len([]rune(got)))
	}
}

func TestColName(t *testing.T) {
	cases := map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"}
	for i, want := range cases {
		if got := colName(i); got != want {
			t.Fatalf("col %d = %q want %q", i, got, want)
		}
	}
}

func TestPartsAreWellFormedXML(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, []Sheet{{Name: "s", Header: []string{"a"}, Rows: [][]Cell{{Text("x"), Number(2)}}}}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(rc)
		for {
			if _, err := dec.Token(); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("%s: %v", f.Name, err)
			}
		}
		rc.Close()
	}
}
