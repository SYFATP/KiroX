# KiroX Web / Docker 部署

Web 模式用于无桌面 Linux 服务器。默认端口为 `8171`，推荐只绑定宿主机本机地址，再由外部 nginx + 域名 + HTTPS 反代访问。

## Docker Compose

Web 登录密码现在可以保存到 `storage.conf`：

```ini
web_password=change-me
```

Docker 中默认配置文件路径通常是：

```text
/root/.config/kirox/storage.conf
```

推荐挂载 `./config` 持久化该目录：

```yaml
ports:
  - "127.0.0.1:8171:8171"
volumes:
  - ./config:/root/.config/kirox
  - ./data:/data
  - ./output:/output
```

首次启动后也可以在 Web UI 的“设置 -> Web 登录密码”中保存密码，保存后重启容器生效。

如需临时覆盖配置文件密码，也可以继续使用环境变量：

```yaml
environment:
  KIROX_WEB_PASSWORD: change-me
```

密码优先级：`--web-password` > `KIROX_WEB_PASSWORD` > `storage.conf` 里的 `web_password` > 空密码。

启动：

```bash
docker compose up -d --build
```

本机验证：

```bash
curl http://127.0.0.1:8171/api/session
```

## nginx 反代

```nginx
server {
    server_name kirox.example.com;

    location / {
        proxy_pass http://127.0.0.1:8171;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

TLS/证书建议交给 nginx/certbot 处理。

## 路径说明

Web 模式里“存储目录/输出目录”填写的是服务器或容器内路径，不是浏览器本机路径。Docker 部署建议：

- 数据目录：`/data`
- 输出目录：`/output`

对应宿主机挂载：

- `./data:/data`
- `./output:/output`

## 本地二进制运行

```bash
npm --prefix frontend run build
go build -o kirox .
./kirox --web
```

可选密码来源：

```bash
# 命令行参数，优先级最高
./kirox --web --web-password your-password

# 环境变量，优先级次之
KIROX_WEB_PASSWORD=your-password ./kirox --web

# 或编辑 storage.conf
web_password=your-password
```

默认监听：

```text
127.0.0.1:8171
```
