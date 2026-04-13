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

## Mirrored resource identify
```yaml
  # 记录 source resource 信息
  annotations:
    config-mirror.example.com/mirrored-at: "2026-04-13T04:29:38Z"
    config-mirror.example.com/source-name: test-secret
    config-mirror.example.com/source-namespace: default
    config-mirror.example.com/source-resource-version: "7080290"
  labels:
    config-mirror.example.com/mirrored-from: default-test-secret # 用于搜索并管理 mirrored resources
```

## Cascading Updates & Deletes

均通过关联 Label 管理
- **更新 (Update):** 同步更新所有符合条件的 Namespace 中且携带关联 Label 的衍生资源。
- **删除 (Delete):**
    - 为开启了 `allow-mirror=true` 的 Source 资源注入 Finalizer (如 `config-mirror.example.com/cleanup`)。
    - 当 Source 资源收到 Delete 请求变为 `Terminating` 状态时，Controller 拦截该事件。 获取所有关联的衍生资源并逐个删除。
    - 清理完成后，移除 Source 上的 Finalizer，允许其被集群物理删除。
    - 如果用户手动去掉了某个衍生资源上的关联 Label，该衍生资源不再被 Controller 管理，从而不受源资源删除的影响（保留在集群中）。
