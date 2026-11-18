package fonts

import "testing"

func TestEmbeddedFontsParseAndNewFace(t *testing.T) {
	ids := []FontID{
		RajdhaniBold,
		RajdhaniSemiBold,
		NotoSansSCMedium,
		NotoSansSCBold,
	}
	for _, id := range ids {
		f, err := Parse(id)
		if err != nil {
			t.Fatalf("Parse(%s) failed: %v", id, err)
		}
		if f == nil {
			t.Fatalf("Parse(%s) returned nil font", id)
		}
		face, err := NewFace(id, 36)
		if err != nil {
			t.Fatalf("NewFace(%s) failed: %v", id, err)
		}
		if err := face.Close(); err != nil {
			t.Fatalf("Close(%s) failed: %v", id, err)
		}
	}
}
