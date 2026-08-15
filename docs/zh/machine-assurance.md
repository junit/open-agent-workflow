# Machine Assurance

`oaw-assurance` 是可选可执行程序，面向需要为一个已选 Markdown Profile 生成精确机器身份声明的用户。

## Assurance Overlay

Overlay 固定完整 Profile digest，并将确定的 Skill 或 Host-action occurrence 映射到精确的 Provider、
distribution、Host、Binding、invocation、内容和 evidence 身份。无法证明请求的 occurrence 或 Binding
时，签发与验证会 fail closed。

Overlay 不包含工程方法、Responsibility ownership、顺序、进度、approval、执行结果或完成声明。
它不能选择或修改 Profile，也不能授权 Host action。

## 命令

```text
oaw-assurance overlay inspect --profile SOURCE:ID
oaw-assurance overlay issue --profile SOURCE:ID --input INPUT.json
oaw-assurance overlay verify --profile SOURCE:ID --input OVERLAY.json
```

该组件通过共享的只读 Profile inspector 读取 Profile，并独立构建和安装；默认 `oaw` 可执行程序
不依赖它。

Assurance 失败只表示请求的机器声明不可用，不会改变已选 Profile 或正常 Policy 工作流。
