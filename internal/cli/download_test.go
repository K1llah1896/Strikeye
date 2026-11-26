package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gcpdPage = `<html><head><script>
g_sGcContinueToken = '';
g_sessionID = "0123456789abcdef01234567";
</script></head><body><table>
<tr><td class="val_left"><table><tbody>
<tr><td><a href="%s">Download</a></td></tr>
<tr><td>2026-11-20 18:22:41 GMT</td></tr>
</tbody></table></td></tr>
<tr><td class="val_left"><table><tbody>
<tr><td>Demo expired</td></tr>
<tr><td>2026-11-19 10:00:00 GMT</td></tr>
</tbody></table></td></tr>
</table></body></html>`

func TestRunDownloadEndToEnd(t *testing.T) {
	archive := []byte("BZh91AY&SY-pretend")
	var demoPath string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	demoPath = srv.URL + "/730/111_222.dem.bz2"
	mux.HandleFunc("/730/111_222.dem.bz2", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/my/gcpd/730", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tab") != "matchhistorypremier" {
			t.Errorf("tab = %q", r.URL.Query().Get("tab"))
		}
		fmt.Fprintf(w, gcpdPage, demoPath)
	})

	t.Setenv("STRIKEYE_GCPD_BASE", srv.URL)
	out := t.TempDir()
	cookieFile := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cookieFile, []byte("Cookie: steamLoginSecure=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runDownload(context.Background(), t.TempDir(), cookieFile, []string{"matchhistorypremier"}, 0, out, &buf); err != nil {
		t.Fatalf("runDownload: %v", err)
	}
	var payload struct {
		Results []downloadOutput `json:"results"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results = %d", len(payload.Results))
	}
	res := payload.Results[0]
	if res.Listed != 1 || res.Downloaded != 1 || res.Skipped != 0 {
		t.Fatalf("result = %+v", res)
	}
	if res.OutDir != out {
		t.Fatalf("out dir = %q", res.OutDir)
	}
	written, err := os.ReadFile(filepath.Join(out, "111_222.dem.bz2"))
	if err != nil || !bytes.Equal(written, archive) {
		t.Fatalf("written = %q err=%v", written, err)
	}

	// Second run must reuse the archive instead of downloading again.
	buf.Reset()
	if err := runDownload(context.Background(), t.TempDir(), cookieFile, []string{"matchhistorypremier"}, 0, out, &buf); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !strings.Contains(buf.String(), `"reused": 1`) {
		t.Fatalf("expected reuse, got %s", buf.String())
	}
}

func TestRunDownloadRequiresCookie(t *testing.T) {
	err := runDownload(context.Background(), t.TempDir(), "", []string{"matchhistorypremier"}, 0, "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no steam cookie") {
		t.Fatalf("err = %v", err)
	}
}
