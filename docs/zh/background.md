# 为什么需要 Open Agent Workflow

[English](../en/background.md) | [README 中文](../../README-zh.md)

Open Agent Workflow（OAW）是 provider-neutral 的治理层，面向同时使用多个工程
workflow 和多个 coding-agent 客户端的开发者。OAW Core 决定使用什么契约以及每项职责
由谁负责；可选 Workflow Coordinator 记录持久化 Workflow State；Agent Host 决定怎样在
当前会话或原生 Subagent 中执行。这些是相互分离的权限边界。

## 一个任务，多个自动触发器

一个 workflow family 可能自带 discovery、规划、实现、TDD、调试、复核和完成流程。
Superpowers、Matt Pocock skills 与 Everything Claude Code（ECC）都覆盖其中多项职责。
当它们同时安装时，重叠的自动触发可能为同一交付物启动多个生命周期。

冲突的核心是所有权，而不是某个 provider 是否有用。没有仲裁门禁时，一个 family 可能
编写规格，另一个 family 同时创建不同的计划；两套 TDD 流程可能选择不同 seam；一个
review 工具也可能在另一 family 已拥有 completion 后再次开启 remediation loop。后续
请求还可能重新触发选择，在任务中途静默更换方法。

OAW 不会仲裁普通 Host 行为。正常请求，包括 Host 自动选择 Skill 或用户直接调用
一个普通 Skill，都继续使用原生 Host。只有显式激活才会建立 task-scoped OAW
Engagement。在该 Engagement 内，OAW 评估任务、展示可用选择、等待用户显式确认，
并把选择绑定到交付物。检测结果可以为 gate 提供信息，但绝不会代替用户选择。

## 一份策略，多个 Agent 工具

当开发者在 Claude Code、Codex CLI、Gemini CLI、OpenCode、Cursor、Windsurf、Cline、
Roo Code 和 GitHub Copilot 之间切换时，workflow 所有权仍应保持稳定。这些工具不共享
同一个指令文件名或作用域模型。有些同时提供用户级和项目级指令入口；另一些只有可靠的
项目级入口，而其全局配置由 GUI 管理、依赖平台、仍属实验性或稳定性较低。

为每个客户端手工维护独立策略会造成跨客户端 drift。某个文件可能漏掉新 profile、保留
旧 hybrid 映射，或描述不同的切换规则。每个文件单独看都可能有效，因此这种差异尤其
难以察觉。

OAW 改为在自己的 XDG namespace 中维护一份 canonical rule source。安装器渲染一个
小型 target-native Activation Router。Router 对普通请求保持安静，只在显式激活后读取完整
Policy。Adapter 只翻译指令入口，不分叉生命周期语义。机械 marker comment 只建立
文件系统所有权，不声称模型优先级。

## Provider 独立性

Workflow provider 保持独立安装、许可、版本管理和配置。OAW 检测已知 capability
indicator，并在用户选择兼容 profile 后进行路由。它不安装、更新或删除 provider，
也不 vendor 或 patch 它们的 skill。

这一边界同时影响信任和维护：

- 用户自行选择 provider 版本和安装渠道。
- 上游许可证与配置继续由上游和用户控制。
- OAW 可以报告 profile 不可用，但不会静默替换 family 或省略所需阶段。
- 更新 OAW 不代表获得下载或执行 provider 代码的授权。
- bounded specialist add-on 只能贡献一个声明的交付物，不会成为第二生命周期 owner。

Agent 工具同样保持独立。OAW 只安装自己的策略和 adapter；它不安装客户端，也不修改
GUI-only 的全局规则存储。

## Core、Coordination 与 Host 边界

OAW 拥有仲裁策略、target-specific 入口、OAW Core 编译、带校验和的 Install State 和
可恢复 backup。可选 Workflow Coordinator 只拥有合作客户端的 Workflow State。Agent Host
拥有物理执行权限。这个边界刻意保持有限：

1. 用户未显式激活 OAW 时，请求保持原生 Host。
2. 为一个交付物建立一个 OAW Engagement，并先运行保证等级预检。
3. 仅对已激活任务分类为 `DIRECT`、`BOUNDED` 或 `WORKFLOW`。
4. 对 Workflow Mode 展示支持的选择，并阻塞等待显式确认。
5. 如实使用 `policy-cooperative`、`core-backed` 或 `coordinator-backed` 声明。
6. 遇到 drift 时关闭失败；只有用户显式 force 时才先备份再变更。
7. uninstall 只删除 OAW-owned 构件。

`DIRECT` 和 `BOUNDED` 不创建 Workflow State。OAW 绝不启动 model process。`CURRENT`
原样使用当前会话，`SUBAGENT` 只有在当前 Host 能创建原生 child 时才可用。

OAW 不判断哪种方法论普遍最好。它的初始三 family 评估只是受经验边界约束的设计输入，
详见[对比文档](comparison.md)。规范性的所有权与切换规则仍位于
[policy/ENGINEERING.md](../../policy/ENGINEERING.md)；[生命周期指南](lifecycle.md)
负责解释如何应用这些规则。

## 结果

最终，普通工作保持原生 Host，每个已激活交付物只有一个显式 workflow 决定。
Provider 可以在 OAW Engagement 内共存而不争夺同一阶段；客户端配置可以变化而不产生
第二份治理来源；安装生命周期操作保持本地、可复核且可恢复。
