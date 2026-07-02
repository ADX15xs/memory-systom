# 远程仓库清理方案 — 公开仓库 `ADX15xs/memory-systom`

> ⚠️ **本文件说明已 push 到远程的内容如何彻底清除。**
> 本地已用 `.gitignore` + `git rm --cached` 保护,但**远程 HEAD `58b55cf` 里仍有 5 个文件的全部内容**,通过 `git clone` 即可还原。

---

## 当前远程状态(已核实 2026-07-02)

| 路径 | 状态 | 敏感度 |
|:-|:-|:-|
| `knowledge/.tag_aliases.yaml` | 已公开 | 中(标签别名,可能暴露分类习惯) |
| `knowledge/dev/common/go-coding-standards.yaml` | 已公开 | 低(通用 Go 规范) |
| `knowledge/dev/common/microservice-arch.yaml` | 已公开 | 低(通用微服务) |
| `knowledge/dev/client-xx-bank/k8s-deploy-troubleshoot.yaml` | 已公开 | **高(标签含 `甲方:XX银行`)** |
| `knowledge/drafts/test-draft.yaml` | 已公开 | 低(测试用) |

仓库是 **Public**,任何人都可 clone/fork/缓存。

---

## 方案 A:删除并重建仓库(最干净,推荐)

适合:**无其他协作者,仓库尚未发布给别人用**。

### A1. 在 GitHub 网页上操作

1. 进入 `https://github.com/ADX15xs/memory-systom`
2. **Settings → Danger Zone → "Delete this repository"**
3. 按提示输入仓库名确认
4. 在本地重建:

```bash
cd /d/github-clone/memory-system
# 1) 备份当前工作区(以防万一)
tar -czf ~/memory-systom-backup.tar.gz --exclude=node_modules --exclude=.git .

# 2) 切掉旧 remote,创建新空仓库
git remote remove origin
# 在 GitHub 网页上 New repository,命名仍为 memory-systom
git remote add origin git@github.com:ADX15xs/memory-systom.git

# 3) 提交当前已 stage 的清理变更
git commit -m "chore: 整体忽略 knowledge/(本地私有,严禁 push)

- .gitignore 增加 'knowledge/**' + 保留根目录 .gitkeep / README.md
- git rm --cached 5 个已追踪文件
- 公开仓库不再包含任何内部知识条目"

# 4) 推送
git push -u origin main
```

### A2. 注意

- GitHub 删库后,**仓库名会立即释放**,同名重建不会有冲突
- 任何已 fork 的副本仍包含旧内容 — 需联系 fork 者删除

---

## 方案 B:重写历史 + force push(适合有协作者或不想丢 fork 链接)

```bash
cd /d/github-clone/memory-system

# 1) 备份
cp -r .git .git.backup.$(date +%s)

# 2) 从 HEAD 之前的 commit 重建分支,移除 knowledge/ 下所有内容
#    方式 1:用 git filter-repo(推荐,需 pip install)
pip install git-filter-repo
git filter-repo --path knowledge/ --invert-paths

#    方式 2:用 BFG Repo-Cleaner(更快,适合大仓库)
#    bfg --delete-folders knowledge

# 3) 强制推送到远程
git remote add origin git@github.com:ADX15xs/memory-systom.git
git push origin main --force
```

### B2. 注意

- **会改变所有 commit hash** — 任何本地副本必须重新 clone
- **已有 fork / clone 仍包含旧历史**(无法强制别人删除)
- 旧 commit 在 GitHub Events 里仍可见 hash,可通过 `git push origin :58b55cf` 删除远程 ref 缓解

---

## 方案 C:不动远程,只把本地保护做好(最小行动)

如果只是**避免未来再泄露**,**而不在意已经泄露的内容**:

```bash
# 仅 commit 当前 stage 的清理
cd /d/github-clone/memory-system
git add .gitignore knowledge/.gitkeep knowledge/README.md docs/mvp-evaluation.md
git commit -m "chore: 整体忽略 knowledge/ 本地私有"
git push origin main
```

之后所有 `git push` 都不会再携带 `knowledge/` 下任何内容。但**远程 58b55cf commit 仍可被 clone**。

---

## 推荐决策表

| 你的处境 | 推荐方案 |
|:-|:-|
| 仓库只是 demo,从未告诉别人地址 | **A**(删库重建) |
| 仓库已有协作者,需要保留 commit 历史 | **B**(filter-repo + force push) |
| 仓库地址已经在简历/博客/群里发过 | **A + 改名**(`memory-systom-public`) |
| 公开内容可接受,只想以后别再传 | **C** |
| 想完全清干净,即使已 fork | A + 联系 fork 者 + 联系 GitHub 支持 |

---

## 验证清理是否成功(任意方案执行后)

```bash
# 在临时目录全新 clone
git clone https://github.com/ADX15xs/memory-systom.git /tmp/verify
cd /tmp/verify
ls -la knowledge/
# 应该只看到 .gitkeep 和 README.md,没有 .yaml / .md / .docx / .xls
```

---

## 进一步加固建议

1. **本地预检钩子**:`.git/hooks/pre-commit` 里加一段拒绝包含敏感字串(客户名、IP、`X:/====`)的提交
2. **远程保护**:GitHub → Settings → Code security → 启用 secret scanning
3. **新功能前**:所有写入 `knowledge/` 的接口已审计路径遍历漏洞,见 `docs/mvp-evaluation.md`
