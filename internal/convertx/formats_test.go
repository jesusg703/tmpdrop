package convertx

import (
	"slices"
	"testing"
)

func TestTargetsFor(t *testing.T) {
	for _, tc := range []struct {
		name    string
		wantNil bool
		must    []string
	}{
		{"video.mp4", false, []string{"mp3", "wav", "aac", "mp4", "webm", "mkv", "avi", "gif"}},
		{"song.mp3", false, []string{"mp3", "wav", "aac", "flac", "ogg", "opus", "m4a"}},
		{"doc.docx", false, []string{"pdf", "docx", "html", "txt", "rtf", "epub"}},
		{"scan.pdf", false, []string{"pdf", "txt"}},
		{"note.md", false, []string{"html", "epub", "pdf", "docx"}},
		{"note.rst", false, []string{"html", "epub", "pdf", "docx"}},
		{"photo.png", false, []string{"png", "jpeg", "webp", "tiff", "gif", "svg"}},
		{"book.epub", false, []string{"epub", "pdf", "mobi", "azw3", "fb2"}},
		{"data.json", false, []string{"pdf", "docx", "txt", "html"}},
		{"conf.yaml", false, []string{"pdf", "txt"}},
		{"code.sh", false, []string{"pdf", "docx", "txt"}},
		{"script.py", false, []string{"pdf", "txt"}},
		{"sheet.csv", false, []string{"pdf", "xlsx", "ods", "html"}},
		{"archive.tar.gz", true, nil},
		{"NOEXT", true, nil},
	} {
		got := TargetsFor(tc.name)
		if tc.wantNil {
			if len(got) != 0 {
				t.Errorf("TargetsFor(%q) = %v, want none", tc.name, got)
			}
			continue
		}
		if len(got) == 0 {
			t.Errorf("TargetsFor(%q) = empty, want some targets", tc.name)
			continue
		}
		for _, m := range tc.must {
			if !slices.Contains(got, m) {
				t.Errorf("TargetsFor(%q) missing %q, got %v", tc.name, m, got)
			}
		}
	}
}

func TestTargetCount(t *testing.T) {
	tests := []struct {
		name string
		max  int
	}{
		{"song.mp3", 13},
		{"video.mp4", 13},
		{"photo.png", 10},
		{"doc.docx", 15},
		{"code.sh", 10},
	}
	for _, tc := range tests {
		got := TargetsFor(tc.name)
		if len(got) > tc.max {
			t.Errorf("TargetsFor(%q) = %d targets (max %d): %v", tc.name, len(got), tc.max, got)
		}
	}
}

func TestSupported(t *testing.T) {
	known := []string{"mp3", "pdf", "png", "jpg", "jpeg", "docx", "epub", "webm", "csv", "svg", "md", "txt", "markdown"}
	for _, ext := range known {
		if !Supported(ext) {
			t.Errorf("Supported(%q) = false, want true", ext)
		}
	}
	if Supported("") || Supported("xyzzy") || Supported("rar") {
		t.Error("Supported returned true for unknown format")
	}
}

func TestConverterFor(t *testing.T) {
	tests := []struct {
		src, target string
		valid       []string
	}{
		{"in.mp4", "mp3", []string{"ffmpeg"}},
		{"in.mp4", "wav", []string{"ffmpeg"}},
		{"in.docx", "pdf", []string{"libreoffice", "pandoc"}},
		{"in.docx", "html", []string{"libreoffice", "pandoc"}},
		{"in.png", "svg", []string{"vtracer"}},
		{"in.png", "jpeg", []string{"vips"}},
		{"in.md", "epub", []string{"pandoc"}},
		{"in.epub", "pdf", []string{"calibre", "pandoc"}},
	}
	for _, tc := range tests {
		got := ConverterFor(tc.src, tc.target)
		if got == "" {
			t.Errorf("ConverterFor(%q, %q) = empty, want one of %v", tc.src, tc.target, tc.valid)
			continue
		}
		if !slices.Contains(tc.valid, got) {
			t.Errorf("ConverterFor(%q, %q) = %q, want one of %v", tc.src, tc.target, got, tc.valid)
		}
	}
}

func TestNormalizeOutputExt(t *testing.T) {
	tests := map[string]string{
		"jpg": "jpeg", "htm": "html", "tif": "tiff", "png": "png", "pdf": "pdf",
	}
	for in, want := range tests {
		if got := normalizeOutputExt(in); got != want {
			t.Errorf("normalizeOutputExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPossibleTargets(t *testing.T) {
	targets := PossibleTargets("song.mp3")
	if len(targets) == 0 {
		t.Fatal("PossibleTargets(mp3) = empty")
	}
	for _, tgt := range targets {
		if tgt.Ext == "" || tgt.Converter == "" {
			t.Errorf("PossibleTargets(mp3) has zero-value target: %+v", tgt)
		}
	}
	for _, want := range []string{"mp3", "wav", "aac", "flac", "ogg", "opus", "m4a"} {
		found := false
		for _, tgt := range targets {
			if tgt.Ext == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PossibleTargets(mp3) missing audio target %q", want)
		}
	}
}

func TestAudioSourceOffersNoFramedTarget(t *testing.T) {
	for _, name := range []string{"tone.wav", "song.mp3", "voice.m4a"} {
		got := TargetsFor(name)
		if len(got) == 0 {
			t.Errorf("TargetsFor(%q) = empty", name)
			continue
		}
		for _, bad := range []string{"gif", "apng"} {
			if slices.Contains(got, bad) {
				t.Errorf("TargetsFor(%q) offers %q, which needs video frames", name, bad)
			}
		}
		if !slices.Contains(got, "mp3") {
			t.Errorf("TargetsFor(%q) = %v, want mp3 among them", name, got)
		}
	}
}

func TestOnlyTabularSourcesOfferSpreadsheets(t *testing.T) {
	for _, name := range []string{"scan.pdf", "doc.docx", "note.md"} {
		for _, bad := range []string{"xlsx", "ods", "xls"} {
			if slices.Contains(TargetsFor(name), bad) {
				t.Errorf("TargetsFor(%q) offers %q", name, bad)
			}
		}
	}
	got := TargetsFor("sheet.csv")
	for _, want := range []string{"xlsx", "ods"} {
		if !slices.Contains(got, want) {
			t.Errorf("TargetsFor(\"sheet.csv\") = %v, missing %q", got, want)
		}
	}
}
