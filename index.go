package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// GenerateIndex 生成导航页面
func GenerateIndex(outputDir, docName string, pages []PageItem) error {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>`)
	sb.WriteString(html.EscapeString(docName))
	sb.WriteString(`</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 20px; }
h1 { text-align: center; margin-bottom: 24px; font-size: 22px; color: #1a1a1a; }
.stats { text-align: center; color: #888; font-size: 14px; margin-bottom: 24px; }
.nav { max-width: 800px; margin: 0 auto; }
.folder { margin-bottom: 16px; }
.folder-name { font-size: 15px; font-weight: 600; color: #555; padding: 8px 12px; background: #e8e8e8; border-radius: 6px; margin-bottom: 4px; }
.page-list { list-style: none; }
.page-list li { border-bottom: 1px solid #eee; }
.page-list a { display: block; padding: 10px 16px; color: #1677ff; text-decoration: none; font-size: 14px; transition: background .15s; border-radius: 4px; }
.page-list a:hover { background: #e6f4ff; }
.page-list .indent-1 { padding-left: 32px; }
.page-list .indent-2 { padding-left: 48px; }
.search-box { max-width: 800px; margin: 0 auto 20px; }
.search-box input { width: 100%; padding: 10px 14px; border: 1px solid #ddd; border-radius: 8px; font-size: 14px; outline: none; }
.search-box input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.1); }
</style>
</head>
<body>
<h1>`)
	sb.WriteString(html.EscapeString(docName))
	sb.WriteString(`</h1>
<div class="stats">共 `)
	sb.WriteString(fmt.Sprintf("%d", len(pages)))
	sb.WriteString(` 个页面</div>
<div class="search-box"><input type="text" id="search" placeholder="搜索页面..." oninput="filterPages(this.value)"></div>
<div class="nav">
<ul class="page-list" id="pageList">
`)

	var currentFolder string
	for _, p := range pages {
		// 从 DirPath 提取顶级目录作为分组
		topFolder := ""
		if p.DirPath != "" {
			parts := strings.SplitN(p.DirPath, "/", 2)
			topFolder = parts[0]
		}

		if topFolder != "" && topFolder != currentFolder {
			if currentFolder != "" {
				sb.WriteString("</ul></div>\n")
			}
			currentFolder = topFolder
			sb.WriteString(fmt.Sprintf("<div class=\"folder\"><div class=\"folder-name\">%s</div><ul class=\"page-list\">\n", html.EscapeString(topFolder)))
		}

		indentClass := ""
		if p.Level > 1 {
			indentClass = fmt.Sprintf(" class=\"indent-%d\"", min(p.Level-1, 2))
		}
		// 链接路径: html/<DirPath>/<page_name>/<filename>.html
		pageDir := SanitizeFilename(p.Name)
		if p.DirPath != "" {
			pageDir = SanitizeDirPath(p.DirPath) + "/" + pageDir
		}
		linkPath := "html/" + pageDir + "/" + p.Filename
		sb.WriteString(fmt.Sprintf("<li%s><a href=\"%s\">%s</a></li>\n",
			indentClass,
			html.EscapeString(linkPath),
			html.EscapeString(p.Name),
		))
	}
	if currentFolder != "" {
		sb.WriteString("</ul></div>\n")
	}

	sb.WriteString(`</ul>
</div>
<script>
function filterPages(q) {
  q = q.toLowerCase();
  document.querySelectorAll('.page-list li').forEach(li => {
    li.style.display = li.textContent.toLowerCase().includes(q) ? '' : 'none';
  });
  document.querySelectorAll('.folder').forEach(f => {
    const visible = f.querySelectorAll('li:not([style*="display: none"])').length;
    f.style.display = visible > 0 ? '' : 'none';
  });
}
</script>
</body>
</html>`)

	outPath := filepath.Join(outputDir, "index.html")
	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
