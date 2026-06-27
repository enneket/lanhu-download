package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var (
		cookie      = flag.String("cookie", "", "蓝湖Cookie (或设置环境变量 LANHU_COOKIE)")
		urlStr      = flag.String("url", "", "蓝湖文档URL")
		output      = flag.String("output", "", "输出目录 (默认: ./output/<文档名>)")
		concurrency = flag.Int("concurrency", 5, "并发下载数")
		singlePage  = flag.Bool("single", false, "只下载当前页面 (根据URL中的pageId)")
		toPNG       = flag.Bool("png", false, "同时截图生成PNG")
		pngWidth    = flag.Int("width", 1920, "PNG视口宽度")
		pngHeight   = flag.Int("height", 1080, "PNG视口高度")
		pngScale    = flag.Float64("scale", 2.0, "PNG缩放比例")
		pngTimeout  = flag.Int("timeout", 30, "PNG截图超时(秒)")
	)
	flag.Parse()

	// 支持位置参数: lanhu-download <url> [cookie]
	if flag.NArg() > 0 && *urlStr == "" {
		*urlStr = flag.Arg(0)
	}
	if flag.NArg() > 1 && *cookie == "" {
		*cookie = flag.Arg(1)
	}

	// Cookie 来源: flag > 环境变量
	if *cookie == "" {
		*cookie = os.Getenv("LANHU_COOKIE")
	}

	if *urlStr == "" {
		printUsage()
		os.Exit(1)
	}
	if *cookie == "" {
		fmt.Fprintln(os.Stderr, "❌ 缺少Cookie，请通过 -cookie 参数或 LANHU_COOKIE 环境变量提供")
		fmt.Fprintln(os.Stderr, "   获取方法: 浏览器登录蓝湖 → F12 → Network → 复制任意请求的 Cookie 头")
		os.Exit(1)
	}

	// 解析URL
	params, err := ParseURL(*urlStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ URL解析失败: %v\n", err)
		os.Exit(1)
	}

	// 如果指定了 single 但 URL 没有 pageId，报错
	if *singlePage && params.PageID == "" {
		fmt.Fprintln(os.Stderr, "❌ -single 模式需要URL中包含 pageId 参数")
		os.Exit(1)
	}

	// 默认输出目录
	if *output == "" {
		*output = "./output"
	}

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       蓝湖原型下载工具 v1.0         ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("📋 项目: %s\n", params.ProjectID)
	fmt.Printf("📄 文档: %s\n", params.DocID)
	if params.PageID != "" {
		fmt.Printf("📃 页面: %s (单页模式)\n", params.PageID)
	} else {
		fmt.Println("📃 模式: 全部页面")
	}
	fmt.Printf("📁 输出: %s\n", *output)
	fmt.Println()

	// 创建下载器
	client := NewLanhuClient(*cookie)
	dl := NewDownloader(client, *output, *concurrency)

	if err := dl.DownloadAll(params); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 下载失败: %v\n", err)
		os.Exit(1)
	}

	// PNG 截图
	if *toPNG {
		converter := NewPNGConverter(*pngWidth, *pngHeight, *pngScale, true, *pngTimeout)
		if err := ConvertAllToPNG(*output, converter); err != nil {
			fmt.Fprintf(os.Stderr, "\n⚠️  PNG转换失败: %v\n", err)
		}
	}

	fmt.Println("\n🎉 全部完成!")
	fmt.Printf("📁 文件保存在: %s\n", *output)

	// 提示如何查看
	if strings.Contains(*urlStr, "pageId") || !*singlePage {
		fmt.Printf("💡 用浏览器打开 %s/index.html 查看所有页面\n", *output)
	}
}

func printUsage() {
	fmt.Println(`蓝湖原型下载工具 - 将蓝湖原型文档下载为本地HTML

用法:
  lanhu-download -url <蓝湖URL> -cookie <Cookie>
  lanhu-download <蓝湖URL> [Cookie]
  lanhu-download -url <蓝湖URL> -single   # 只下载当前页面

参数:
  -url         蓝湖文档URL (必需)
  -cookie      蓝湖Cookie (或设置环境变量 LANHU_COOKIE)
  -output      输出目录 (默认: ./output)
  -concurrency 并发下载数 (默认: 5)
  -single      只下载URL中pageId对应的页面
  -png         同时截图生成PNG
  -width       PNG视口宽度 (默认: 1920)
  -height      PNG视口高度 (默认: 1080)
  -scale       PNG缩放比例 (默认: 2.0)
  -timeout     PNG截图超时秒数 (默认: 30)

环境变量:
  LANHU_COOKIE  蓝湖Cookie (可替代 -cookie 参数)

示例:
  export LANHU_COOKIE="your_cookie_here"
  lanhu-download -url "https://lanhuapp.com/web/#/item/project/product?tid=xxx&pid=xxx&docId=xxx"
  lanhu-download -url "蓝湖URL" -cookie "cookie" -png -scale 1

获取Cookie:
  1. 浏览器登录 https://lanhuapp.com
  2. F12 打开开发者工具 → Network 标签
  3. 刷新页面，点击任意请求
  4. 复制请求头中的 Cookie 值`)
}
