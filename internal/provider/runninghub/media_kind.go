package runninghub

import (
	"path/filepath"
	"strings"
)

const (
	MediaKindImage = "image"
	MediaKindVideo = "video"
	MediaKindAudio = "audio"
)

// ClassifyMediaKind returns image|video|audio or "" if unsupported for official multimodal refs.
// Prefers MIME when provided; falls back to filename extension (HAR: JPG/PNG, MP4, MP3/WAV).
func ClassifyMediaKind(filename, mime string) string {
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	switch mime {
	case "image/jpeg", "image/jpg", "image/png":
		return MediaKindImage
	case "video/mp4":
		return MediaKindVideo
	case "audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav", "audio/wave":
		return MediaKindAudio
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png":
		return MediaKindImage
	case ".mp4":
		return MediaKindVideo
	case ".mp3", ".wav":
		return MediaKindAudio
	}
	return ""
}
