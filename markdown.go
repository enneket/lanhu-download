package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var reMultiBlank = regexp.MustCompile(`\n{3,}`)

// ConvertHTMLToMarkdown 从 Axure HTML 提取结构化 Markdown
func ConvertHTMLToMarkdown(htmlPath, pageName string) (string, error) {
	f, err := os.Open(htmlPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return "", fmt.Errorf("解析HTML失败: %w", err)
	}

	body := doc.Find("body")
	if body.Length() == 0 {
		return "", fmt.Errorf("未找到body元素")
	}

	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(pageName)
	sb.WriteString("\n\n")

	// 递归遍历 DOM 树，按顺序提取内容
	extractNode(body, &sb, 0)

	// 清理
	markdown := sb.String()
	markdown = reMultiBlank.ReplaceAllString(markdown, "\n\n")
	markdown = strings.TrimSpace(markdown)

	if len(markdown) < 100 {
		markdown += "\n\n> 此页面主要是图片/交互组件，文本内容较少"
	}

	return markdown, nil
}

// extractNode 递归提取节点内容
func extractNode(s *goquery.Selection, sb *strings.Builder, depth int) {
	// 防止递归过深
	if depth > 50 {
		return
	}

	s.Contents().Each(func(i int, child *goquery.Selection) {
		// 文本节点
		if goquery.NodeName(child) == "#text" {
			text := strings.TrimSpace(child.Text())
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
			return
		}

		// 元素节点
		tag := goquery.NodeName(child)
		cls := child.AttrOr("class", "")

		// 跳过隐藏元素
		style := child.AttrOr("style", "")
		if strings.Contains(style, "display:none") || strings.Contains(style, "visibility: hidden") {
			return
		}
		if strings.Contains(cls, "ax_default_hidden") {
			return
		}

		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			text := cleanText(child.Text())
			if text != "" {
				sb.WriteString("\n" + strings.Repeat("#", 4) + " " + text + "\n\n")
			}
			return

		case "img":
			alt := child.AttrOr("alt", "")
			src := child.AttrOr("src", "")
			if src != "" {
				if alt == "" {
					alt = "图片"
				}
				sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", alt, src))
			}
			return

		case "table":
			extractTable(child, sb)
			return

		case "br":
			sb.WriteString("\n")
			return

		case "hr":
			sb.WriteString("\n---\n\n")
			return
		}

		// 根据 class 处理特殊元素
		switch {
		case strings.Contains(cls, "_一级标题"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("\n## " + text + "\n\n")
			}
			return

		case strings.Contains(cls, "_二级标题"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("\n### " + text + "\n\n")
			}
			return

		case strings.Contains(cls, "_三级标题"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("\n#### " + text + "\n\n")
			}
			return

		case strings.Contains(cls, "_段落"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString(text + "\n\n")
			}
			return

		case strings.Contains(cls, "label"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("**" + text + ":** ")
			}
			return

		case strings.Contains(cls, "link_button"), strings.Contains(cls, "primary_button"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("`" + text + "` ")
			}
			return

		case strings.Contains(cls, "checkbox"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("- [ ] " + text + "\n")
			}
			return

		case strings.Contains(cls, "radio_button"):
			text := extractTextFromDiv(child)
			checked := strings.Contains(cls, "selected")
			if text != "" {
				if checked {
					sb.WriteString("- [x] " + text + "\n")
				} else {
					sb.WriteString("- [ ] " + text + "\n")
				}
			}
			return

		case strings.Contains(cls, "text_field1"), strings.Contains(cls, "droplist"):
			text := extractTextFromDiv(child)
			if text != "" {
				sb.WriteString("`" + text + "`")
			}
			return
		}

		// 普通文本 div（没有特殊类名的）
		if strings.Contains(cls, "text") && !strings.Contains(cls, "text_field") {
			text := extractTextFromDiv(child)
			if text != "" && len(text) > 1 {
				sb.WriteString(text + "\n")
			}
			return
		}

		// 递归处理子节点
		extractNode(child, sb, depth+1)
	})
}

