# HarnessLab — Community Launch Materials

> 传播叙事（路线图 §11）：不是 "I built another agent framework"，
> 而是 **"Same model. Same task. Same repository. Different harness."**

---

## 核心一句话

> HarnessLab is an open-source platform that records a coding-agent run,
> replays it offline, compares trajectories, and benchmarks different harness
> strategies — so you can **prove** why one harness beats another.

## 技术故事（Technical Story，供长文/演示引用）

**问题**：AI coding agent 的"harness"（prompt、规划、上下文、工具、验证、重试、
预算）大多是写死的。换一个提示词、加一个验证命令，效果好坏全凭感觉，无法测量。

**方案**：HarnessLab 把 harness 变成声明式配置（`harness.yaml`），并围绕它提供完整工程闭环：

1. **Trace** — 每次运行记录 JSONL 轨迹（模型调用/工具调用/结果）
2. **Offline Replay** — 不调模型 API，用录制的 replay store 复现整条轨迹（实测 16ms vs 实跑 ~4s）
3. **Diff** — 同任务、不同 harness 的轨迹对比 + 首个分歧点
4. **Benchmark** — 34 个真实任务 × 8 类 × dev/holdout 分离 × `--repeat N` 统计报告（mean/P50/P90/CI95）
5. **Ablation** — H0 基线 → planner → verification → retry → context → skills → full，逐个组件归因
6. **Optimizer** — 失败分析 → LLM 生成候选 harness → dev 评估 → Pareto 选择 → **holdout REJECT 门**

**证据**（真实运行）：
- 40 次公开基准：39/40 PASS，总成本 USD 0.38
- 离线重放：全轨迹从 store 回放，验证 PASS，workspace 变更重新落地
- LLM Optimizer：候选 harness 比基线省 ~50% token（26k vs 54k），holdout 验证后推荐
- 双运行时：同一管线跑 tRPC-Agent-Go 与 Codex CLI（runtime-agnostic）

## 各平台文案

### Hacker News（英文长评风格标题）

> **Show HN: HarnessLab — benchmark AI coding-agent "harnesses", not agents**

I kept hitting the same wall with coding agents: change one prompt or add a
verification command, and I couldn't tell whether it actually helped. So I
built HarnessLab: the harness (prompt, planning, context, tools, verification,
retry, budget) becomes a declarative `harness.yaml`, and every run is traced,
replayable offline (16ms, no API calls), diffable, and benchmarkable across
34 real tasks with dev/holdout splits and confidence intervals. It even runs a
closed loop: failure analysis → LLM generates candidate harnesses → dev eval →
Pareto → holdout gate (a candidate that wins on dev but regresses on holdout is
rejected). Runtime-agnostic: same pipeline drives tRPC-Agent-Go and the Codex
CLI. Apache-2.0, binaries on GitHub Releases.

### Reddit /r/MachineLearning 或 /r/LocalLLaMA（贴文）

Title: `[P] HarnessLab — measure your coding-agent harness, not just the model`

Body: Most of us tune prompts/tools/verification by gut feel. HarnessLab makes
harness changes measurable: trace → offline replay → trajectory diff →
benchmark (34 tasks, dev/holdout, repeated runs, CI on success rate/tokens/cost)
→ LLM optimizer with a holdout reject gate. Real numbers from a 40-run public
benchmark: 39/40 pass at $0.38 total, offline replay in 16ms with zero external
calls. Open source, Go, Apache-2.0.

### X / Twitter（3-4 条线程）

1. Same model. Same task. Same repository. Different harness. → measurable.
   HarnessLab turns a coding agent's harness into declarative YAML + full
   engineering loop: trace, offline replay, diff, benchmark.
2. Offline replay: your agent run, replayed in 16ms, 0 external model/tool
   calls, verification passing. No API key needed. `harness replay <run-id>`
3. Benchmark: 34 real tasks, dev/holdout, --repeat N, mean/P50/P90/CI95 per
   harness variant. Ablation H0→H6 attributes each component's value.
4. LLM optimizer: failure analysis → LLM candidates → dev eval → Pareto →
   holdout gate. Dev-only wins get REJECTED. Open source: github.com/ittakestwo123/Harnesslab

### 知乎（中文长文）

标题：《把 coding agent 的 harness 变成可测量、可复现、可优化 —— HarnessLab 开源》

要点：模型一样、任务一样、仓库一样，只换 harness 配置，效果差异就能被量化。
文章按 问题 → 设计（trace/replay/diff/benchmark/optimizer）→ 证据（40 跑 $0.38、
16ms 离线重放、LLM optimizer 候选省 50% token、双运行时）→ 快速上手 展开。

### 掘金（中文技术文）

标题：《Go 实现的 Agent Harness 工程平台：trace / 离线重放 / 轨迹对比 / 基准 / LLM 自动优化》

偏工程实现：HarnessSpec 声明式配置、Runtime Interface + TRPC/Codex 双适配器、
canonicalizer 路径归一化（$WORKSPACE）、replay store 哈希一致性、bench 调度器与
统计报告、LLM optimizer 的 dev/holdout REJECT 门。附 CLI 速查。

---

## 仓库建议

- **Description**：`Build, trace, replay, benchmark and evolve AI agent harnesses.`
- **Topics**：`ai-agent, agent, agentic-ai, harness, coding-agent, golang, llm, benchmark, observability, mcp`

## 素材清单

| 素材 | 位置 | 状态 |
|---|---|---|
| Demo GIF（12s，真实输出） | `docs/demo.gif` | ✅ 已生成（1040×640, 94 帧, 11.76s） |
| Demo 静态预览 | `docs/demo-preview.png` | ✅ 已生成 |
| README 首屏（含 GIF + 安装） | `README.md` | ✅ 已更新 |
| 本发布文案 | `docs/announcement.md` | ✅ |
