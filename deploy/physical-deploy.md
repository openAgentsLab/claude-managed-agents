# AI-Forge 物理机部署指南（Local 沙箱）

工具调用直接在 forge 进程内执行，无需 Docker 或 Kubernetes。适合开发环境、单用户内部工具、受信任的团队场景。

> **Local 沙箱说明**：bash、文件操作等工具在 forge 自身进程中直接 exec，与宿主机共享文件系统和网络，没有进程隔离。如需多用户隔离或生产安全边界，请参考 [docker-deploy.md](docker-deploy.md) 或 [k8s-deploy.md](k8s-deploy.md)。

## 目录

- [环境要求](#环境要求)
- [依赖服务安装](#依赖服务安装)
- [获取代码](#获取代码)
- [构建前端](#构建前端)
- [构建后端](#构建后端)
- [配置](#配置)
- [运行服务](#运行服务)
- [使用 systemd 管理服务](#使用-systemd-管理服务)
- [Nginx 反向代理](#nginx-反向代理)
- [验证部署](#验证部署)

---

## 环境要求

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| OS | Linux (Ubuntu 22.04+ / CentOS 8+) | 推荐 Ubuntu 22.04 LTS |
| Go | 1.26+ | 编译后端二进制 |
| Node.js | 20+ | 构建前端静态文件 |
| npm | 10+ | 前端包管理 |
| PostgreSQL | 14+ | 生产环境数据库 |
| Redis | 7+ | 会话缓存（可选） |
| Nginx | 1.20+ | 静态文件服务 + 反向代理 |

---

## 依赖服务安装

### 安装 PostgreSQL

```bash
# Ubuntu 22.04
sudo apt update
sudo apt install -y postgresql postgresql-contrib

# 启动并设置开机自启
sudo systemctl enable --now postgresql

# 创建数据库和用户
sudo -u postgres psql <<'EOF'
CREATE USER forge WITH PASSWORD 'your_strong_password';
CREATE DATABASE forge OWNER forge;
GRANT ALL PRIVILEGES ON DATABASE forge TO forge;
EOF
```

### 安装 Redis（可选）

```bash
# Ubuntu 22.04
sudo apt install -y redis-server

# 修改配置，绑定本地并设置密码
sudo sed -i 's/^# requirepass .*/requirepass your_redis_password/' /etc/redis/redis.conf
sudo sed -i 's/^bind 127.0.0.1.*/bind 127.0.0.1/' /etc/redis/redis.conf

sudo systemctl enable --now redis-server
```

### 安装 Go 1.26

```bash
# 下载并安装 Go
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz

# 添加到 PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

go version
```

### 安装 Node.js 20

```bash
# 使用 NodeSource 安装
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

node --version
npm --version
```

---

## 获取代码

```bash
# 克隆代码仓库（根据实际情况修改）
git clone https://github.com/your-org/ai-forge.git /opt/ai-forge
cd /opt/ai-forge
```

---

## 构建前端

```bash
cd /opt/ai-forge/web

# 安装依赖
npm install

# 生产构建
npm run build
# 输出目录：/opt/ai-forge/web/dist/

# 将构建产物拷贝到 Nginx 静态目录
sudo mkdir -p /var/www/ai-forge
sudo cp -r dist/* /var/www/ai-forge/
sudo chown -R www-data:www-data /var/www/ai-forge
```

---

## 构建后端

```bash
cd /opt/ai-forge

# 安装 Go 依赖
go mod download

# 编译二进制（生产构建，关闭 CGO，方便跨环境部署）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /usr/local/bin/forge ./cmd/forge

# 验证
forge --version
```

---

## 配置

### 创建配置目录

```bash
sudo mkdir -p /etc/ai-forge
sudo mkdir -p /var/lib/ai-forge/workspaces
sudo mkdir -p /var/log/ai-forge
```

### 主配置文件 `/etc/ai-forge/forge.yaml`

```yaml
server:
  http_addr: ":8080"

log:
  level: "info"        # debug | info | warn | error
  format: "json"       # json 格式便于日志收集

model:
  provider: "anthropic"        # anthropic | openai
  api_key: "${ANTHROPIC_API_KEY}"
  base_url: "https://api.anthropic.com"
  model: "claude-sonnet-4-6"
  # 如使用内部代理，修改 base_url：
  # base_url: "http://ai-service.internal.com"

storage:
  postgres:
    default:
      dsn: "postgres://forge:your_strong_password@localhost:5432/forge?sslmode=disable"
  redis:
    default:
      addr: "localhost:6379"
      password: "your_redis_password"
      db: 0

session:
  driver: "postgres.default"   # 生产环境使用 PostgreSQL

memory:
  static:
    driver: "postgres.default"

sandbox:
  driver: "local"              # 工具在 forge 进程内直接执行，无容器隔离
  workspace_root: "/var/lib/ai-forge/workspaces"

tasks:
  driver: "postgres.default"
```

### 环境变量文件 `/etc/ai-forge/forge.env`

```bash
# LLM API Keys
ANTHROPIC_API_KEY=sk-ant-your-key-here
# OPENAI_API_KEY=sk-your-openai-key

# Web 搜索（可选）
# BRAVE_API_KEY=your-brave-key
# SERPER_API_KEY=your-serper-key

# GitHub MCP（可选）
# GITHUB_PERSONAL_ACCESS_TOKEN=ghp_your-token
```

```bash
# 设置权限，防止密钥泄露
sudo chmod 600 /etc/ai-forge/forge.env
sudo chmod 600 /etc/ai-forge/forge.yaml
```

### MCP 配置文件 `/etc/ai-forge/forge.mcp.yaml`（可选）

```yaml
mcp:
  servers: []
  # 根据需要添加 MCP 服务配置
```

---

## 运行服务

### 创建系统用户

```bash
sudo useradd --system --no-create-home --shell /bin/false forge
sudo chown -R forge:forge /var/lib/ai-forge /var/log/ai-forge /etc/ai-forge
```

### 手动启动（测试）

```bash
sudo -u forge forge serve \
  --config /etc/ai-forge/forge.yaml \
  --mcp-config /etc/ai-forge/forge.mcp.yaml
```

---

## 使用 systemd 管理服务

### 创建 systemd 单元文件 `/etc/systemd/system/ai-forge.service`

```ini
[Unit]
Description=AI-Forge Backend Service
Documentation=https://github.com/your-org/ai-forge
After=network.target postgresql.service redis.service
Wants=postgresql.service

[Service]
Type=simple
User=forge
Group=forge
WorkingDirectory=/var/lib/ai-forge

# 加载环境变量
EnvironmentFile=/etc/ai-forge/forge.env

# 启动命令
ExecStart=/usr/local/bin/forge serve \
    --config /etc/ai-forge/forge.yaml \
    --mcp-config /etc/ai-forge/forge.mcp.yaml

# 进程管理
Restart=on-failure
RestartSec=5s
KillMode=mixed
TimeoutStopSec=30

# 日志
StandardOutput=append:/var/log/ai-forge/forge.log
StandardError=append:/var/log/ai-forge/forge-error.log

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/ai-forge /var/log/ai-forge /etc/ai-forge

[Install]
WantedBy=multi-user.target
```

### 启用并启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable ai-forge
sudo systemctl start ai-forge

# 查看状态
sudo systemctl status ai-forge

# 查看日志
sudo journalctl -u ai-forge -f
# 或直接查看日志文件
sudo tail -f /var/log/ai-forge/forge.log
```

---

## Nginx 反向代理

### 安装 Nginx

```bash
sudo apt install -y nginx
```

### 配置文件 `/etc/nginx/sites-available/ai-forge`

```nginx
# HTTP → HTTPS 跳转
server {
    listen 80;
    server_name forge.example.com;
    return 301 https://$host$request_uri;
}

# HTTPS 主站
server {
    listen 443 ssl http2;
    server_name forge.example.com;

    # SSL 证书（使用 Let's Encrypt 或自签名证书）
    ssl_certificate     /etc/nginx/ssl/forge.crt;
    ssl_certificate_key /etc/nginx/ssl/forge.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # 前端静态文件
    root /var/www/ai-forge;
    index index.html;

    # 前端路由（SPA history 模式）
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 后端 API 代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE（Server-Sent Events）流式响应
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
        chunked_transfer_encoding on;
    }

    # 管理接口（限制访问 IP）
    location /admin/ {
        allow 10.0.0.0/8;
        allow 192.168.0.0/16;
        deny all;

        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 健康检查（不记录日志）
    location /healthz {
        proxy_pass http://127.0.0.1:8080;
        access_log off;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/ai-forge /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 申请 Let's Encrypt 证书（推荐）

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d forge.example.com
```

---

## 验证部署

```bash
# 1. 检查后端健康状态
curl http://localhost:8080/healthz

# 2. 通过 Nginx 检查
curl https://forge.example.com/healthz

# 3. 检查 PostgreSQL 连接
sudo -u forge psql "postgres://forge:your_strong_password@localhost:5432/forge" -c "SELECT 1;"

# 4. 检查后端日志
sudo tail -f /var/log/ai-forge/forge.log

# 5. 检查 Nginx 错误日志
sudo tail -f /var/log/nginx/error.log
```

---

## 常见问题

### forge 服务启动失败

```bash
# 查看详细错误
sudo journalctl -u ai-forge --no-pager -n 50

# 常见原因：
# 1. 配置文件路径不对或格式错误
# 2. PostgreSQL 连接失败（确认 DSN 正确）
# 3. 端口被占用（lsof -i :8080）
```

### 前端页面空白

```bash
# 确认静态文件已部署
ls -la /var/www/ai-forge/

# 确认 Nginx 根目录配置正确
nginx -T | grep root
```

### SSE 流式响应中断

Nginx 默认启用响应缓冲，确保 `proxy_buffering off` 已配置在 `/api/` location 中。
