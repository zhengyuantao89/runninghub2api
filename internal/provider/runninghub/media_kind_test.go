package runninghub

import "testing"

func TestClassifyMediaKind(t *testing.T) {
	cases := []struct {
		name, mime, want string
	}{
		{"a.png", "image/png", MediaKindImage},
		{"a.JPG", "image/jpeg", MediaKindImage},
		{"a.mp4", "video/mp4", MediaKindVideo},
		{"a.wav", "audio/wav", MediaKindAudio},
		{"a.mp3", "audio/mpeg", MediaKindAudio},
		{"a.txt", "text/plain", ""},
		{"a.mp4", "", MediaKindVideo},
		{"voice.WAV", "", MediaKindAudio},
		{"x.png", "image/png; charset=binary", MediaKindImage},
	}
	for _, c := range cases {
		got := ClassifyMediaKind(c.name, c.mime)
		if got != c.want {
			t.Fatalf("%s mime=%q got=%q want=%q", c.name, c.mime, got, c.want)
		}
	}
}
