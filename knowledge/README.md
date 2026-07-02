# `knowledge/` — 知识库占位目录

> ⚠️ **知识库已迁出本项目，转移到独立私有 Git 仓库管理。**
>
> 本目录不再包含实际知识内容，仅为兼容 `-dir ./knowledge` 默认启动保留占位。
> 实际知识库托管在项目同级 `../knowledge-repo/`。

## 快速开始

```bash
# 启动服务时指向独立知识库
cd /d/github-clone/memory-system
./knowledge-server.exe -dir ../knowledge-repo -port 8080
```

## 审计与修订历史

知识库使用独立 Git 仓库进行版本管理：

```bash
cd /d/github-clone/knowledge-repo

# 查看变更历史
git log --oneline

# 查看某次变更详情
git show <commit-hash>

# 查看某个文件的修改记录
git log --follow -- <file>

# 查看某行最后的修改者
git blame <file>
```

## 目录结构（独立仓库内）

```
knowledge-repo/
├── .tag_aliases.yaml    ← 标签别名
├── dev/                 ← 活跃知识（开发领域）
│   ├── common/          ← 通用规范
│   ├── client-xx-bank/  ← 按客户端/项目归类的知识
│   └── ...              ← 其他领域子目录
├── drafts/              ← 待审核草稿（服务自动创建）
└── archive/             ← 历史归档（不在索引中加载）
```

## 隐私说明

- 这个独立的 knowledge-repo **永远不要 push 到公开仓库**
- 它和 memory-system 项目的公开 git 历史完全隔离
- 所有客户名、内部 IP、账号信息仍按原规范禁止出现在知识条目中
