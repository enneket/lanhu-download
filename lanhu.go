package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL = "https://lanhuapp.com"
	CDNURL  = "https://axure-file.lanhuapp.com"
)

type URLParams struct {
	TeamID     string
	ProjectID  string
	DocID      string
	VersionID  string
	PageID     string
	SinglePage bool
}

type LanhuClient struct {
	httpClient *http.Client
	cookie     string
}

func NewLanhuClient(cookie string) *LanhuClient {
	return &LanhuClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		cookie:     cookie,
	}
}

func ParseURL(rawURL string) (*URLParams, error) {
	params := &URLParams{}

	// 从 fragment 中提取参数
	if strings.HasPrefix(rawURL, "http") {
		idx := strings.Index(rawURL, "#")
		if idx < 0 {
			return nil, fmt.Errorf("无效的蓝湖URL: 缺少fragment部分")
		}
		rawURL = rawURL[idx+1:]
		if i := strings.Index(rawURL, "?"); i >= 0 {
			rawURL = rawURL[i+1:]
		}
	}
	rawURL = strings.TrimPrefix(rawURL, "?")

	kv := make(map[string]string)
	for _, part := range strings.Split(rawURL, "&") {
		if i := strings.Index(part, "="); i >= 0 {
			kv[part[:i]] = part[i+1:]
		}
	}

	params.TeamID = kv["tid"]
	params.ProjectID = kv["pid"]
	params.DocID = kv["docId"]
	if params.DocID == "" {
		params.DocID = kv["image_id"]
	}
	params.VersionID = kv["versionId"]
	params.PageID = kv["pageId"]

	if params.TeamID == "" || params.ProjectID == "" {
		return nil, fmt.Errorf("URL缺少必要参数: tid=%s pid=%s", params.TeamID, params.ProjectID)
	}
	return params, nil
}

func (c *LanhuClient) newRequest(method, reqURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://lanhuapp.com/web/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("request-from", "web")
	req.Header.Set("real-path", "/item/project/product")
	return req, nil
}

func (c *LanhuClient) GetJSON(reqURL string, target interface{}) error {
	req, err := c.newRequest("GET", reqURL)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败 %s: %w", reqURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *LanhuClient) GetRaw(reqURL string) ([]byte, error) {
	req, err := c.newRequest("GET", reqURL)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败 %s: %w", reqURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// GetDocInfo 获取文档信息，返回 versions 列表
func (c *LanhuClient) GetDocInfo(projectID, docID string) (*DocInfoResp, error) {
	u := fmt.Sprintf("%s/api/project/image?pid=%s&image_id=%s", BaseURL, url.QueryEscape(projectID), url.QueryEscape(docID))

	req, err := c.newRequest("GET", u)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败 %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// API 响应可能用 data 或 result 包装
	var raw struct {
		Code   string          `json:"code"`
		Msg    string          `json:"msg"`
		Data   json.RawMessage `json:"data"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Code != "00000" && raw.Code != "0" && raw.Code != "" {
		return nil, fmt.Errorf("API错误: %s (code=%s)", raw.Msg, raw.Code)
	}

	// 优先用 data，其次用 result
	payload := raw.Data
	if len(payload) == 0 || string(payload) == "null" {
		payload = raw.Result
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("API返回空数据")
	}

	var info DocInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return nil, fmt.Errorf("解析文档信息失败: %w", err)
	}
	return &DocInfoResp{Code: raw.Code, Data: &info}, nil
}

// GetProjectMapping 获取项目级 mapping JSON (sitemap + pages)
func (c *LanhuClient) GetProjectMapping(jsonURL string) (*ProjectMapping, error) {
	var mapping ProjectMapping
	if err := c.GetJSON(jsonURL, &mapping); err != nil {
		return nil, fmt.Errorf("获取mapping失败: %w", err)
	}
	return &mapping, nil
}

// GetPageConfig 获取单个页面的资源配置
func (c *LanhuClient) GetPageConfig(mappingMD5 string) (*PageConfig, error) {
	u := CDNURL + "/" + mappingMD5
	var config PageConfig
	if err := c.GetJSON(u, &config); err != nil {
		return nil, fmt.Errorf("获取页面配置失败: %w", err)
	}
	return &config, nil
}

// DownloadFile 下载单个文件到内存
func (c *LanhuClient) DownloadFile(signMD5 string) ([]byte, error) {
	u := signMD5
	if !strings.HasPrefix(signMD5, "http") {
		u = CDNURL + "/" + signMD5
	}
	// CDN 下载不需要 cookie
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败 %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, u)
	}
	return io.ReadAll(resp.Body)
}

// --- 数据模型 ---

type DocInfoResp struct {
	Code string    `json:"code"`
	Msg  string    `json:"msg"`
	Data *DocInfo  `json:"data"`
}

type DocInfo struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	CreateTime string           `json:"create_time"`
	UpdateTime string           `json:"update_time"`
	Versions   []VersionInfo    `json:"versions"`
}

type VersionInfo struct {
	ID      string `json:"id"`
	JSONURL string `json:"json_url"`
	VersionInfo string `json:"version_info"`
}

type ProjectMapping struct {
	Pages   map[string]PageEntry `json:"pages"`
	Sitemap *Sitemap             `json:"sitemap"`
}

type PageEntry struct {
	HTML       AssetRef `json:"html"`
	MappingMD5 string   `json:"mapping_md5"`
}

type AssetRef struct {
	SignMD5 string `json:"sign_md5"`
	Src     string `json:"src"`
}

type Sitemap struct {
	RootNodes []SitemapNode `json:"rootNodes"`
}

type SitemapNode struct {
	PageName string        `json:"pageName"`
	URL      string        `json:"url"`
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Children []SitemapNode `json:"children"`
}

type PageConfig struct {
	HTML     AssetRef            `json:"html"`
	DataJs   AssetRef            `json:"dataJs"`
	Styles   map[string]AssetRef `json:"styles"`
	Scripts  map[string]AssetRef `json:"scripts"`
	Images   map[string]AssetRef `json:"images"`
}

// PageItem 表示一个可下载的页面
type PageItem struct {
	Name     string // 页面名称
	Filename string // HTML 文件名 (如 "abc123.html")
	Level    int    // 层级深度
	Folder   string // 所属文件夹
}
