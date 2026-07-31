# 为 Open Agent Workflow 贡献

Open Agent Workflow（OAW）接受按 issue 划分的纵向改动，每个改动只负责一个可观察
结果，并保持 provider-neutral 生命周期契约。行为变更应先在 issue 中讨论。

## 开发契约

- 保持 Bash 3.2 兼容，不增加 Node.js、Python、jq 或包管理器运行时依赖。
- 不 vendor、安装、更新或修改任何 workflow provider。
- 通过真实 CLI 的 black-box seam 验证行为；测试必须使用隔离的 HOME、
  XDG_CONFIG_HOME、XDG_STATE_HOME 和项目根目录。
- 将聚焦测试加入编号套件。文档契约位于 tests/10-docs-test.sh，完整套件由
  tests/run.sh 执行。
- 用户可见行为、命令、安全说明或支持级别发生变化时，保持 English/Chinese
  文档语义一致。
- remote publication、push、release、凭据以及第三方资源修改必须由所有者另行
  批准，安装器代码不得执行这些操作。

## Adapter Evidence

每个适配器变更都必须提供 adapter evidence：

1. 官方一手来源 URL 和 retrieval date。
2. 精确的用户级、项目级目标路径和支持级别。
3. loader、import/reference、precedence 与 reload 行为。
4. 路径所有权、纯渲染和共享目标 collision 规则。
5. black-box fixtures，以及恶意路径、symlink 和惰性数据检查。

适配器不能改变生命周期语义，也不能依赖未记录的 provider 安装方式。实验性行为
必须保持 experimental 标记，直到证据和 fixtures 足以支持升级。

## 提交前

运行：

    bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
    shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
    bash tests/run.sh

检查 diff 中是否存在秘密、无关生成文件、不安全路径展开或缺失的
English/Chinese 对等内容。使用 conventional commit，并说明测试证据。不要在本地
贡献流程中执行 remote publication。
