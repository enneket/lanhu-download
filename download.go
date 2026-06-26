package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type Downloader struct {
	client      *LanhuClient
	outputDir   string
	concurrency int
	downloaded  sync.Map // sign_md5 → local path (去重)
}

func NewDownloader(client *LanhuClient, outputDir string, concurrency int) *Downloader {
	return &Downloader{
		client:      client,
		outputDir:   outputDir,
		concurrency: concurrency,
	}
}

// DownloadAll 下载所有页面
func (d *Downloader) DownloadAll(params *URLParams) error {
	// 1. 获取文档信息
	fmt.Println("📡 获取文档信息...")
	docInfo, err := d.client.GetDocInfo(params.ProjectID, params.DocID)
	if err != nil {
		return fmt.Errorf("获取文档信息失败: %w", err)
	}
	if docInfo.Data == nil || len(docInfo.Data.Versions) == 0 {
		return fmt.Errorf("文档没有版本信息")
	}

	// 2. 获取 mapping JSON
	version := docInfo.Data.Versions[0]
	jsonURL := version.JSONURL
	if jsonURL == "" {
		return fmt.Errorf("版本缺少 json_url")
	}
	fmt.Println("📡 获取页面列表...")
	mapping, err := d.client.GetProjectMapping(jsonURL)
	if err != nil {
		return err
	}

	// 3. 提取页面列表
	pages := extractPages(mapping)
	if len(pages) == 0 {
		return fmt.Errorf("没有找到可下载的页面")
	}

	fmt.Printf("📄 共 %d 个页面\n", len(pages))

	// 4. 如果指定了 pageId，只下载该页面
	if params.PageID != "" {
		filtered := filterByPageID(mapping, pages, params.PageID)
		if len(filtered) == 0 {
			fmt.Printf("⚠️  未找到 pageId=%s 对应的页面，下载全部页面\n", params.PageID)
		} else {
			pages = filtered
		}
	}

	// 5. 下载页面
	if err := os.MkdirAll(d.outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	err = d.downloadPages(pages, mapping)
	if err != nil {
		return err
	}

	// 6. 生成 index.html
	if len(pages) > 1 {
		fmt.Println("📝 生成导航页...")
		if err := GenerateIndex(d.outputDir, docInfo.Data.Name, pages); err != nil {
			fmt.Printf("⚠️  生成导航页失败: %v\n", err)
		} else {
			fmt.Printf("✅ 导航页: %s\n", filepath.Join(d.outputDir, "index.html"))
		}
	}

	return nil
}

// downloadPages 并发下载多个页面
func (d *Downloader) downloadPages(pages []PageItem, mapping *ProjectMapping) error {
	sem := make(chan struct{}, d.concurrency)
	var wg sync.WaitGroup
	var errCount int64
	var sucCount int64

	for i, page := range pages {
		wg.Add(1)
		go func(idx int, p PageItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := d.downloadPage(p, mapping); err != nil {
				atomic.AddInt64(&errCount, 1)
				fmt.Printf("  ❌ [%d/%d] %s: %v\n", idx+1, len(pages), p.Name, err)
			} else {
				atomic.AddInt64(&sucCount, 1)
				fmt.Printf("  ✅ [%d/%d] %s\n", idx+1, len(pages), p.Name)
			}
		}(i, page)
	}

	wg.Wait()
	fmt.Printf("\n📊 下载完成: 成功 %d, 失败 %d, 共 %d\n", sucCount, errCount, len(pages))
	if errCount > 0 {
		return fmt.Errorf("%d 个页面下载失败", errCount)
	}
	return nil
}

// downloadPage 下载单个页面的所有资源
func (d *Downloader) downloadPage(page PageItem, mapping *ProjectMapping) error {
	// 从 mapping 中获取页面信息
	pageEntry, ok := mapping.Pages[page.Filename]
	if !ok {
		return fmt.Errorf("页面 %s 不在 mapping 中", page.Filename)
	}

	// 如果有 mapping_md5，使用页面级配置下载
	if pageEntry.MappingMD5 != "" {
		return d.downloadPageByConfig(page, pageEntry.MappingMD5)
	}

	// 否则只下载 HTML
	if pageEntry.HTML.SignMD5 != "" {
		data, err := d.client.DownloadFile(pageEntry.HTML.SignMD5)
		if err != nil {
			return err
		}
		content := FixHTML(string(data))
		outPath := d.pageHTMLPath(page)
		os.MkdirAll(filepath.Dir(outPath), 0755)
		return os.WriteFile(outPath, []byte(content), 0644)
	}

	return fmt.Errorf("页面缺少下载信息")
}

// pageDir 返回页面所在目录: output/<folder>/<page_name>/
func (d *Downloader) pageDir(page PageItem) string {
	pageName := SanitizeFilename(page.Name)
	if page.Folder != "" && page.Folder != "根目录" {
		return filepath.Join(d.outputDir, SanitizeFilename(page.Folder), pageName)
	}
	return filepath.Join(d.outputDir, pageName)
}

// pageHTMLPath 返回页面 HTML 路径: output/<folder>/<page_name>/<filename>.html
func (d *Downloader) pageHTMLPath(page PageItem) string {
	return filepath.Join(d.pageDir(page), page.Filename)
}

// downloadPageByConfig 通过页面级配置下载所有资源
func (d *Downloader) downloadPageByConfig(page PageItem, mappingMD5 string) error {
	// 1. 获取页面配置
	config, err := d.client.GetPageConfig(mappingMD5)
	if err != nil {
		return err
	}

	// 2. 页面目录: <folder>/<page_name>/
	resDir := d.pageDir(page)
	if err := os.MkdirAll(resDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 3. 下载 HTML
	if config.HTML.SignMD5 != "" {
		data, err := d.client.DownloadFile(config.HTML.SignMD5)
		if err != nil {
			return fmt.Errorf("下载HTML失败: %w", err)
		}
		content := FixHTML(string(data))
		outPath := d.pageHTMLPath(page)
		os.MkdirAll(filepath.Dir(outPath), 0755)
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	// 4. 下载 data.js
	if config.DataJs.SignMD5 != "" {
		data, err := d.client.DownloadFile(config.DataJs.SignMD5)
		if err == nil {
			content := FixDataJs(string(data))
			outPath := filepath.Join(resDir, "data.js")
			os.WriteFile(outPath, []byte(content), 0644)
		}
	}

	// 5. 下载 CSS
	imgDir := filepath.Join(resDir, "images")
	for localPath, asset := range config.Styles {
		if asset.SignMD5 == "" {
			continue
		}
		data, err := d.client.DownloadFile(asset.SignMD5)
		if err != nil {
			continue
		}
		cssFileDir := filepath.Join(resDir, filepath.Dir(localPath))
		content, bgDownloads := FixCSS(string(data), cssFileDir, imgDir)
		outPath := filepath.Join(resDir, localPath)
		os.MkdirAll(filepath.Dir(outPath), 0755)
		os.WriteFile(outPath, []byte(content), 0644)

		// 下载 CSS 中引用的背景图片
		for remoteURL, absImgPath := range bgDownloads {
			d.downloadToPath(remoteURL, absImgPath)
		}
	}

	// 6. 下载 JS
	for localPath, asset := range config.Scripts {
		if asset.SignMD5 == "" {
			continue
		}
		data, err := d.client.DownloadFile(asset.SignMD5)
		if err != nil {
			continue
		}
		outPath := filepath.Join(resDir, localPath)
		os.MkdirAll(filepath.Dir(outPath), 0755)
		os.WriteFile(outPath, data, 0644)
	}

	// 7. 下载图片
	var imgWg sync.WaitGroup
	imgSem := make(chan struct{}, 5)
	for localPath, asset := range config.Images {
		if asset.SignMD5 == "" {
			continue
		}
		imgWg.Add(1)
		go func(lp, md5 string) {
			defer imgWg.Done()
			imgSem <- struct{}{}
			defer func() { <-imgSem }()
			d.downloadToPath(md5, filepath.Join(resDir, lp))
		}(localPath, asset.SignMD5)
	}
	imgWg.Wait()

	return nil
}

// downloadToPath 下载文件到指定路径（带去重）
func (d *Downloader) downloadToPath(signMD5, localPath string) {
	// 去重检查
	if _, loaded := d.downloaded.LoadOrStore(signMD5, localPath); loaded {
		return
	}

	if _, err := os.Stat(localPath); err == nil {
		return // 文件已存在
	}

	data, err := d.client.DownloadFile(signMD5)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(localPath), 0755)
	os.WriteFile(localPath, data, 0644)
}

// extractPages 从 mapping 中提取页面列表（保留层级结构）
func extractPages(mapping *ProjectMapping) []PageItem {
	var pages []PageItem
	if mapping.Sitemap != nil {
		for _, node := range mapping.Sitemap.RootNodes {
			collectPages(node, &pages, "", 0, "")
		}
	}
	// 如果 sitemap 为空，从 pages dict 构建
	if len(pages) == 0 {
		for filename := range mapping.Pages {
			pages = append(pages, PageItem{
				Name:     strings.TrimSuffix(filename, ".html"),
				Filename: filename,
			})
		}
		sort.Slice(pages, func(i, j int) bool {
			return pages[i].Filename < pages[j].Filename
		})
	}
	return pages
}

func collectPages(node SitemapNode, pages *[]PageItem, parentPath string, level int, folder string) {
	currentPath := node.PageName
	if parentPath != "" {
		currentPath = parentPath + "/" + node.PageName
	}

	if node.PageName != "" && node.URL != "" {
		*pages = append(*pages, PageItem{
			Name:     node.PageName,
			Filename: node.URL,
			Level:    level,
			Folder:   folder,
		})
	}

	// 递归子节点
	for _, child := range node.Children {
		nextFolder := folder
		if node.Type == "Folder" && node.URL == "" {
			nextFolder = node.PageName
		}
		collectPages(child, pages, currentPath, level+1, nextFolder)
	}
}

// filterByPageID 根据 pageId 过滤页面
func filterByPageID(mapping *ProjectMapping, pages []PageItem, pageID string) []PageItem {
	// 在 sitemap 中查找匹配的节点
	if mapping.Sitemap != nil {
		for _, node := range mapping.Sitemap.RootNodes {
			if found := findNodeByID(node, pageID); found != nil {
				// 找到了，收集该节点下的所有页面
				var filtered []PageItem
				collectPages(*found, &filtered, "", 0, "")
				if len(filtered) > 0 {
					return filtered
				}
				// 叶子节点，直接返回
				return []PageItem{{
					Name:     found.PageName,
					Filename: found.URL,
				}}
			}
		}
	}
	return pages
}

func findNodeByID(node SitemapNode, id string) *SitemapNode {
	if node.ID == id {
		return &node
	}
	for _, child := range node.Children {
		if found := findNodeByID(child, id); found != nil {
			return found
		}
	}
	return nil
}
