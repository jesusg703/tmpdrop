package convertx

import (
	"sort"
	"strings"
)

type Target struct {
	Ext       string `json:"ext"`
	Converter string `json:"converter"`
}

var inputExtAlias = map[string]string{
	"md":         "markdown",
	"markdown":   "markdown",
	"tex":        "latex",
	"latex":      "latex",
	"commonmark": "commonmark",
	"gfm":        "gfm",
	"mediawiki":  "mediawiki",
	"adoc":       "asciidoc",
	"htm":        "html",
	"xml":        "xml",
	"csv":        "csv",
	"tsv":        "tsv",
	"jpg":        "jpeg",
	"tif":        "tiff",
	"jfif":       "jpeg",
	"avcs":       "avif",
	"sql":        "txt", "log": "txt",
	"json": "txt", "yaml": "txt", "yml": "txt", "toml": "txt",
	"ini": "txt", "conf": "txt", "cfg": "txt", "properties": "txt", "env": "txt",
	"sh": "txt", "bash": "txt", "zsh": "txt", "fish": "txt", "ps1": "txt", "bat": "txt",
	"py": "txt", "rb": "txt", "pl": "txt", "lua": "txt", "r": "txt",
	"js": "txt", "mjs": "txt", "ts": "txt", "jsx": "txt", "tsx": "txt", "php": "txt",
	"java": "txt", "c": "txt", "h": "txt", "cpp": "txt", "cc": "txt", "hpp": "txt",
	"cs": "txt", "go": "txt", "rs": "txt", "swift": "txt", "kt": "txt", "scala": "txt",
}

func resolveInputExt(ext string) []string {
	ext = normalizeInputExt(ext)
	alias := inputExtAlias[ext]
	if alias != "" && alias != ext {
		return []string{ext, alias}
	}
	return []string{ext}
}

func normalizeInputExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	ext = strings.TrimPrefix(ext, ".")
	return ext
}

func normalizeOutputExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	switch ext {
	case "jpg":
		return "jpeg"
	case "htm":
		return "html"
	case "tif":
		return "tiff"
	}
	return ext
}

var converterForInput = map[string]map[string]bool{
	"mp3": {converterFFmpeg: true}, "wav": {converterFFmpeg: true},
	"aac": {converterFFmpeg: true}, "flac": {converterFFmpeg: true},
	"ogg": {converterFFmpeg: true}, "opus": {converterFFmpeg: true},
	"m4a": {converterFFmpeg: true}, "wma": {converterFFmpeg: true},
	"mp4": {converterFFmpeg: true}, "webm": {converterFFmpeg: true},
	"mkv": {converterFFmpeg: true}, "mov": {converterFFmpeg: true},
	"avi": {converterFFmpeg: true}, "gif": {converterFFmpeg: true},
	"png":        {converterVips: true, converterVtracer: true},
	"jpeg":       {converterVips: true},
	"jpg":        {converterVips: true},
	"webp":       {converterVips: true},
	"avif":       {converterVips: true},
	"tiff":       {converterVips: true},
	"tif":        {converterVips: true},
	"bmp":        {converterVips: true},
	"heic":       {converterVips: true},
	"jxl":        {converterLibjxl: true},
	"svg":        {converterInkscape: true, converterResvg: true},
	"docx":       {converterLibreOffice: true, converterPandoc: true},
	"doc":        {converterLibreOffice: true},
	"odt":        {converterLibreOffice: true, converterPandoc: true},
	"rtf":        {converterLibreOffice: true, converterPandoc: true},
	"pdf":        {converterLibreOffice: true, converterPandoc: true, converterInkscape: true},
	"html":       {converterLibreOffice: true, converterPandoc: true},
	"htm":        {converterLibreOffice: true, converterPandoc: true},
	"csv":        {converterLibreOffice: true},
	"xlsx":       {converterLibreOffice: true},
	"xls":        {converterLibreOffice: true},
	"pptx":       {converterLibreOffice: true, converterPandoc: true},
	"ppt":        {converterLibreOffice: true},
	"ods":        {converterLibreOffice: true},
	"txt":        {converterLibreOffice: true, converterPandoc: true},
	"md":         {converterPandoc: true},
	"markdown":   {converterPandoc: true},
	"rst":        {converterPandoc: true},
	"org":        {converterPandoc: true},
	"tex":        {converterPandoc: true, converterXeLaTeX: true},
	"latex":      {converterPandoc: true, converterXeLaTeX: true},
	"commonmark": {converterPandoc: true},
	"gfm":        {converterPandoc: true},
	"epub":       {converterCalibre: true, converterPandoc: true},
	"mobi":       {converterCalibre: true},
	"azw3":       {converterCalibre: true},
	"fb2":        {converterCalibre: true},
	"json":       {converterLibreOffice: true},
	"yaml":       {converterLibreOffice: true},
	"yml":        {converterLibreOffice: true},
	"toml":       {converterLibreOffice: true},
	"xml":        {converterLibreOffice: true},
	"sh":         {converterLibreOffice: true}, "py": {converterLibreOffice: true},
	"js": {converterLibreOffice: true}, "ts": {converterLibreOffice: true},
	"go": {converterLibreOffice: true}, "rs": {converterLibreOffice: true},
	"java": {converterLibreOffice: true}, "c": {converterLibreOffice: true},
	"cpp": {converterLibreOffice: true}, "sql": {converterLibreOffice: true},
	"vcf": {converterVCF: true},
}

