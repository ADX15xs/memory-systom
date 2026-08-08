# docs/ 文档地图

本目录按「核心文档 / 决策记录」两类组织，避免文档平铺带来的阅读负担。

## 阅读顺序建议

1. **`architecture.md`** — 系统怎么工作（技术架构、模块、数据流、约定）。新接触项目先读这份。
2. **`design-principles.md`** — 为什么这么设计（减法哲学、MVP 规划、设计参考来源 / prior art）。理解"为何这么做"再读它。

## 目录说明

### 核心文档（本项目产出，必读）
| 文件 | 定位 |
|:--|:--|
| `architecture.md` | 技术架构：分层、模块职责、`internal/` 包关系、数据流、约定 |
| `design-principles.md` | 设计哲学（减法）、MVP 架构与规划、参考来源（LLM-Wiki / MemSkill 等） |

### `decisions/` — 决策记录（按日期，可长期归档）
记录"当时为什么这么定"，含已发现并修复 / 待修复的关键问题。

| 文件 | 内容 |
|:--|:--|
| `decisions/2026-06-28-mvp-evaluation.md` | MVP 自评报告（首次提交后曾被误删，已恢复）。标注两个 🔴 严重缺陷 |
| `decisions/2026-08-08-kb-pilot-evaluation.md` | 对 kb-pilot 的批判性选型评估。结论：**不采纳为下一形态**，选择性吸收 3 个设计点。文末附来源文章核心论点 |

> 外部原始文章《放弃 RAG，让 AI 像人一样读文档》不纳入本仓库；其核心论点与本项目的批判性评估已整合进 `decisions/2026-08-08-kb-pilot-evaluation.md` 文末，原文见 https://github.com/waylondev/kb-pilot 。

## 备注
- 代码约定、构建与测试命令见仓库根 `AGENTS.md`，不在本文档重复。
