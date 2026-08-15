# 安全策略

## Supported Version

最新 tag release 和当前 `main` 分支是 supported version；更早快照不提供独立安全维护。

## Private Reporting

不要在公开 issue 中披露漏洞细节或敏感配置。请创建不含利用细节的最小 issue，并请求维护者提供
private 报告渠道。报告应包含受影响版本、前置条件、最小复现、影响以及可用的缓解方案。

项目没有专用安全地址，也不承诺 response SLA。

## Trust Boundary

Agent Host、操作系统、仓库、凭证、sandbox、approval 和用户仍然拥有物理权限。OAW Policy 不授予权限，
也不启动 model process。

Go installer 校验拥有的 destination、拒绝 symlink 重定向、在 apply 前准备 mutation，并将 Install State
保密。这些控制不能让不可信 checkout 变安全，也不保护所选根目录之外的文件。

install.sh 只执行同目录的 oaw 或 oaw.exe，从不搜索 PATH、下载代码或在运行时构建。使用前校验 release checksum。

Machine Assurance 与 Bridge 是可选证据组件，不能选择或 veto Policy Profile，也不能强制 sandbox。Host
安全策略仍可独立拒绝物理调用。
