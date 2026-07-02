// Command server runs the knowledge-base MCP service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"knowledge-base/internal/fs"
	"knowledge-base/internal/search"
	"knowledge-base/internal/server"
	"knowledge-base/internal/tag"
	"knowledge-base/internal/watch"
	"gopkg.in/yaml.v3"
)

func main() {
	var (
		rootDir = flag.String("dir", ".", "知识库根目录路径")
		port    = flag.Int("port", 8080, "HTTP 服务监听端口")
	)
	flag.Parse()

	// Resolve absolute root directory
	absDir, err := filepath.Abs(*rootDir)
	if err != nil {
		log.Fatalf("resolve root dir: %v", err)
	}

	// Ensure root directory exists
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		log.Fatalf("知识库目录 %q 不存在或不是目录", absDir)
	}

	log.Printf("知识库根目录: %s", absDir)

	// Load tag aliases
	tagMgr := loadTagAliases(absDir)
	log.Printf("标签管理器已初始化")

	// Create file store
	store := fs.NewStore(absDir)

	// Initial load
	if err := store.Reload(); err != nil {
		log.Fatalf("初始加载知识库失败: %v", err)
	}
	log.Printf("已加载 %d 个文档", len(store.AllDocs()))

	// Create search engine and build index
	engine := search.New(tagMgr)
	engine.BuildIndex(store.AllDocs())
	log.Printf("搜索引擎已就绪")

	// Setup hot reload
	store.SetReloadCallback(func() {
		engine.BuildIndex(store.AllDocs())
		log.Printf("热加载完成，当前 %d 个文档", len(store.AllDocs()))
	})

	watcher, err := watch.New(absDir, func() error {
		if err := store.Reload(); err != nil {
			return err
		}
		engine.BuildIndex(store.AllDocs())
		log.Printf("文件变化重载完成，当前 %d 个文档", len(store.AllDocs()))
		return nil
	})
	if err != nil {
		log.Fatalf("启动文件监听失败: %v", err)
	}
	defer watcher.Stop()
	log.Printf("文件热加载监听已启动")

	// Create and start HTTP server
	srv := server.New(store, engine, tagMgr)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("MCP 服务启动于 http://localhost%s", addr)
	log.Printf("  查询接口: POST http://localhost%s/query", addr)
	log.Printf("  草稿提交: POST http://localhost%s/draft", addr)
	log.Printf("  草稿列表: GET  http://localhost%s/drafts", addr)
	log.Printf("  草稿审核: POST http://localhost%s/drafts/approve?path=<filepath>", addr)
	log.Printf("  草稿废弃: POST http://localhost%s/drafts/reject?path=<filepath>", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("HTTP 服务错误: %v", err)
	}
}

// loadTagAliases reads .tag_aliases.yaml from the root directory.
func loadTagAliases(rootDir string) *tag.Manager {
	aliasesPath := filepath.Join(rootDir, ".tag_aliases.yaml")
	aliases := make(map[string][]string)

	data, err := os.ReadFile(aliasesPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &aliases); err != nil {
			log.Printf("警告: 解析标签别名文件 %q 失败: %v", aliasesPath, err)
		} else {
			log.Printf("已加载标签别名映射 (%d 个规范标签)", len(aliases))
		}
	} else {
		log.Printf("未找到标签别名文件 %q，使用默认配置", aliasesPath)
		// Provide sensible defaults
		aliases = map[string][]string{
			"go":       {"golang", "go语言"},
			"k8s":      {"kubernetes", "k8s", "kube"},
			"docker":   {"docker", "容器"},
			"deploy":   {"部署", "deployment", "deploy"},
			"trouble":  {"踩坑", "troubleshoot", "troubleshooting", "问题", "bug"},
			"standard": {"规范", "standard", "style", "编码规范"},
		}
	}

	return tag.NewManager(aliases)
}
