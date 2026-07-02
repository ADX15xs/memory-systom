# knowledge-base

基于文件系统的团队知识库 MCP 服务。知识以 YAML+Markdown 文件存储，Go 服务提供 JSON-RPC 2.0 查询与草稿接口。

## 快速开始

```bash
# 启动服务
go build -o knowledge-server ./cmd/server
./knowledge-server -dir ./knowledge -port 8080

# 查询知识
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"query_knowledge","params":{"query":"go 服务"},"id":1}'

# 提交草稿
curl -X POST http://localhost:8080/draft \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"submit_knowledge","params":{"title":"...","content":"...","tags":["场景:开发","域:Go"]},"id":1}'
```

## 目录结构

```
knowledge/      ← 知识库根目录
├── dev/        ← 开发领域
│   ├── common/ ← 通用规范
│   └── client-xx-bank/  ← 甲方定制
├── drafts/     ← 待审核草稿
└── archive/    ← 历史归档
```

## 命令

| 命令 | 用途 |
|:--|:--|
| `go build ./...` | 编译检查 |
| `go test ./...` | 运行全部测试 |
| `go test -v ./internal/tag/ ./internal/search/` | 指定包测试 |

## 详细文档

见 [docs/architecture.md](docs/architecture.md) 与 [AGENTS.md](AGENTS.md)。
