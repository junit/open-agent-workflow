# 安装器

安装器在 User 或 Project scope 管理一套 Canonical Policy Set。它不安装 Skill、不读取 Skill 内容、
不调用 model，也不创建工作流执行状态。

## 命令

~~~text
oaw check [--project PATH] [--target IDS]
oaw install [--project PATH] [--target IDS] [--dry-run] [--force]
oaw update [--project PATH] [--target IDS] [--dry-run] [--force]
oaw uninstall [--project PATH] [--target IDS] [--dry-run] [--force]
oaw profile list
oaw profile show SOURCE:ID
oaw profile check SOURCE:ID
~~~

User installation 默认写入用户 Host 指令文件。Project installation 将自包含的 Policy Set 写入
PATH/.oaw/policy 和项目 Adapter。项目 set 优先于 User set，但不会合并。

## 所有权

Managed block 会保留 Host 指令周围内容。只有目标不存在时才创建 owned file。Install State 记录本次
安装拥有的 Policy Set 文件、target、checksum、scope 和目录。它只是更新与卸载的私有记账，不是工作流进度。

install、update、uninstall 在写入前校验所有路径和源。Force 可以在已跟踪内容漂移时创建私有 backup，
但不会接管未跟踪文件，也不会改变另一 scope 的安装。

## 包装器与发布

install.sh 只解析同目录的 oaw 或 oaw.exe，不搜索 PATH 中的其他可执行文件，不下载 release，也不构建代码。
Release archive 包含预编译二进制、包装器、Policy 文档和 checksum。

从源代码构建：go build -o ./oaw ./cmd/oaw。执行前使用发布的 SHA256SUMS 校验 release。