var mainOutputs = map[string]map[string]bool{
	converterFFmpeg: {
		"mp3": true, "wav": true, "aac": true, "flac": true, "ogg": true, "opus": true, "m4a": true,
		"mp4": true, "webm": true, "mkv": true, "mov": true, "avi": true, "gif": true,
	},
	converterLibreOffice: {
		"pdf": true, "docx": true, "odt": true, "rtf": true,
		"txt": true, "html": true, "csv": true,
		"xlsx": true, "ods": true, "pptx": true, "epub": true,
	},
	converterPandoc: {
		"md": true, "html": true, "pdf": true, "epub": true,
		"docx": true, "latex": true, "rst": true, "org": true,
		"txt": true, "odt": true, "pptx": true,
	},
	converterVips: {
		"png": true, "jpeg": true, "webp": true,
		"tiff": true, "gif": true, "jxl": true,
	},
	converterCalibre: {
		"epub": true, "mobi": true, "azw3": true, "pdf": true, "fb2": true, "txt": true,
	},
	converterInkscape:   {"svg": true, "pdf": true, "png": true},
	converterLibjxl:     nil,
	converterVtracer:    nil,
	converterResvg:      nil,
	converterVCF:        nil,
	converterXeLaTeX:    nil,
	converterMarkitdown: nil,
	converterMsgconvert: nil,
}

var spreadsheetInputs = map[string]bool{
	"csv": true, "tsv": true, "tab": true,
	"xlsx": true, "xls": true, "ods": true, "xlsm": true,
}

var spreadsheetOutputs = map[string]bool{"xlsx": true, "ods": true, "xls": true}

var audioInputs = map[string]bool{
	"mp3": true, "wav": true, "aac": true, "flac": true,
	"ogg": true, "opus": true, "m4a": true, "wma": true,
}

var framedOutputs = map[string]bool{"gif": true, "apng": true}

func PossibleTargets(fileName string) []Target {
	ext := extractExt(fileName)
	if ext == "" {
		return nil
	}
	inputs := resolveInputExt(ext)

	var allowed map[string]bool
	for _, e := range inputs {
		if m, ok := converterForInput[e]; ok {
			allowed = m
			break
		}
	}

	seen := map[string]bool{}
	var out []Target
	for conv, spec := range converterSpecs {
		if allowed != nil && !allowed[conv] {
			continue
		}
		matched := false
		for _, e := range inputs {
			if containsStr(spec.from, e) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		filter := mainOutputs[conv]
		for _, t := range spec.to {
			nt := normalizeOutputExt(t)
			if filter != nil && !filter[nt] && !filter[t] {
				continue
			}
			if audioInputs[ext] && framedOutputs[nt] {
				continue
			}
			if spreadsheetOutputs[nt] && !spreadsheetInputs[ext] {
				continue
			}
			key := nt + "|" + conv
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Target{Ext: nt, Converter: conv})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ext != out[j].Ext {
			return out[i].Ext < out[j].Ext
		}
		return out[i].Converter < out[j].Converter
	})
	return out
}

func TargetsFor(fileName string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range PossibleTargets(fileName) {
		if !seen[t.Ext] {
			seen[t.Ext] = true
			out = append(out, t.Ext)
		}
	}
	sort.Strings(out)
	return out
}

func ConverterFor(fileName, target string) string {
	inputs := resolveInputExt(extractExt(fileName))
	target = normalizeOutputExt(target)

	var allowed map[string]bool
	for _, e := range inputs {
		if m, ok := converterForInput[e]; ok {
			allowed = m
			break
		}
	}

	for conv, spec := range converterSpecs {
		if allowed != nil && !allowed[conv] {
			continue
		}
		matched := false
		for _, e := range inputs {
			if containsStr(spec.from, e) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if containsStr(spec.to, target) {
			return conv
		}
	}
	for conv, spec := range converterSpecs {
		matched := false
		for _, e := range inputs {
			if containsStr(spec.from, e) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if containsStr(spec.to, target) {
			return conv
		}
	}
	return ""
}

func Supported(target string) bool {
	target = normalizeOutputExt(target)
	for _, spec := range converterSpecs {
		if containsStr(spec.to, target) {
			return true
		}
	}
	return false
}

func ConverterSpecs() map[string]convSpec {
	return converterSpecs
}

func uploadExt(ext string) string {
	ext = normalizeInputExt(ext)
	if alias, ok := inputExtAlias[ext]; ok {
		return alias
	}
	for _, spec := range converterSpecs {
		if containsStr(spec.from, ext) {
			return ext
		}
	}
	return ""
}

func extractExt(fileName string) string {
	ext := strings.ToLower(strings.TrimSpace(fileName))
	if i := strings.LastIndexByte(ext, '.'); i >= 0 {
		ext = ext[i+1:]
	}
	return ext
}

func containsStr(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
