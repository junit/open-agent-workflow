# 安全策略

## Supported Version

当前 active local branch 上尚未发布的 0.1.0 candidate 是唯一 supported version；
更早快照不提供独立安全维护。

## Private Reporting

不要在公开 issue 中披露漏洞细节或敏感配置。请创建一个不含利用细节的最小 issue，
请求维护者指定 private 报告渠道。报告应包含受影响版本、前置条件、最小复现、影响，
以及可用时的缓解建议。

项目目前没有公布专用安全地址。维护者会尽力确认并调查，但不提供有保证的响应服务
级别协议（no guaranteed response SLA），也不承诺固定的 embargo 时间。

## Installer Trust Boundary

installer trust boundary 包括公开 Go binary、用于构建它的可选源码 checkout、命令行
参数、HOME、XDG_CONFIG_HOME、XDG_STATE_HOME、可选项目根目录、已有适配器文件，
以及 OAW 状态和备份数据。选定的 checkout 与解压后的 binary 都应被视为可执行代码。
发布归档包含预编译二进制，management 在运行时不会下载可执行文件。

`install.sh` 是最小离线兼容包装器。它只执行旁边的 `oaw` 或 `oaw.exe`，绝不使用
`PATH` candidate、下载 artifact 或 runtime build。执行前应验证 archive checksum 并复核
release source。

OAW 校验注册表拥有的目标、拒绝 symlink 重定向、以不求值方式解析状态、在 apply
前完成准备，并在强制处理 drift 前备份。这些控制不能让不可信 checkout 变得安全，
也不负责保护所选根目录之外的文件免受其他软件影响。

Install State 与 Workflow State 使用独立 namespace 和 authority model，不会自动迁移。
OAW Core 与 Workflow Coordinator record 必须 secret-free，只保留 opaque digest reference。
Capability Grant 或 Resource Lease 只为 cooperating clients 表达 logical workflow authority；
Agent Host 拥有物理执行权限，包括 Host sandbox and approvals。OAW never starts a model CLI。

Host integration 只能暴露 `policy` 或明确的 `host-native` surface。后者可以报告 session
fact 与 Receipt，但不会让 OAW 拥有 Host tool。OAW never guarantees MCP、Hook、Skill 或
Plugin inheritance 到 child context；这些事实由 active Host 决定。Grant 不能物理阻止
Host 在协议外执行 action。

## 报告处理

维护者应在隔离根目录中复现，避免泄露报告者数据，记录可利用性和严重性，在合适时
添加 black-box 回归，并在协调修复后再发布细节。若报告证明凭据已泄露，应立即轮换；
OAW 的正常安装器操作不需要凭据。
