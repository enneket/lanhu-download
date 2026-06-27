# lanhu-download

蓝湖原型下载工具 — 将蓝湖 Axure 原型下载为本地 HTML，支持 PNG 截图。

## 安装

```bash
go install github.com/enneket/lanhu-download@latest
```

或从 [Releases](https://github.com/enneket/lanhu-download/releases) 下载预编译二进制。

## 获取 Cookie

1. 浏览器登录 [蓝湖](https://lanhuapp.com)
2. `F12` → Network → 刷新页面
3. 点击任意请求，复制请求头中的 `Cookie` 值

## 配置

复制 `.env.example` 为 `.env`，填入 Cookie：

```bash
cp .env.example .env
```

```
LANHU_COOKIE=your_cookie_here
```

支持的环境变量：

| 变量 | 说明 |
|------|------|
| `LANHU_COOKIE` | 蓝湖 Cookie（必填） |

## 用法

```bash
# 环境变量方式
export LANHU_COOKIE="your_cookie_here"
lanhu-download -url "蓝湖URL"

# 参数方式
lanhu-download -url "蓝湖URL" -cookie "your_cookie"

# 只下载当前页面
lanhu-download -url "蓝湖URL" -cookie "cookie" -single

# 下载 + PNG 截图
lanhu-download -url "蓝湖URL" -cookie "cookie" -png
```

## 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-url` | | 蓝湖文档 URL（必需） |
| `-cookie` | | 蓝湖 Cookie（或 `LANHU_COOKIE` 环境变量） |
| `-output` | `./output` | 输出目录 |
| `-concurrency` | `5` | 并发下载数 |
| `-single` | `false` | 只下载 URL 中 pageId 对应的页面 |
| `-png` | `false` | 同时截图生成 PNG |
| `-width` | `1920` | PNG 视口宽度 |
| `-height` | `1080` | PNG 视口高度 |
| `-scale` | `2.0` | PNG 缩放比例 |
| `-timeout` | `30` | PNG 截图超时（秒） |

## 输出结构

目录层级与蓝湖左侧导航一致：

```
output/
├── index.html          # 导航页（多页时生成）
├── html/
│   ├── 后台/
│   │   ├── APP商城配置/
│   │   │   ├── app商城配置.html
│   │   │   └── 页面管理/
│   │   │       └── 页面管理.html
│   │   └── 商品种类/
│   └── APP/
│       └── 我的/
│           ├── 我的.html
│           └── 优惠券/
└── png/                # -png 时生成
    ├── 后台/
    └── APP/
```

## 依赖

- Go 1.26+
- Chrome/Chromium（PNG 截图需要）
