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

// ConvertHTMLToMarkdown 基于 Axure CSS 类名提取结构化 Markdown
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

	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(pageName)
	sb.WriteString("\n\n")

	// 提取一级标题作为页面主标题
	h1Seen := make(map[string]bool)
	doc.Find("[class*='_一级标题']").Each(func(i int, s *goquery.Selection) {
		text := extractText(s)
		if text != "" && !h1Seen[text] {
			h1Seen[text] = true
			sb.WriteString("## ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	// 提取二级标题作为子标题
	h2Seen := make(map[string]bool)
	doc.Find("[class*='_二级标题']").Each(func(i int, s *goquery.Selection) {
		text := extractText(s)
		if text != "" && !h2Seen[text] {
			h2Seen[text] = true
			sb.WriteString("### ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	// 提取段落内容
	paraSeen := make(map[string]bool)
	doc.Find("[class*='_段落']").Each(func(i int, s *goquery.Selection) {
		text := extractText(s)
		if text != "" && !paraSeen[text] && len(text) > 2 {
			paraSeen[text] = true
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	// 提取表格
	tables := extractTables(doc)
	for _, table := range tables {
		sb.WriteString(table)
		sb.WriteString("\n\n")
	}

	// 提取表单字段
	forms := extractFormFields(doc)
	if len(forms) > 0 {
		sb.WriteString("**表单字段:**\n\n")
		for _, f := range forms {
			sb.WriteString(f)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 提取按钮
	buttons := extractButtons(doc)
	if len(buttons) > 0 {
		sb.WriteString("**操作按钮:** ")
		sb.WriteString(strings.Join(buttons, " | "))
		sb.WriteString("\n\n")
	}

	// 清理
	markdown := sb.String()
	markdown = reMultiBlank.ReplaceAllString(markdown, "\n\n")
	markdown = strings.TrimSpace(markdown)

	if len(markdown) < 100 {
		markdown += "\n\n> 此页面主要是图片/交互组件，文本内容较少"
	}

	return markdown, nil
}

// extractText 从元素中提取文本（只取 _text 子元素）
func extractText(s *goquery.Selection) string {
	// 优先找 _text 子元素
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

// extractTables 提取表格内容
func extractTables(doc *goquery.Document) []string {
	var tables []string

	// 查找包含多个 table_cell 的容器
	doc.Find("div").Each(func(i int, s *goquery.Selection) {
		cells := s.Children().Filter(".table_cell")
		if cells.Length() < 4 {
			return
		}

		// 收集所有单元格文本
		var rows [][]string
		var currentRow []string

		cells.Each(func(j int, cell *goquery.Selection) {
			text := extractText(cell)
			if text == "" {
				text = " "
			}
			currentRow = append(currentRow, text)

			// 估算列数并分行
			cols := estimateCols(cells.Length())
			if len(currentRow) >= cols {
				rows = append(rows, currentRow)
				currentRow = nil
			}
		})
		if len(currentRow) > 0 {
			rows = append(rows, currentRow)
		}

		if len(rows) < 2 {
			return
		}

		// 生成 Markdown 表格
		cols := len(rows[0])
		var table strings.Builder
		table.WriteString("|")
		for c := 0; c < cols; c++ {
			table.WriteString(" ")
			if c < len(rows[0]) {
				table.WriteString(rows[0][c])
			}
			table.WriteString(" |")
		}
		table.WriteString("\n|")
		for c := 0; c < cols; c++ {
			table.WriteString(" --- |")
		}
		table.WriteString("\n")
		for r := 1; r < len(rows); r++ {
			table.WriteString("|")
			for c := 0; c < cols; c++ {
				table.WriteString(" ")
				if c < len(rows[r]) {
					table.WriteString(rows[r][c])
				}
				table.WriteString(" |")
			}
			table.WriteString("\n")
		}

		tables = append(tables, table.String())
	})

	return tables
}

// estimateCols 估算表格列数
func estimateCols(n int) int {
	for _, cols := range []int{2, 3, 4, 5, 6, 7, 8} {
		if n%cols == 0 && n/cols >= 2 {
			return cols
		}
	}
	return 4
}

// extractFormFields 提取表单字段
func extractFormFields(doc *goquery.Document) []string {
	var fields []string
	seen := make(map[string]bool)

	doc.Find(".label").Each(func(i int, s *goquery.Selection) {
		text := extractText(s)
		if text == "" || seen[text] || len(text) > 30 {
			return
		}
		seen[text] = true

		// 查找相邻的输入/选择元素
		parent := s.Parent()
		val := ""
		parent.Find(".text_field1, .droplist").Each(func(j int, inp *goquery.Selection) {
			v := extractText(inp)
			if v != "" {
				val = v
			}
		})
		// 检查选中的 radio/checkbox
		parent.Find(".selected").Each(func(j int, sel *goquery.Selection) {
			v := extractText(sel)
			if v != "" {
				val = v
			}
		})

		if val != "" {
			fields = append(fields, fmt.Sprintf("- **%s:** %s", text, val))
		} else {
			fields = append(fields, fmt.Sprintf("- **%s:** _（待填写）_", text))
		}
	})

	return fields
}

// extractButtons 提取按钮
func extractButtons(doc *goquery.Document) []string {
	var buttons []string
	seen := make(map[string]bool)

	doc.Find(".link_button, .primary_button").Each(func(i int, s *goquery.Selection) {
		text := extractText(s)
		if text != "" && !seen[text] && len(text) < 20 {
			buttons = append(buttons, fmt.Sprintf("[%s]", text))
			seen[text] = true
		}
	})

	return buttons
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
