# 为 Open Agent Workflow 贡献

OAW 是规则驱动产品。所有改动都必须保持静态 Policy Set 为正常产品核心，并让可选机器组件保持 additive。

## 开发契约

- 让 Go 安装管理作为 check、install、update、uninstall 和 advisory Profile inspection 的权威实现。
- 保持 install.sh 为 Bash 3.2 兼容的离线同目录二进制包装器。
- 不安装、更新、修改或 vendor 外部 Skill provider。
- 在隔离的 HOME、XDG_CONFIG_HOME、XDG_STATE_HOME 和项目根目录中验证 CLI。
- 保持 English 与 Chinese 用户文档语义一致。
- Host 特定路径放在 policy/adapters，可移植语义放在 Policy 和 Profile。
- 不要让 scanner、state machine 或 machine evidence 成为 Policy 路径的前置条件。

## 测试

聚焦测试应覆盖 Policy Set validation、安装所有权、Profile discovery、可选组件隔离和 no-binary operation。
运行：

    go test ./... -count=1
    go test -race ./... -count=1
    go vet ./...
    bash -n install.sh tests/*.sh scripts/*.sh
    bash tests/run.sh
    bash scripts/check-docs.sh

检查 diff 中是否有 secret、无关生成文件、不安全路径展开、无用兼容代码和缺失的文档对等内容。
