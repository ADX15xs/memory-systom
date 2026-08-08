---
title: 状态核对与主流方向对照
description: 核对当前代码真实状态（纠正决策文档第 11 节的过时结论），并对照 2026 年行业主流验证后续方向
date: 2026-08-09
status: 结论——两个 🔴 缺陷已修复；方向符合主流且偏超前；当前紧要项为补 fs 层测试与召回上限
---

## 一、背景

用户问两件事：(1) 当前最紧要的开发任务是什么？(2) 后续推进方向是否符合主流？
核对依据：`docs/` 决策文档、`AGENTS.md`、对 `internal/` 实际代码与 git 历史的核查，外加 WebSearch 行业资料（2026-08 检索）。

## 二、关键纠正：决策文档第十一节已过时

`docs/decisions/2026-08-08-kb-pilot-evaluation.md` 第十一节称两个 🔴 严重缺陷"经核查在当前代码中均未修复"，并据此主张"应排在任何架构演进之前……本末倒置"。

**该结论已过时。** 提交 `00083d1`（`fix: 修复草稿审核路径遍历漏洞与查询 score 丢失`，2026-08-09）已修复两项：

- **路径遍历**：新增 `Server.resolveDraftPath`，用 `filepath.Abs` + `filepath.Rel` 把目标锁死在 `Store.DraftsDir()` 内，越界一律拒回 400。
- **score 恒为 0**：删除从未被赋值的 `fs.Document.Score` 死字段，API 改读 `search.Result.Score`（命中次数），对外转成正分。
- 新增 `internal/server/server_test.go`，含真实利用尝试（绝对路径 / `../` 遍历删 victim.txt 被拦）的回归测试。
- `go build ./...` / `go test ./...` / `go vet ./...` 全绿。

故该节所述"任意文件删除漏洞"已不存在，"本末倒置"的优先级判断不再成立。

## 三、当前代码状态核查

| 缺陷（来源） | 原评级 | 当前状态 | 证据 |
| :-- | :-- | :-- | :-- |
| 路径遍历（mvp-eval 🔴#1 / kb-pilot 11.1） | 🔴 严重 | ✅ 已修复 | `server.go` `resolveDraftPath` + `00083d1` |
| score 恒为 0（mvp-eval 🔴#2 / kb-pilot 11.2） | 🔴 严重 | ✅ 已修复 | `server.go` `resultsToMap` 读 `r.Score` |
| 双重 BuildIndex（mvp-eval 🟡#3） | 🟡 | ⚠️ 仍在 | `main.go:59-71` |
| 内容仅索引前 100 字（mvp-eval 🟡#4） | 🟡 | ⚠️ 仍在 | `search.go:68-72` |
| 模糊匹配 O(n) 全扫描（mvp-eval 🟡#5） | 🟡 | ⚠️ 仍在（MVP 可接受） | `search.go:134-140` |
| 标签维度前缀过宽（mvp-eval 🟡#6） | 🟡 | ⚠️ 仍在（边缘 case） | `tag.go:110-117` |
| `fs` 包无测试 | 测试缺口 | ⚠️ 仍在 | `internal/fs/` 仅 `fs.go` |
| `server` 包无测试 | 测试缺口 | ✅ 已补 | `server_test.go` 已存在 |

## 四、当前真正紧要的任务（按优先级）

| 优先级 | 任务 | 说明 |
| :-- | :-- | :-- |
| **P0** | 补 `internal/fs` 回归测试 | 安全修复只覆盖 HTTP 层；`ParseFile`/`WriteDraft`/`ApproveDraft`/`RejectDraft` 文件移动核心逻辑仍 0 覆盖，同类 bug 易复发 |
| **P1** | 修 🟡#4：内容索引扩出前 100 字 | 直接影响召回率（核心价值），长文档超界部分永不被检索 |
| **P1** | 修 🟡#3：去掉双重 BuildIndex | `main.go` 每次变更建两次索引，纯浪费，改动极小 |
| **P2** | 修 🟡#6：标签维度精确比较 | `parts[0] == DimScene` 替代过宽前缀匹配（边缘 case） |
| **P1** | 补文档治理债 | `architecture.md` 加"设计参考与取舍"一节；`design-principles.md` 未被其/README 引用（kb-pilot 评估第 10 节已建议） |

> 注意：原"两个 🔴 严重缺陷"已从阻塞列表移除——它们已修复。不要再将其视为当前紧要项。

## 五、后续方向是否符合主流（WebSearch 对照）

**结论：符合主流，且略有前瞻。**

1. **"文件系统即数据库"是 2026 年 agent memory 的默认范式。** 收敛时间线（dev.to 综述）：Anthropic Memory 工具（写 `/memories` 文件系统，2025-08）→ Anthropic Context Engineering（警告 "context rot"）→ Linux Foundation Agentic AI Foundation（AGENTS.md，6 万+ 项目采用）→ **Karpathy LLM Wiki gist（2026-04，三层 Markdown wiki，明确对照 naive RAG）**。本项目"文件系统即数据库 + 四维结构化 tag + 人在回路"与之高度吻合，且独立收敛于 LLM Wiki gist 之前。
2. **"减法 / 反对过度工程"被反复验证。** 主流共识：先文件化跑通，只有当语料超出文件承载能力时再加 RAG/向量，而非先建向量库。MVP 砍掉向量/图谱/重排、先做最小闭环，正踩在点上。
3. **拒绝 kb-pilot、保留 PageIndex 作备选——与 2026 "vectorless RAG" 浪潮一致。** PageIndex（VectifyAI）已约 3 万 star，是"无向量 + 树检索"权威实现；但其适用域是**长文档**章节树导航。本项目知识单元是人工撰写的原子 YAML 条目，切块问题从设计上已绕开，故 kb-pilot 解决的是不存在的问题——该判断稳健。
4. **平衡点（用于 P1 之后）。** 所有来源一致认为：向量 RAG 在"大规模非结构化语料 / 模糊语义检索"仍最优。当前规模（dev 知识、数百条）正落在文件化甜区。但 **P1 的 MinerU 批量导入会推高体量**，届时已规划的"为每条知识加 LLM 生成的 `summary` 字段纳入索引"（kb-pilot 评估吸收的第 2 点）恰是"outgrow 文件后加一层"的正确过渡，而非推翻现有架构。

## 六、下一步建议

- 立即做 P0（`fs` 测试）+ P1（#4 召回上限、#3 双重索引）。
- 把"设计参考与取舍"补进 `architecture.md`（治理债）。
- P1 批量导入落地后，按规划的 summary 路由层平滑扩展，不引入向量库前置依赖。

## 七、参考来源

- Anthropic Memory tool / Context Engineering / Linux Foundation Agentic AI Foundation（AGENTS.md）— 2025-2026 收敛综述（dev.to, "Considering RAG for your Agent? Build this instead"）。
- Karpathy LLM Wiki gist（2026-04）。
- PageIndex（VectifyAI）"vectorless / reasoning-based RAG" — GitHub ~3 万 star，FinanceBench 98.7%。
- 多来源一致结论：file-based memory 是 agent state 默认，向量 RAG 仅在大规模非结构化语料场景占优。
- 本仓库代码与 git 历史（`00083d1` 等）。
