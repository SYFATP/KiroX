# KiroX Web / Docker 部署

Web 模式用于无桌面 Linux 服务器。默认端口为 `8171`，推荐只绑定宿主机本机地址，再由外部 nginx + 域名 + HTTPS 反代访问。

## Docker Compose

修改 `docker-compose.yml` 中的密码：

```yaml
environment:
  KIROX_WEB_PASSWORD: change-me
ports:
  - "127.0.0.1:8171:8171"
```

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
KIROX_WEB_PASSWORD=your-password ./kirox --web
```

默认监听：

```text
127.0.0.1:8171
```
