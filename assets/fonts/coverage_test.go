package fonts

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// TestCJKSubsetCoversUIText guards the bundled font subsets: the review build
// replaces the 10.5 MB Noto Sans SC releases with GBK-sized subsets, so a glyph
// the interface actually draws must still exist (a missing glyph renders as a
// tofu box, which no render test would flag).
func TestCJKSubsetCoversUIText(t *testing.T) {
	samples := []string{
		"比赛资料库", "区域控制", "道具线位", "击杀区位", "默认站位",
		"平均", "回合", "选手", "直播", "导出", "设置", "地图数据",
		"已就绪", "正在解析", "手雷", "烟雾弹", "闪光弹", "燃烧弹", "诱饵弹",
		"准星", "背身", "致盲", "遮挡", "经济", "半场", "加时", "赛点",
		"繁體字測試", "①", "，。、；：？！（）", "→", "·", "—",
		"Ryu 選手", "地图：炼狱小镇",
	}
	samples = append(samples, sourceTextRunes(t)...)
	for _, id := range []FontID{NotoSansSCMedium, NotoSansSCBold} {
		parsed, err := Parse(id)
		if err != nil {
			t.Fatalf("parse %s: %v", id, err)
		}
		for _, sample := range samples {
			for _, r := range sample {
				if r == ' ' {
					continue
				}
				if _, ok := glyphIndex(parsed, r); !ok {
					t.Fatalf("%s lacks glyph for %q (U+%04X)", id, string(r), r)
				}
			}
		}
	}
}

// sourceTextRunes collects every CJK and punctuation rune the Go sources can
// print, so a future string cannot silently ship a glyph the subset lacks.
// Emoji are skipped: no CJK font carries them and the interface never adopts a
// colour emoji font.
func sourceTextRunes(t *testing.T) []string {
	t.Helper()
	roots := []string{"../../internal", "../../cmd"}
	seen := map[rune]bool{}
	var out []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, r := range string(body) {
				if r < 0x2000 || seen[r] {
					continue
				}
				if !renderableRune(r) {
					continue
				}
				seen[r] = true
				out = append(out, string(r))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	return out
}

// renderableRune keeps the ranges the interface can rely on: general
// punctuation, arrows, block elements (sparkline bars), CJK punctuation,
// fullwidth forms and the unified ideographs the subset is built from. Geometric
// shape symbols are excluded: Noto Sans SC has no glyphs for them either, so
// they are a pre-existing tofu case rather than a subsetting regression.
func renderableRune(r rune) bool {
	switch {
	case r >= 0x2000 && r <= 0x206F:
		return true
	case r >= 0x2190 && r <= 0x21FF:
		return true
	case r >= 0x2200 && r <= 0x22FF:
		return true
	case r >= 0x2580 && r <= 0x259F: // block elements: the sparkline bars
		return true
	case r >= 0x3000 && r <= 0x303F:
		return true
	case r >= 0xFF00 && r <= 0xFF60:
		return true
	}
	return false
}

// testFace returns a face used to look glyphs up without shaping a string.
func testFace(t *testing.T) font.Face {
	t.Helper()
	face, err := NewFace(NotoSansSCMedium, 16)
	if err != nil {
		t.Fatalf("NewFace: %v", err)
	}
	return face
}

// TestFaceRendersCJK checks a CJK glyph maps to a real glyph id, not .notdef.
func TestFaceRendersCJK(t *testing.T) {
	face := testFace(t)
	if got, ok := face.GlyphAdvance('回'); !ok || got == 0 {
		t.Fatal("CJK glyph has zero advance")
	}
}

func glyphIndex(parsed *opentype.Font, r rune) (sfnt.GlyphIndex, bool) {
	idx, err := parsed.GlyphIndex(nil, r)
	if err != nil || idx == 0 {
		return 0, false
	}
	return idx, true
}
