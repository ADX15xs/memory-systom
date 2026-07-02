# knowledge-base — 记忆系统 MCP 服务

基于文件系统的知识库检索 MCP 服务。知识以 YAML+Markdown 文件存储在独立的私有 Git 仓库 `../knowledge-repo/` 中（通过 `-dir` 指定），Go 服务提供 JSON-RPC 2.0 查询/草稿接口。

---

## Project

- **栈**: Go 1.26.3, 纯标准库 + `gopkg.in/yaml.v3` + `github.com/fsnotify/fsnotify`
- **入口**: `cmd/server/main.go`（`flag` 解析 `-dir` 和 `-port`）
- **领域**: 开发规范知识管理（MVP 仅 `dev/` 领域）

## Commands

| 命令 | 用途 |
|:-|:-|
| `go build -o knowledge-server ./cmd/server` | 构建二进制 |
| `go build ./...` | 检查编译 |
| `go test ./...` | 运行全部测试 |
| `go test ./internal/tag/ ./internal/search/ -v` | 运行指定包测试+详细输出 |
| `./knowledge-server.exe -dir ./knowledge -port 8080` | 启动服务（加载知识库占位目录） |
| `./knowledge-server.exe -dir ../knowledge-repo -port 8080` | 启动服务（加载独立私有知识库） |

## Architecture

```
cmd/server/main.go          ← 启动入口（flag, 初始化, 回调串联）
internal/
├── tag/                    ← 标签管理器（规范化, 别名映射, 四维验证）
├── fs/                     ← 文件系统读写（YAML 解析, 草稿 CRUD, 知识库重载）
├── search/                 ← 倒排索引检索引擎（BuildIndex, Search）
├── server/                 ← MCP JSON-RPC 2.0 HTTP 服务（/query, /draft, /drafts）
└── watch/                  ← fsnotify 热加载（递归监听 + 100ms 防抖）
../knowledge-repo/      ← 知识库独立私有仓库（由 -dir 指向）
├── .tag_aliases.yaml       ← 标签别名映射
├── dev/                    ← 活跃知识目录
├── drafts/                 ← 草稿目录
└── archive/                ← 历史归档（索引排除）
```

**数据流**: 文件变更 → watch 触发 → store.Reload() → engine.BuildIndex() → 查询走内存索引

## Conventions

- **错误处理**: 函数签名返回 `error`，不在库内部 `log.Fatal`/`panic`；主入口 `log.Fatalf` 仅用于不可恢复的启动错误
- **命名**: 包名小写单数（`fs`, `tag`, `search`），导出的构造函数 `NewXxx`，接口方法 `Xxx`（`Reload`, `Search`）
- **并发**: 所有读/写路径用 `sync.RWMutex` 保护，`Store.docs` 和 `Engine.index` 均互斥
- **测试**: 表格驱动测试（`[]struct{...}`）+ `t.Run` 子测试；`internal/tag` 和 `internal/search` 有测试
- **标签格式**: `维度:值`（如 `场景:开发`），四维至少 3 个；规范化小写+中划线
- **文档格式**: YAML 前页 `---\ntitle: ...\ntags: [...}\n---` + Markdown 正文
- **草稿流程**: `drafts/` → 人工审核 → `Post /drafts/approve` → 移入 `dev/` 对应目录

## Notes

- **知识库审计**: 所有修订记录在 `../knowledge-repo/` 中通过 Git 管理，使用 `git log`/`git diff`/`git blame` 审计

<!-- Quick-add space for future task-specific facts -->