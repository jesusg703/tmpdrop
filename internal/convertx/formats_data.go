package convertx

type convSpec struct {
	from []string
	to   []string
}

const (
	converterAssimp         = "assimp"
	converterCalibre        = "calibre"
	converterDasel          = "dasel"
	converterDvisvgm        = "dvisvgm"
	converterFFmpeg         = "ffmpeg"
	converterGraphicsMagick = "graphicsmagick"
	converterImageMagick    = "imagemagick"
	converterInkscape       = "inkscape"
	converterLibheif        = "libheif"
	converterLibjxl         = "libjxl"
	converterLibreOffice    = "libreoffice"
	converterMarkitdown     = "markitdown"
	converterMsgconvert     = "msgconvert"
	converterPandoc         = "pandoc"
	converterPotrace        = "potrace"
	converterResvg          = "resvg"
	converterVCF            = "vcf"
	converterVips           = "vips"
	converterVtracer        = "vtracer"
	converterXeLaTeX        = "xelatex"
)

var converterSpecs = map[string]convSpec{
	converterAssimp: {
		from: []string{"3d", "3ds", "3mf", "ac", "ac3d", "acc", "amf", "amj", "ase", "ask", "assbin", "b3d", "blend", "bsp", "bvh", "cob", "csm", "dae", "dxf", "enff", "fbx", "glb", "gltf", "hmb", "hmp", "ifc", "ifczip", "iqm", "irr", "irrmesh", "lwo", "lws", "lxo", "m3d", "md2", "md3", "md5anim", "md5camera", "md5mesh", "mdc", "mdl", "mesh.xml", "mesh", "mot", "ms3d", "ndo", "nff", "obj", "off", "ogex", "pk3", "ply", "pmx", "prj", "q3o", "q3s", "raw", "scn", "sib", "smd", "step", "stl", "stp", "ter", "uc", "usd", "usda", "usdc", "usdz", "vta", "x", "x3d", "x3db", "xgl", "xml", "zae", "zgl"},
		to:   []string{"3ds", "3mf", "assbin", "assjson", "assxml", "collada", "dae", "fbx", "fbxa", "glb", "glb2", "gltf", "gltf2", "json", "obj", "objnomtl", "pbrt", "ply", "plyb", "stl", "stlb", "stp", "x"},
	},
	converterCalibre: {
		from: []string{"azw3", "azw4", "chm", "cbr", "cbz", "cbt", "cba", "cb7", "djvu", "docx", "epub", "fb2", "htlz", "html", "lit", "lrf", "mobi", "odt", "pdb", "pdf", "pml", "rb", "rtf", "recipe", "snb", "tcr", "txt"},
		to:   []string{"azw3", "docx", "epub", "fb2", "html", "htmlz", "kepub.epub", "lit", "lrf", "mobi", "oeb", "pdb", "pdf", "pml", "rb", "rtf", "snb", "tcr", "txt", "txtz"},
	},
	converterDvisvgm: {
		from: []string{"dvi", "xdv", "pdf", "eps"},
		to:   []string{"svg", "svgz"},
	},
	converterFFmpeg: {
		from: []string{"264", "265", "266", "302", "3dostr", "3g2", "3gp", "4xm", "669", "722", "aa", "aa3", "aac", "aax", "ac3", "ac4", "ace", "acm", "act", "adf", "adp", "ads", "adx", "aea", "afc", "aiff", "aix", "al", "alaw", "alias_pix", "alp", "alsa", "amf", "amr", "amrnb", "amrwb", "ams", "anm", "ans", "apc", "ape", "apl", "apm", "apng", "aptx", "aptxhd", "aqt", "aqtitle", "argo_asf", "argo_brp", "art", "asc", "asf", "asf_o", "ass", "ast", "au", "av1", "avc", "avi", "avif", "avr", "avs", "avs2", "avs3", "awb", "bcstm", "bethsoftvid", "bfi", "bfstm", "bin", "bink", "binka", "bit", "bitpacked", "bmv", "bmp", "bonk", "boa", "brender_pix", "brstm", "c2", "c93", "caf", "cavsvideo", "cdata", "cdg", "cdxl", "cgi", "cif", "cine", "codec2", "codec2raw", "concat", "cri", "dash", "dat", "data", "daud", "dav", "dbm", "dcstr", "dds", "derf", "dfpwm", "dfa", "dhav", "dif", "digi", "dirac", "diz", "dmf", "dnxhd", "dpx_pipe", "dsf", "dsicin", "dsm", "dss", "dtk", "dtm", "dts", "dtshd", "dv", "dvbsub", "dvbtxt", "dxa", "ea", "eac3", "ea_cdata", "epaf", "exr_pipe", "f32be", "f32le", "ec3", "evc", "f4v", "f64be", "f64le", "fap", "far", "fbdev", "ffmetadata", "filmstrip", "film_cpk", "fits", "flac", "flic", "flm", "flv", "frm", "fsb", "fwse", "g722", "g723_1", "g726", "g726le", "g729", "gdm", "gdv", "genh", "gif", "gsm", "gxf", "h261", "h263", "h264", "h265", "h266", "h26l", "hca", "hcom", "hevc", "hls", "hnm", "ice", "ico", "idcin", "idf", "idx", "iec61883", "iff", "ifv", "ilbc", "image2", "imf", "imx", "ingenient", "ipmovie", "ipu", "ircam", "ism", "isma", "ismv", "iss", "it", "iv8", "ivf", "ivr", "j2b", "j2k", "jack", "jacosub", "jv", "jpegls", "jpeg", "jxl", "kmsgrab", "kux", "kvag", "lavfi", "laf", "lmlm4", "loas", "lrc", "luodat", "lvf", "lxf", "m15", "m2a", "m4a", "m4b", "m4v", "mac", "mca", "mcc", "mdl", "med", "microdvd", "mj2", "mjpeg", "mjpg", "mk3d", "mka", "mks", "mkv", "mlp", "mlv", "mm", "mmcmp", "mmf", "mms", "mo3", "mod", "mods", "moflex", "mov", "mp2", "mp3", "mp4", "mpa", "mpc", "mpeg", "mpg", "msbc", "msf", "msnwctcp", "mtaf", "mtv", "musx", "mv", "mvi", "mxf", "mxg", "nc", "nistsphere", "nsp", "nsv", "nut", "nuv", "obu", "oga", "ogg", "ogv", "oma", "omg", "opau64", "oplm4a", "oplpcm", "opus", "osq", "paf", "pam", "pbm", "pcm", "pcx", "pfr", "pgm", "pgmyuv", "phm", "photocd", "pict", "pix", "ppm", "psxstr", "pva", "pvf", "qcp", "qoi", "qvg", "r3d", "rawvideo", "ra", "ras", "raw", "rcwt", "realtext", "redspark", "rl2", "rm", "roq", "rsd", "rso", "rtp", "rtsp", "s337m", "samide", "sap", "sauce", "sbg", "scc", "scd", "sdns", "sdp", "sdr2", "sds", "sdx", "ser", "sga", "shn", "siff", "simbiosis_imx", "slk", "smk", "smush", "sol", "sox", "spdif", "spx", "srt", "srtp", "ss2", "ssa", "sub", "sun", "sunras", "sup", "svag", "svs", "swf", "tak", "tco", "ted", "thd", "tiertexseq", "tiff", "tmv", "truehd", "ts", "tta", "ttml", "tun", "ub", "ul", "usf", "utf8", "v", "vag", "valve", "vbn", "vc1", "vc2", "vidc", "vie", "vividas", "vivo", "vmd", "vob", "voc", "vpk", "vplayer", "vqf", "vvc", "w64", "wav", "wavlike", "wc3movie", "webm", "webp", "wii_sam", "wm", "wma", "wmv", "wn", "wsaud", "wsvqa", "wtv", "wv", "wve", "xa", "xbin", "xmv", "xvag", "xwma", "yop", "yuv4mpegpipe", "y4m"},
		to:   []string{"264", "265", "266", "302", "3g2", "3gp", "a64", "aac", "ac3", "ac4", "adts", "adx", "afc", "aif", "aifc", "aiff", "al", "amr", "amv", "apm", "apng", "aptx", "aptxhd", "asf", "ass", "ast", "au", "aud", "av1.mkv", "av1.mp4", "avi", "avif", "avs", "avs2", "avs3", "bit", "bmp", "c2", "caf", "cavs", "chk", "cpk", "cvg", "dfpwm", "dnxhd", "dnxhr", "dpx", "drc", "dts", "dv", "dvd", "eac3", "ec3", "evc", "exr", "f4v", "ffmeta", "fits", "flac", "flm", "flv", "g722", "gif", "gsm", "gxf", "h261", "h263", "h264.mkv", "h264.mp4", "h265.mkv", "h265.mp4", "h266.mkv", "hdr", "hevc", "ico", "im1", "im24", "im8", "ircam", "isma", "ismv", "ivf", "j2c", "j2k", "jls", "jp2", "jpeg", "jpg", "js", "jss", "jxl", "latm", "lbc", "ljpg", "loas", "lrc", "m1v", "m2a", "m2t", "m2ts", "m2v", "m3u8", "m4a", "m4b", "m4v", "mjpeg", "mjpg", "mkv", "mlp", "mmf", "mov", "mp2", "mp3", "mp4", "mpa", "mpd", "mpeg", "mpg", "msbc", "mts", "mxf", "nut", "obu", "oga", "ogg", "ogv", "oma", "opus", "pam", "pbm", "pcm", "pcx", "pfm", "pgm", "pgmyuv", "phm", "pix", "png", "ppm", "psp", "qoi", "ra", "ras", "rco", "rcv", "rgb", "rm", "roq", "rs", "rso", "sb", "sbc", "scc", "sf", "sgi", "sox", "spdif", "spx", "srt", "ssa", "sub", "sun", "sunras", "sup", "sw", "swf", "tco", "tga", "thd", "tif", "tiff", "ts", "tta", "ttml", "tun", "ub", "ul", "uw", "vag", "vbn", "vc1", "vc2", "vob", "voc", "vtt", "vvc", "w64", "wav", "wbmp", "webm", "webp", "wma", "wmv", "wtv", "wv", "xbm", "xface", "xml", "xwd", "y", "y4m", "yuv"},
	},
	converterGraphicsMagick: {
		from: []string{"3fr", "8bim", "8bimtext", "8bimwtext", "app1", "app1jpeg", "art", "arw", "avs", "b", "bie", "bigtiff", "bmp", "c", "cals", "caption", "cin", "cmyk", "cmyka", "cr2", "crw", "cur", "cut", "dcm", "dcr", "dcx", "dng", "dpx", "epdf", "epi", "eps", "epsf", "epsi", "ept", "ept2", "ept3", "erf", "exif", "fax", "file", "fits", "fractal", "ftp", "g", "gif", "gif87", "gradient", "gray", "graya", "heic", "heif", "hrz", "http", "icb", "icc", "icm", "ico", "icon", "identity", "image", "iptc", "iptctext", "iptcwtext", "jbg", "jbig", "jng", "jnx", "jpeg", "jpg", "k", "k25", "kdc", "label", "m", "mac", "map", "mat", "mef", "miff", "mng", "mono", "mpc", "mrw", "msl", "mtv", "mvg", "nef", "null", "o", "orf", "otb", "p7", "pal", "palm", "pam", "pbm", "pcd", "pcds", "pct", "pcx", "pdb", "pdf", "pef", "pfa", "pfb", "pgm", "picon", "pict", "pix", "plasma", "png", "png00", "png24", "png32", "png48", "png64", "png8", "pnm", "ppm", "ps", "ptif", "pwp", "r", "raf", "ras", "rgb", "rgba", "rla", "rle", "sct", "sfw", "sgi", "sr2", "srf", "stegano", "sun", "svg", "svgz", "text", "tga", "tif", "tiff", "tile", "tim", "topol", "ttf", "txt", "uyvy", "vda", "vicar", "vid", "viff", "vst", "wbmp", "webp", "wmf", "wpg", "x3f", "xbm", "xc", "xcf", "xmp", "xpm", "xv", "xwd", "y", "yuv"},
		to:   []string{"8bim", "8bimtext", "8bimwtext", "app1", "app1jpeg", "art", "avs", "b", "bie", "bigtiff", "bmp", "bmp2", "bmp3", "brf", "c", "cals", "cin", "cmyk", "cmyka", "dcx", "dpx", "epdf", "epi", "eps", "eps2", "eps3", "epsf", "epsi", "ept", "ept2", "ept3", "exif", "fax", "fits", "g", "gif", "gif87", "gray", "graya", "histogram", "html", "icb", "icc", "icm", "info", "iptc", "iptctext", "iptcwtext", "isobrl", "isobrl6", "jbg", "jbig", "jng", "jpeg", "k", "m", "m2v", "map", "mat", "matte", "miff", "mng", "mono", "mpc", "mpeg", "mpg", "msl", "mtv", "mvg", "null", "o", "otb", "p7", "pal", "pam", "pbm", "pcd", "pcds", "pcl", "pct", "pcx", "pdb", "pdf", "pgm", "picon", "pict", "png", "png00", "png24", "png32", "png48", "png64", "png8", "pnm", "ppm", "preview", "ps", "ps2", "ps3", "ptif", "r", "ras", "rgb", "rgba", "sgi", "shtml", "sun", "text", "tga", "tiff", "txt", "ubrl", "ubrl6", "uil", "uyvy", "vda", "vicar", "vid", "viff", "vst", "wbmp", "webp", "x", "xbm", "xmp", "xpm", "xv", "xwd", "y", "yuv"},
	},
	converterImageMagick: {
		from: []string{"3fr", "3g2", "3gp", "aai", "ai", "apng", "art", "arw", "avci", "avi", "avif", "avs", "bayer", "bayera", "bgr", "bgra", "bgro", "bmp", "bmp2", "bmp3", "cal", "cals", "canvas", "caption", "cin", "clip", "clipboard", "cmyk", "cmyka", "cr2", "cr3", "crw", "cube", "cur", "cut", "data", "dcm", "dcr", "dcraw", "dcx", "dds", "dfont", "dng", "dpx", "dxt1", "dxt5", "emf", "epdf", "epi", "eps", "epsf", "epsi", "ept", "ept2", "ept3", "erf", "exr", "farbfeld", "fax", "ff", "fff", "file", "fits", "fl32", "flif", "flv", "fractal", "ftp", "fts", "ftxt", "g3", "g4", "gif", "gif87", "gradient", "gray", "graya", "group4", "hald", "hdr", "heic", "heif", "hrz", "http", "https", "icb", "ico", "icon", "iiq", "inline", "ipl", "j2c", "j2k", "jng", "jnx", "jp2", "jpc", "jpe", "jpeg", "jpg", "jpm", "jps", "jpt", "jxl", "k25", "kdc", "label", "m2v", "m4v", "mac", "map", "mask", "mat", "mdc", "mef", "miff", "mkv", "mng", "mono", "mos", "mov", "mp4", "mpc", "mpeg", "mpg", "mpo", "mrw", "msl", "msvg", "mtv", "mvg", "nef", "nrw", "null", "ora", "orf", "otb", "otf", "pal", "palm", "pam", "pango", "pattern", "pbm", "pcd", "pcds", "pcl", "pct", "pcx", "pdb", "pdf", "pdfa", "pef", "pes", "pfa", "pfb", "pfm", "pgm", "pgx", "phm", "picon", "pict", "pix", "pjpeg", "plasma", "png", "png00", "png24", "png32", "png48", "png64", "png8", "pnm", "pocketmod", "ppm", "ps", "psb", "psd", "ptif", "pwp", "qoi", "radial", "raf", "ras", "raw", "rgb", "rgb565", "rgba", "rgbo", "rgf", "rla", "rle", "rmf", "rsvg", "rw2", "rwl", "scr", "screenshot", "sct", "sfw", "sgi", "six", "sixel", "sr2", "srf", "srw", "stegano", "sti", "strimg", "sun", "svg", "svgz", "text", "tga", "tiff", "tiff64", "tile", "tim", "tm2", "ttc", "ttf", "txt", "uyvy", "vda", "vicar", "vid", "viff", "vips", "vst", "wbmp", "webm", "webp", "wmf", "wmv", "wpg", "x3f", "xbm", "xc", "xcf", "xpm", "xps", "xv", "ycbcr", "ycbcra", "yuv"},
		to:   []string{"aai", "ai", "apng", "art", "ashlar", "avif", "avs", "bayer", "bayera", "bgr", "bgra", "bgro", "bmp", "bmp2", "bmp3", "brf", "cal", "cals", "cin", "cip", "clip", "clipboard", "cmyk", "cmyka", "cur", "data", "dcx", "dds", "dpx", "dxt1", "dxt5", "epdf", "epi", "eps", "eps2", "eps3", "epsf", "epsi", "ept", "ept2", "ept3", "exr", "farbfeld", "fax", "ff", "fits", "fl32", "flif", "flv", "fts", "ftxt", "g3", "g4", "gif", "gif87", "gray", "graya", "group4", "hdr", "histogram", "hrz", "htm", "html", "icb", "ico", "icon", "info", "inline", "ipl", "isobrl", "isobrl6", "j2c", "j2k", "jng", "jp2", "jpc", "jpe", "jpeg", "jpg", "jpm", "jps", "jpt", "json", "jxl", "m2v", "m4v", "map", "mask", "mat", "matte", "miff", "mkv", "mng", "mono", "mov", "mp4", "mpc", "mpeg", "mpg", "msl", "msvg", "mtv", "mvg", "null", "otb", "pal", "palm", "pam", "pbm", "pcd", "pcds", "pcl", "pct", "pcx", "pdb", "pdf", "pdfa", "pfm", "pgm", "pgx", "phm", "picon", "pict", "pjpeg", "png", "png00", "png24", "png32", "png48", "png64", "png8", "pnm", "pocketmod", "ppm", "ps", "ps2", "ps3", "psb", "psd", "ptif", "qoi", "ras", "rgb", "rgba", "rgbo", "rgf", "rsvg", "sgi", "shtml", "six", "sixel", "sparse", "strimg", "sun", "svg", "svgz", "tga", "thumbnail", "tiff", "tiff64", "txt", "ubrl", "ubrl6", "uil", "uyvy", "vda", "vicar", "vid", "viff", "vips", "vst", "wbmp", "webm", "webp", "wmv", "wpg", "xbm", "xpm", "xv", "yaml", "ycbcr", "ycbcra", "yuv"},
	},
	converterInkscape: {
		from: []string{"svg", "pdf", "eps", "ps", "wmf", "emf", "png"},
		to:   []string{"dxf", "emf", "eps", "fxg", "gpl", "hpgl", "html", "odg", "pdf", "png", "pov", "ps", "sif", "svg", "svgz", "tex", "wmf"},
	},
	converterLibheif: {
		from: []string{"avci", "avcs", "avif", "h264", "heic", "heics", "heif", "heifs", "hif", "mkv", "mp4"},
		to:   []string{"jpeg", "png", "y4m"},
	},
	converterLibjxl: {
		from: []string{"jxl", "apng", "exr", "gif", "jpeg", "pam", "pfm", "pgm", "pgx", "png", "ppm"},
		to:   []string{"apng", "exr", "jpeg", "pam", "pfm", "pgm", "pgx", "png", "ppm", "jxl"},
	},
	converterLibreOffice: {
		from: []string{"602", "abw", "cwk", "doc", "docm", "docx", "dot", "dotx", "dotm", "epub", "fb2", "fodt", "htm", "html", "hwp", "mcw", "mw", "mwd", "lwp", "lrf", "odt", "ott", "pages", "pdf", "psw", "rtf", "sdw", "stw", "sxw", "tab", "txt", "wn", "wpd", "wps", "wpt", "wri", "xhtml", "xml", "zabw", "csv", "ods", "tsv", "xls", "xlsx"},
		to:   []string{"doc", "docm", "docx", "dot", "dotx", "dotm", "epub", "fodt", "htm", "html", "odt", "ott", "pdf", "rtf", "tab", "txt", "wps", "wpt", "xhtml", "xml", "ods", "xls", "xlsx", "ots", "xlsm", "xlt", "xltm"},
	},
	converterMarkitdown: {
		from: []string{"pdf", "powerpoint", "excel", "docx", "pptx", "html"},
		to:   []string{"md"},
	},
	converterMsgconvert: {
		from: []string{"msg"},
		to:   []string{"eml"},
	},
	converterPandoc: {
		from: []string{"textile", "tikiwiki", "tsv", "twiki", "typst", "vimwiki", "biblatex", "bibtex", "bits", "commonmark", "commonmark_x", "creole", "csljson", "csv", "djot", "docbook", "docx", "dokuwiki", "endnotexml", "epub", "fb2", "gfm", "haddock", "html", "ipynb", "jats", "jira", "json", "latex", "man", "markdown", "markdown_mmd", "markdown_phpextra", "markdown_strict", "mediawiki", "muse", "pandoc native", "opml", "org", "ris", "rst", "rtf", "t2t"},
		to:   []string{"tei", "texinfo", "textile", "typst", "xwiki", "zimwiki", "asciidoc", "asciidoc_legacy", "asciidoctor", "beamer", "biblatex", "bibtex", "chunkedhtml", "commonmark", "commonmark_x", "context", "csljson", "djot", "docbook", "docbook4", "docbook5", "docx", "dokuwiki", "dzslides", "epub", "epub2", "epub3", "fb2", "gfm", "haddock", "html", "html4", "html5", "icml", "ipynb", "jats", "jats_archiving", "jats_articleauthoring", "jats_publishing", "jira", "json", "latex", "man", "markdown", "markdown_mmd", "markdown_phpextra", "markdown_strict", "markua", "mediawiki", "ms", "muse", "pandoc native", "odt", "opendocument", "opml", "org", "pdf", "plain", "pptx", "revealjs", "rst", "rtf", "s5", "slideous", "slidy"},
	},
	converterPotrace: {
		from: []string{"pnm", "pbm", "pgm", "bmp"},
		to:   []string{"svg", "pdf", "pdfpage", "eps", "postscript", "ps", "dxf", "geojson", "pgm", "gimppath", "xfig"},
	},
	converterResvg: {
		from: []string{"svg"},
		to:   []string{"png"},
	},
	converterVCF: {
		from: []string{"vcf"},
		to:   []string{"csv"},
	},
	converterVips: {
		from: []string{"avif", "bif", "csv", "exr", "fits", "gif", "hdr.gz", "hdr", "heic", "heif", "img.gz", "img", "j2c", "j2k", "jp2", "jpeg", "jpx", "jxl", "mat", "mrxs", "ndpi", "nia.gz", "nia", "nii.gz", "nii", "pdf", "pfm", "pgm", "pic", "png", "ppm", "raw", "scn", "svg", "svs", "svslide", "szi", "tif", "tiff", "v", "vips", "vms", "vmu", "webp", "zip"},
		to:   []string{"avif", "dzi", "fits", "gif", "hdr.gz", "heic", "heif", "img.gz", "j2c", "j2k", "jp2", "jpeg", "jpx", "jxl", "mat", "nia.gz", "nia", "nii.gz", "nii", "png", "tiff", "vips", "webp"},
	},
	converterVtracer: {
		from: []string{"jpg", "jpeg", "png", "bmp", "gif", "tiff", "tif", "webp"},
		to:   []string{"svg"},
	},
	converterXeLaTeX: {
		from: []string{"tex", "latex"},
		to:   []string{"pdf"},
	},
}