// extractTextFromDiv 从 div 中提取文本（只取 _text 子元素或直接文本）
func extractTextFromDiv(s *goquery.Selection) string {
	// 找 _text 子元素
	textDiv := s.Find("[id$='_text']")
	if textDiv.Length() > 0 {
		text := strings.TrimSpace(textDiv.First().Text())
		text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
		return text
	}
	// 直接取文本
	text := strings.TrimSpace(s.Text())
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return text
}

// extractTable 提取表格
func extractTable(table *goquery.Selection, sb *strings.Builder) {
	var rows [][]string

	table.Find("tr").Each(func(i int, tr *goquery.Selection) {
		var row []string
		tr.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			text := cleanText(cell.Text())
			if text == "" {
				text = " "
			}
			row = append(row, text)
		})
		if len(row) > 0 {
			rows = append(rows, row)
		}
	})

	if len(rows) == 0 {
		// 可能是 Axure 的 table_cell 结构
		table.Find(".table_cell").Each(func(i int, cell *goquery.Selection) {
			text := cleanText(cell.Text())
			if text == "" {
				text = " "
			}
			// 简单按顺序排列
			if len(rows) == 0 {
				rows = append(rows, []string{})
			}
			rows[0] = append(rows[0], text)
		})
	}

	if len(rows) < 2 {
		return
	}

	// 生成 markdown 表格
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}

	sb.WriteString("\n|")
	for c := 0; c < cols; c++ {
		sb.WriteString(" ")
		if c < len(rows[0]) {
			sb.WriteString(rows[0][c])
		}
		sb.WriteString(" |")
	}
	sb.WriteString("\n|")
	for c := 0; c < cols; c++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for r := 1; r < len(rows); r++ {
		sb.WriteString("|")
		for c := 0; c < cols; c++ {
			sb.WriteString(" ")
			if c < len(rows[r]) {
				sb.WriteString(rows[r][c])
			}
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

// cleanText 清理文本
func cleanText(text string) string {
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return text
}

// ConvertAndSaveMarkdown 转换 HTML 为 Markdown 并保存
func ConvertAndSaveMarkdown(htmlPath string) (string, error) {
	pageName := strings.TrimSuffix(filepath.Base(htmlPath), ".html")
	markdown, err := ConvertHTMLToMarkdown(htmlPath, pageName)
	if err != nil {
		return "", err
	}

	mdPath := strings.TrimSuffix(htmlPath, ".html") + ".md"
	if err := os.WriteFile(mdPath, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("保存Markdown失败: %w", err)
	}
	return mdPath, nil
}

// GenerateIndexMD 生成 Markdown 导航文件
func GenerateIndexMD(outputDir, docName string, pages []PageItem) error {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(docName)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("共 %d 个页面\n\n", len(pages)))
	sb.WriteString("---\n\n")

	var currentFolder string
	for _, p := range pages {
		if p.Folder != "" && p.Folder != currentFolder && p.Folder != "根目录" {
			if currentFolder != "" {
				sb.WriteString("\n")
			}
			currentFolder = p.Folder
			sb.WriteString("## ")
			sb.WriteString(currentFolder)
			sb.WriteString("\n\n")
		}

		indent := ""
		if p.Level > 0 {
			indent = strings.Repeat("  ", p.Level)
		}

		pageDir := SanitizeFilename(p.Name)
		if p.Folder != "" && p.Folder != "根目录" {
			pageDir = SanitizeFilename(p.Folder) + "/" + pageDir
		}
		linkPath := pageDir + "/" + strings.TrimSuffix(p.Filename, ".html") + ".md"

		sb.WriteString(fmt.Sprintf("%s- [%s](%s)\n", indent, p.Name, linkPath))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("*Generated by lanhu-download*\n")

	outPath := filepath.Join(outputDir, "index.md")
	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}
