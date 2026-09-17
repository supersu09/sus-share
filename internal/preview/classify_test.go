package preview

import "testing"

var pngHead = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

func TestClassifySafeAndUnsafeTypes(t *testing.T) {
	cases := []struct {
		name       string
		filename   string
		clientType string
		head       []byte
		want       Kind
		wantInline bool
	}{
		{name: "png", filename: "a.png", clientType: "image/png", head: pngHead, want: InlineImage, wantInline: true},
		{name: "html", filename: "a.html", clientType: "text/html", head: []byte("<html>"), want: Download},
		{name: "svg", filename: "a.svg", clientType: "image/svg+xml", head: []byte("<svg"), want: Download},
		{name: "text", filename: "a.sql", clientType: "text/plain", head: []byte("SELECT 1"), want: Text},
		{name: "unknown", filename: "a.zzz", clientType: "application/octet-stream", head: []byte{0x00}, want: Download},
		{name: "mismatch", filename: "a.jpg", clientType: "image/jpeg", head: pngHead, want: Download},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.filename, tc.clientType, tc.head)
			if got.Kind != tc.want || got.SafeInline != tc.wantInline {
				t.Fatalf("classification = %+v, want kind %s inline %v", got, tc.want, tc.wantInline)
			}
		})
	}
}
