from pathlib import Path

p = Path(r"internal/provider/runninghub/qc_test.go")
t = p.read_text(encoding="utf-8")
start = t.find("func TestSanitizeQCProjectName")
end = t.find("func TestAWS4SigningKeyStable")
if start < 0 or end < 0:
    raise SystemExit(f"markers not found start={start} end={end}")
new_fn = '''func TestSanitizeQCProjectName(t *testing.T) {
	if got := sanitizeQCProjectName(""); got != "qc" {
		t.Fatalf("empty=%q", got)
	}
	if got := sanitizeQCProjectName("  hello\\r\\nworld  "); got != "hello world" {
		t.Fatalf("crlf=%q", got)
	}
	long := strings.Repeat("测", 300)
	got := sanitizeQCProjectName(long)
	runes := []rune(got)
	if len(runes) != qcProjectNameMax+1 || runes[len(runes)-1] != '…' {
		t.Fatalf("truncate got=%q len=%d", got, len(runes))
	}
}

func TestQCFreeVideoMegapixels(t *testing.T) {
	if qcFreeVideoMegapixels("480p") != 0.135164359862 {
		t.Fatalf("480p=%v", qcFreeVideoMegapixels("480p"))
	}
	if qcFreeVideoMegapixels("768p") != 0.346020761246 {
		t.Fatalf("768p=%v", qcFreeVideoMegapixels("768p"))
	}
	if qcFreeVideoMegapixels("") != 0.684269571799 {
		t.Fatalf("default=%v", qcFreeVideoMegapixels(""))
	}
}

'''
# fix the crlf line - should be actual backslash-r in Go source
new_fn = new_fn.replace("hello\\r\\nworld", "hello\r\nworld")
# Wait - in Go source we need the characters \r\n as escape in string literal
new_fn = '''func TestSanitizeQCProjectName(t *testing.T) {
	if got := sanitizeQCProjectName(""); got != "qc" {
		t.Fatalf("empty=%q", got)
	}
	if got := sanitizeQCProjectName("  hello\\r\\nworld  "); got != "hello world" {
		t.Fatalf("crlf=%q", got)
	}
	long := strings.Repeat("测", 300)
	got := sanitizeQCProjectName(long)
	runes := []rune(got)
	if len(runes) != qcProjectNameMax+1 || runes[len(runes)-1] != '…' {
		t.Fatalf("truncate got=%q len=%d", got, len(runes))
	}
}

func TestQCFreeVideoMegapixels(t *testing.T) {
	if qcFreeVideoMegapixels("480p") != 0.135164359862 {
		t.Fatalf("480p=%v", qcFreeVideoMegapixels("480p"))
	}
	if qcFreeVideoMegapixels("768p") != 0.346020761246 {
		t.Fatalf("768p=%v", qcFreeVideoMegapixels("768p"))
	}
	if qcFreeVideoMegapixels("") != 0.684269571799 {
		t.Fatalf("default=%v", qcFreeVideoMegapixels(""))
	}
}

'''
p.write_text(t[:start] + new_fn + t[end:], encoding="utf-8")
print("patched", start, end)
