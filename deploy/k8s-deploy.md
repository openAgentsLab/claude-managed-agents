# AI-Forge Kubernetes 部署指南

每个用户会话独立一个 Pod + Service + PVC，由 forge-backend 通过 K8s API 动态创建和管理。K8s 原生存储（PVC）负责 workspace 持久化，**forge 节点之间不需要共享文件系统**。适合大规模多用户生产部署。

## 文件结构

```
deploy/k8s/
├── base.yaml      # Namespace + ServiceAccount + ConfigMap
├── secret.yaml    # 密钥（不提交 git）
├── app.yaml       # Backend + Worker + Frontend Deployment/Service + Ingress
└── rbac.yaml      # 沙箱 RBAC
```

## 前置条件

- Kubernetes 1.24+
- nginx ingress controller
- cert-manager（可选，用于自动签发 TLS）
- 外部 PostgreSQL（连接信息填入 secret.yaml）
- StorageClass 支持动态 PVC 供应（如 `standard`、`gp2`、Ceph RBD 等）

---

## 架构说明

```
Ingress (forge.example.com)
  /api, /admin → forge-backend:8080
  /            → forge-frontend:80

forge-backend  (Deployment × N)   forge serve   — HTTP API + SSE
forge-worker   (Deployment × N)   forge worker  — 后台任务处理
forge-frontend (Deployment × N)   nginx         — 静态文件

forge-sandbox namespace（动态创建，由 forge-backend 管理）
  每个 session:
    forge-sb-<key>   Pod（RestartPolicy: Never）— 运行 tool-server
    forge-sb-<key>   Service (ClusterIP)        — 稳定的内部访问地址
    forge-ws-<key>   PVC                        — workspace 持久化存储
```

**与 Docker 沙箱的关键区别**：
- workspace 由 PVC 管理，forge-backend 无需挂载共享文件系统
- forge-backend 可以任意副本数水平扩展，无 Session Affinity 要求
- Pod 使用 `RestartPolicy: Never`，tool-server 崩溃后 Pod 进入 Failed 状态；forge-backend 的 Pod Watch 感知到后删除 Pod，下次 Acquire 自动重建（workspace 保留）
- 网络隔离通过 NetworkPolicy 实现（当 session 配置 `networking: limited` 时自动创建）

---

## 部署步骤

### 1. 修改配置

**base.yaml** — 替换镜像地址和 Redis 地址：
```yaml
sandbox.options.image: "your-registry/ai-forge-toolserver:v1.0.0"
```

**secret.yaml** — 填入真实密钥（base64 编码）：
```bash
echo -n 'sk-ant-xxx' | base64                                               # ANTHROPIC_API_KEY
echo -n 'postgres://forge:pass@host:5432/forge?sslmode=disable' | base64   # FORGE_STORAGE_DSN
```

**app.yaml** — 替换镜像地址和域名：
```yaml
image: "your-registry/ai-forge-backend:v1.0.0"
image: "your-registry/ai-forge-frontend:v1.0.0"
host: forge.example.com
```

### 2. 构建沙箱镜像

沙箱镜像（tool-server 镜像）需要包含 `forge` 二进制：

```dockerfile
# Dockerfile.toolserver
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    bash git curl wget python3 python3-pip nodejs npm \
    && rm -rf /var/lib/apt/lists/*
COPY forge /forge
RUN chmod +x /forge
CMD ["/forge", "tool-server", "--addr", ":7777"]
```

```bash
# 编译 forge 二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o forge ./cmd/forge

docker build -f Dockerfile.toolserver \
  -t your-registry/ai-forge-toolserver:v1.0.0 .
docker push your-registry/ai-forge-toolserver:v1.0.0
```

### 3. 部署

```bash
kubectl apply -f deploy/k8s/base.yaml
kubectl apply -f deploy/k8s/secret.yaml
kubectl apply -f deploy/k8s/rbac.yaml
kubectl apply -f deploy/k8s/app.yaml
```

### 4. 验证

```bash
# 查看所有资源
kubectl get all -n forge

# 查看 backend 日志
kubectl logs -f deployment/forge-backend -n forge

# 查看 worker 日志
kubectl logs -f deployment/forge-worker -n forge

# 健康检查
kubectl exec -n forge deployment/forge-backend -- curl -s http://localhost:8080/healthz

# 触发一次对话后查看沙箱资源（有用户使用后出现）
kubectl get pod,svc,pvc -n forge-sandbox
```

---

## 沙箱 RBAC 说明

forge-backend 的 ServiceAccount `forge` 通过跨 namespace RoleBinding，获得在 `forge-sandbox` 中管理 Pod / Service / PVC / NetworkPolicy 和 Watch Pod 的权限。

```bash
# 验证权限是否生效
kubectl auth can-i create pods \
  --as=system:serviceaccount:forge:forge \
  -n forge-sandbox

kubectl auth can-i watch pods \
  --as=system:serviceaccount:forge:forge \
  -n forge-sandbox
```

---

## 沙箱配置参数

`base.yaml` ConfigMap 中 `sandbox.options` 支持以下参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `namespace` | `forge-sandbox` | 沙箱 Pod 所在 namespace |
| `image` | `forge:latest` | tool-server 镜像，**必须替换** |
| `workspace_size` | `1Gi` | 每个 session 的 PVC 大小 |
| `storage_class` | （集群默认） | PVC 使用的 StorageClass |
| `service_account` | — | 沙箱 Pod 使用的 ServiceAccount（可选） |

---

## 常见问题

### 沙箱 Pod 一直 Pending

```bash
kubectl describe pod <pod-name> -n forge-sandbox
```

常见原因：
- StorageClass 不存在或无法动态供应 PVC
- 节点资源不足（CPU / Memory）
- `service_account` 配置了不存在的 ServiceAccount

### 工具调用超时

```bash
# 检查沙箱 Pod 状态
kubectl get pod -n forge-sandbox

# 查看 Pod 日志
kubectl logs <forge-sb-xxx> -n forge-sandbox
```

若 Pod 处于 Failed 状态，forge-backend 的 Pod Watch 会感知并在下次工具调用时重建 Pod（workspace 保留）。

### Ingress SSE 响应中断

确认 Ingress 注解包含：
```yaml
nginx.ingress.kubernetes.io/proxy-buffering: "off"
nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
```
