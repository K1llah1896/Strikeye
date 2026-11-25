// Package xlsx writes minimal Office Open XML workbooks: inline strings, numbers
// and one header row per sheet. It exists so exports do not need a spreadsheet
// dependency, and so the output stays diffable in tests.
package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Cell is one value in a sheet. Numbers are written as numeric cells so
// spreadsheet formulas keep working; everything else is an inline string.
type Cell struct {
	Text   string
	Number float64
	IsNum  bool
}

// Text builds a string cell.
func Text(s string) Cell { return Cell{Text: s} }

// Number builds a numeric cell.
func Number(v float64) Cell { return Cell{Number: v, IsNum: true} }

// Sheet is one worksheet with an optional header row.
type Sheet struct {
	Name   string
	Header []string
	Rows   [][]Cell
}

// Write serialises the workbook to w as a zip archive.
func Write(w io.Writer, sheets []Sheet) error {
	if len(sheets) == 0 {
		return fmt.Errorf("xlsx: no sheets")
	}
	zw := zip.NewWriter(w)
	add := func(name, body string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}

	if err := add("[Content_Types].xml", contentTypes(len(sheets))); err != nil {
		return err
	}
	if err := add("_rels/.rels", rootRels()); err != nil {
		return err
	}
	if err := add("xl/workbook.xml", workbook(sheets)); err != nil {
		return err
	}
	if err := add("xl/_rels/workbook.xml.rels", workbookRels(len(sheets))); err != nil {
		return err
	}
	if err := add("xl/styles.xml", styles()); err != nil {
		return err
	}
	for i, sh := range sheets {
		if err := add(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), sheetXML(sh)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func contentTypes(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	b.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func rootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
}

func workbook(sheets []Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	for i, sh := range sheets {
		fmt.Fprintf(&b, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escape(sheetName(sh.Name, i)), i+1, i+1)
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}

func workbookRels(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i)
	}
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, n+1)
	b.WriteString(`</Relationships>`)
	return b.String()
}

func styles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>` +
		`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`</styleSheet>`
}

func sheetXML(sh Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	row := 1
	if len(sh.Header) > 0 {
		writeRow(&b, row, sh.Header)
		row++
	}
	for _, r := range sh.Rows {
		writeRowCells(&b, row, r)
		row++
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func writeRow(b *strings.Builder, row int, values []string) {
	fmt.Fprintf(b, `<row r="%d">`, row)
	for i, v := range values {
		fmt.Fprintf(b, `<c r="%s%d" t="inlineStr" s="1"><is><t xml:space="preserve">%s</t></is></c>`, colName(i), row, escape(v))
	}
	b.WriteString(`</row>`)
}

func writeRowCells(b *strings.Builder, row int, cells []Cell) {
	if len(cells) == 0 {
		fmt.Fprintf(b, `<row r="%d"/>`, row)
		return
	}
	fmt.Fprintf(b, `<row r="%d">`, row)
	for i, c := range cells {
		ref := fmt.Sprintf("%s%d", colName(i), row)
		if c.IsNum {
			fmt.Fprintf(b, `<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(c.Number, 'f', -1, 64))
			continue
		}
		fmt.Fprintf(b, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, escape(c.Text))
	}
	b.WriteString(`</row>`)
}

// colName converts a zero-based column index into A1 letters.
func colName(i int) string {
	name := ""
	for i >= 0 {
		name = string(rune('A'+i%26)) + name
		i = i/26 - 1
	}
	return name
}

// sheetName keeps names inside Excel's 31 character limit and drops characters
// that are illegal in sheet names.
func sheetName(name string, idx int) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', '?', '*', '[', ']', ':':
			return '-'
		}
		return r
	}, name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		clean = fmt.Sprintf("Sheet%d", idx+1)
	}
	runes := []rune(clean)
	if len(runes) > 31 {
		clean = string(runes[:31])
	}
	return clean
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
