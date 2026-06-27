package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reDataSrcImg    = regexp.MustCompile(`(<img[^>]*?)\s+data-src=`)
	reDataSrcScript = regexp.MustCompile(`(<script[^>]*?)\s+data-src=`)
	reDataSrcLink   = regexp.MustCompile(`(<link[^>]*?)\s+data-src=`)
	reBodyHidden    = regexp.MustCompile(`<body[^>]*style="[^"]*display\s*:\s*none[^"]*opacity\s*:\s*0[^"]*"[^>]*>`)
	reBgImageURL    = regexp.MustCompile(`background-image\b[^);]*?url\s*\(\s*["']?([^"')\s]+)["']?\s*\)`)
)

// FixHTML 修复蓝湖下载的 HTML 文件
func FixHTML(content string) string {
	// data-src → src (img, script)
	content = reDataSrcImg.ReplaceAllString(content, `${1} src=`)
	content = reDataSrcScript.ReplaceAllString(content, `${1} src=`)
	// data-src → href (link)
	content = reDataSrcLink.ReplaceAllString(content, `${1} href=`)

	// 也处理已有 src/href 但值为 data-src 的情况
	content = strings.ReplaceAll(content, `data-src="`, `src="`)

	// 移除 body 隐藏样式
	content = reBodyHidden.ReplaceAllString(content, "<body>")

	// 注入 lanhu_Axure_Mapping_Data 桩函数
	stubScript := `<script>
function lanhu_Axure_Mapping_Data(data) { return data; }
</script>`
	if strings.Contains(content, "</head>") {
		content = strings.Replace(content, "</head>", stubScript+"\n</head>", 1)
	}

	return content
}

// FixCSS 修复 CSS 中的 background-image URL，返回修复后的内容和需要下载的图片映射
// cssDir 是 CSS 文件所在目录的绝对路径
// imgDir 是图片存放目录的绝对路径
func FixCSS(content string, cssDir string, imgDir string) (string, map[string]string) {
	downloads := make(map[string]string) // remote_url → absolute_path

	matches := reBgImageURL.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		imgURL := m[1]
		// 跳过已经是本地路径的
		if strings.HasPrefix(imgURL, "./") || strings.HasPrefix(imgURL, "../") || strings.HasPrefix(imgURL, "/") {
			continue
		}
		// 跳过 data: URL
		if strings.HasPrefix(imgURL, "data:") {
			continue
		}

		// 构建完整URL
		fullURL := imgURL
		if !strings.HasPrefix(imgURL, "http") {
			fullURL = CDNURL + "/" + strings.TrimPrefix(imgURL, "./")
		}

		// 下载到 imgDir，用文件名作为本地名
		imgName := strings.TrimPrefix(imgURL, "./")
		absImgPath := filepath.Join(imgDir, imgName)
		downloads[fullURL] = absImgPath

		// CSS 中用相对路径引用图片
		relPath, _ := filepath.Rel(cssDir, absImgPath)
		content = strings.Replace(content, m[0], "background-image: url("+relPath+")", 1)
	}

	return content, downloads
}

// FixDataJs 修复 data.js，去除 lanhu_Axure_Mapping_Data 包装
func FixDataJs(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "lanhu_Axure_Mapping_Data(") && strings.HasSuffix(content, ")") {
		content = content[len("lanhu_Axure_Mapping_Data(") : len(content)-1]
	}
	return content
}

// SanitizeFilename 清理文件名中的特殊字符
func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}

// SanitizeDirPath 清理目录路径中的特殊字符（保留 / 分隔符）
func SanitizeDirPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = SanitizeFilename(part)
	}
	return strings.Join(parts, "/")
}
