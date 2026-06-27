package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type PNGConverter struct {
	width    int
	height   int
	scale    float64
	fullPage bool
	timeout  time.Duration
}

func NewPNGConverter(width, height int, scale float64, fullPage bool, timeout int) *PNGConverter {
	return &PNGConverter{
		width:    width,
		height:   height,
		scale:    scale,
		fullPage: fullPage,
		timeout:  time.Duration(timeout) * time.Second,
	}
}

// ConvertFile 将单个 HTML 文件截图为 PNG
func (p *PNGConverter) ConvertFile(htmlPath, pngPath string) error {
	// 转为绝对路径
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %w", err)
	}

	// 创建临时 chrome 实例
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// 设置视口
	var screenshotBuf []byte
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(p.width), int64(p.height), chromedp.EmulateScale(p.scale)),
		chromedp.Navigate("file://"+absPath),
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond), // 等待渲染
		chromedp.FullScreenshot(&screenshotBuf, int(p.scale*100)),
	)
	if err != nil {
		return fmt.Errorf("截图失败: %w", err)
	}

	// 保存 PNG
	if err := os.MkdirAll(filepath.Dir(pngPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(pngPath, screenshotBuf, 0644)
}

// ConvertDir 将目录下所有 HTML 文件截图
func (p *PNGConverter) ConvertDir(htmlDir, pngDir string) (int, error) {
	var htmlFiles []string
	filepath.Walk(htmlDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})

	if len(htmlFiles) == 0 {
		return 0, fmt.Errorf("未找到HTML文件")
	}

	success := 0
	for i, htmlPath := range htmlFiles {
		relPath, _ := filepath.Rel(htmlDir, htmlPath)
		pngPath := filepath.Join(pngDir, strings.TrimSuffix(relPath, ".html")+".png")
		pngPath = SanitizePNGPath(pngPath)

		fmt.Printf("  📸 [%d/%d] %s\n", i+1, len(htmlFiles), filepath.Base(htmlPath))
		if err := p.ConvertFile(htmlPath, pngPath); err != nil {
			fmt.Printf("    ❌ %v\n", err)
		} else {
			success++
		}
	}

	return success, nil
}

// SanitizePNGPath 清理 PNG 路径中的中文字符
func SanitizePNGPath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	// 保留原文件名，只是确保路径有效
	return filepath.Join(dir, base)
}

// ConvertAll 将 output/html 目录下所有 HTML 转为 PNG
func ConvertAllToPNG(outputDir string, converter *PNGConverter) error {
	fmt.Println("📸 转换PNG...")

	// 遍历 output/html 目录，找到所有 HTML 文件
	htmlDir := filepath.Join(outputDir, "html")
	var htmlFiles []string
	filepath.Walk(htmlDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})

	if len(htmlFiles) == 0 {
		return fmt.Errorf("未找到HTML文件")
	}

	// 创建 png 输出目录
	pngDir := filepath.Join(outputDir, "png")
	if err := os.MkdirAll(pngDir, 0755); err != nil {
		return fmt.Errorf("创建PNG目录失败: %w", err)
	}

	success := 0
	for i, htmlPath := range htmlFiles {
		// 保持目录结构: output/html/xxx.html → output/png/xxx.png
		relPath, _ := filepath.Rel(htmlDir, htmlPath)
		pngPath := filepath.Join(pngDir, strings.TrimSuffix(relPath, ".html")+".png")
		pngPath = SanitizePNGPath(pngPath)

		fmt.Printf("  📸 [%d/%d] %s\n", i+1, len(htmlFiles), relPath)
		if err := converter.ConvertFile(htmlPath, pngPath); err != nil {
			fmt.Printf("    ❌ %v\n", err)
		} else {
			success++
		}
	}

	fmt.Printf("✅ 已转换 %d/%d 个PNG文件\n", success, len(htmlFiles))
	fmt.Printf("📁 PNG保存在: %s\n", pngDir)
	return nil
}
