# 记忆系统 MVP 评估报告

> 评估日期：2026-06-28
> 评估范围：`cmd/server`、`internal/{tag,fs,search,server,watch}`、`knowledge/`
> 验证方式：`go build ./...` ✅ 通过 · `go test ./...` ✅ 全部通过

> **补正（2026-08-09）**：本报告标注的两个 🔴 严重缺陷——"草稿审核接口任意文件删除漏洞"与"API 返回的 score 恒为 0"——**已于 2026-08-09 提交 `00083d1` 修复**，并新增 `internal/server/server_test.go` 回归测试；`server` 包现已非"零覆盖"。当前代码已无此二项缺陷，本报告作为 2026-06-28 的历史快照保留。最新状态与剩余缺口见 `docs/decisions/2026-08-09-status-check.md`。

---

## 总览

| 维度 | 状态 | 说明 |
|:-|:-|:-|
| 编译 | ✅ 通过 | `go build ./...` 无错误 |
| 测试 | ✅ 通过 | `tag` + `search` 包测试全部通过 |
| 架构 | ✅ 良好 | 6 包清晰分层，职责单一 |
| 测试覆盖 | ⚠️ 不足 | 6 包中仅 2 包有测试 (`fs`/`server`/`watch`/`cmd` 零覆盖) |
| 安全性 | 🔴 严重缺陷 | 草稿审核接口存在任意文件删除漏洞 |

---

## 🔴 严重问题（必须修复）

### 1. 任意文件删除/读取漏洞 — 路径遍历

**位置**：`internal/server/server.go:260`、`internal/server/server.go:289`

```go
filePath := r.URL.Query().Get("path")  // 用户可控，无任何校验
```

该 `filePath` 直接传入 `fs.ParseFile` → `os.ReadFile` → `os.Remove`（`internal/fs/fs.go:326`、`internal/fs/fs.go:335`）。

**风险**：攻击者可构造 `POST /drafts/reject?path=../../../etc/important_file` 来**删除服务器上任意文件**，或通过 `/drafts/approve` 读取并复制任意文件到知识库目录。

**修复方向**：校验 `filePath` 必须在 `drafts/` 目录内（`filepath.Rel` + `filepath.Clean` 检查），拒绝绝对路径和 `..` 遍历。

---

### 2. API 返回的 score 永远为 0

**位置**：`internal/server/server.go:162`

```go
"score": r.Document.Score,  // 读 Document.Score (float64)
```

搜索引擎将分数写入 `Result.Score`（`internal/search/search.go:147`，`int` 类型），**从未设置 `Document.Score`**。API 响应中所有结果的 `score` 字段恒为 `0`，排序信息完全丢失。

**修复**：改为 `"score": r.Score`。

---

## 🟡 设计缺陷（建议改进）

### 3. 热加载触发双重索引重建

**位置**：`cmd/server/main.go:59-62`、`cmd/server/main.go:64-71`

`main.go` 注册了 `store.SetReloadCallback` 回调，回调内调用 `engine.BuildIndex`。同时 watcher 回调也调用 `store.Reload()` **然后再次** `engine.BuildIndex`。

而 `store.Reload()`（`internal/fs/fs.go:230-232`）末尾会调用 `s.onReload()`——即注册的回调。所以每次文件变更会触发 **两次** `BuildIndex`，浪费资源。

**修复**：去掉 watcher 回调中冗余的 `engine.BuildIndex` 调用，仅依赖 `onReload` 回调。

---

### 4. 内容索引仅覆盖前 100 字符

**位置**：`internal/search/search.go:68-72`

```go
if len([]rune(contentPreview)) > 100 {
    contentPreview = string(contentRunes[:100])
}
```

长文档超出 100 字符的部分**永不被索引**，严重影响召回率。知识文档通常较长，这是显著的功能限制。

---

### 5. 模糊匹配 O(n) 全索引扫描

**位置**：`internal/search/search.go:134-140`

当精确匹配失败时，对每个查询词遍历**全部索引键**做 `strings.Contains`。索引量大时性能会显著下降。MVP 阶段可接受，但应在路线图中标记为优化项。

---

### 6. 标签维度前缀匹配过于宽松

**位置**：`internal/tag/tag.go:110-117`

```go
strings.HasPrefix(parts[0]+":", DimScene+":")
```

这等价于 `HasPrefix(parts[0], DimScene)`，意味着 `场景XYZ:值` 也会被计入 `场景` 维度。应改为精确比较 `parts[0] == DimScene`。

---

## 🟢 做得好的部分

1. **架构分层清晰**：`tag` / `fs` / `search` / `server` / `watch` 职责单一，依赖方向正确，`main.go` 作为组合根串联所有组件。
2. **并发安全**：`Store.docs` 和 `Engine.index` 均用 `sync.RWMutex` 保护，读写路径正确加锁。
3. **中英文分词**：`tokenize`（`search.go:210-254`）正确处理中文标点分割 + 英文 CamelCase 拆分，覆盖核心使用场景。
4. **标签别名系统**：`Manager.Resolve`（`tag.go:46-54`）支持双向映射（`golang` → `go`），测试覆盖良好。
5. **热加载防抖**：`watch.go:64-110` 使用 100ms 防抖避免频繁重载，设计合理。
6. **草稿审核流程**：`drafts/` → 人工审核 → `dev/` 入库的流程完整，`ApproveDraft` 支持按 `域:` 标签自动归类。
7. **降级搜索策略**：结果不足 3 条时自动移除停用词重试（`search.go:157-197`），提升召回。

---

## 测试覆盖差距

| 包 | 测试状态 | 风险 |
|:-|:-|:-|
| `internal/tag` | ✅ 有测试 | 覆盖 Normalize/Resolve/Validate |
| `internal/search` | ✅ 有测试 | 覆盖 tokenize/BuildIndex/Search |
| `internal/fs` | ❌ 无测试 | ParseFile/WriteDraft/ApproveDraft 无覆盖 |
| `internal/server` | ❌ 无测试 | JSON-RPC 处理/错误路径无覆盖 |
| `internal/watch` | ❌ 无测试 | 防抖逻辑无覆盖 |
| `cmd/server` | ❌ 无测试 | 启动流程无覆盖 |

**建议优先补充**：`fs` 包的 `ParseFile`（YAML 解析边界情况）和 `ApproveDraft`（文件移动逻辑）测试，以及 `server` 包的 HTTP 端到端测试（尤其是上面发现的安全漏洞需要回归测试）。

---

## 结论

这是一个**架构设计良好的 MVP**，分层清晰、核心功能（标签管理/倒排索引/热加载/草稿流程）完整可用。但在**安全性**（路径遍历漏洞）和**数据正确性**（score 恒为 0）上存在必须修复的阻塞问题。建议优先处理上述 🔴 两项严重问题后，再逐步补充测试覆盖和优化搜索召回率。
