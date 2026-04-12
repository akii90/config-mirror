# Config Mirror
将特定的 Secret 和 ConfigMap 资源， 基于 Annotation 的策略，自动同步到指定的多个 Namespace（包括现有及未来新建的 Namespace）中，并保障资源状态的一致性。

## Install

```shell
kubectl create -f config
```

## Enable Mirror 
为 ConfigMap/Secret 资源打上 `config-mirror.example.com/allow-mirror: "true"` 的 Annotation, controller

## Mirror range
- `config-mirror.example.com/namespace-include`: 允许同步的 Namespace 列表（逗号分隔，元素支持正则表达式，如 `app-.*,default`）。
- `config-mirror.example.com/namespace-exclude`: 拒绝同步的 Namespace 列表（逗号分隔，元素支持正则表达式，如 `kube-system,istio-.*`）。优先级高于 Include。
