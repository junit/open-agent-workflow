# 背景

Agent Host 可能同时加载多个独立的工程 Skill 集合，每个集合都可能提供 planning、implementation、
testing、review 或 verification 规则。没有共同规则时，这些触发器会竞争，模型也可能在一个交付物中
途径变化。

OAW 用一套可移植 Policy Set 和命名 Profile 解决这个问题。Policy 说明边界，Profile 说明方法，模型
在某项 Responsibility 成为当前工作时解析可读 Skill。仲裁因此存在于可读规则中，而不是隐藏的可执行程序。

OAW 也为多个 Host 安装薄的原生指令入口。入口跟随同一个 Activation Router，不嵌入 Policy 路径，
同时保留每个 Host 的格式、scope 和 reload 行为。安装器只拥有这些文件及私有记账。

Host 仍是执行权威。OAW 不模拟 model、不启动 child process，也不承诺物理 containment。可选 Machine
Assurance 与 Bridge 可以增加证据，但不是先决条件。
