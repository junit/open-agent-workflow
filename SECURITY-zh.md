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

installer trust boundary 包括当前本地 checkout、命令行参数、HOME、
XDG_CONFIG_HOME、XDG_STATE_HOME、可选项目根目录、已有适配器文件，以及 OAW
状态和备份数据。选定的本地 checkout 应被视为可执行代码。OAW 不获取或执行远程
代码。

OAW 校验注册表拥有的目标、拒绝 symlink 重定向、以不求值方式解析状态、在 apply
前完成准备，并在强制处理 drift 前备份。这些控制不能让不可信 checkout 变得安全，
也不负责保护所选根目录之外的文件免受其他软件影响。

## 报告处理

维护者应在隔离根目录中复现，避免泄露报告者数据，记录可利用性和严重性，在合适时
添加 black-box 回归，并在协调修复后再发布细节。若报告证明凭据已泄露，应立即轮换；
OAW 的正常安装器操作不需要凭据。
