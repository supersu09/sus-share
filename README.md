# SusShare

SusShare 是 VPN 内的单实例临时共享板。它提供一个 Go 进程、嵌入式 Web UI 和 REST API，用于发布文本与文件；不做设备同步、WebSocket、自动发现、CLI、桌面客户端、数据库或 Redis。

## 配置

复制 `config.example.yaml` 到容器的 `/config/config.yaml`，并挂载持久卷：

```text
/config/config.yaml
/data
```

`auth.password_hash` 留空时可以使用默认密码 `password` 登录；登录后建议立即在设置页修改密码。配置自定义密码时必须是 Argon2id PHC 字符串，不允许明文密码。密码值没有长度、字符类别或复杂度要求，空密码也允许。常见生成方式：

```bash
docker run --rm htrestan/argon2 your-password
```

该示例依赖第三方镜像时，请自行审查其内容；也可以使用任何输出 Argon2id PHC 格式的可信工具。

构建和启动：

```bash
docker build --platform linux/amd64 --build-arg GOPROXY=https://goproxy.cn,direct \
  -t sus-share:0.1.0 -t sus-share:latest .
```

在 `winserver` Docker context 上构建并保留镜像：

```bash
./scripts/build-winserver.sh
```

脚本默认生成 `sus-share:0.1.0` 和 `sus-share:latest`。后续修改版本或强制重建：

```bash
SUS_SHARE_IMAGE_TAG=0.2.0 ./scripts/build-winserver.sh
SUS_SHARE_NO_CACHE=1 ./scripts/build-winserver.sh
```

Compose 部署：

```bash
mkdir -p config data
cp config.example.yaml config/config.yaml
# 可选：编辑 config/config.yaml，配置外部前缀或自定义密码哈希
sudo chown -R 100:101 config data
cp .env.example .env
docker compose up -d
```

`SUSSHARE_BIND` 默认绑定所有网卡。只允许本机访问时改成 `127.0.0.1`。运行时修改密码或保留策略后，容器会重写 `/config/config.yaml`，所以必须挂载可写的配置目录。

## 子路径部署

站点部署在根路径时保持：

```yaml
server:
  base_path: ""
```

通过 Nginx 暴露在 `https://www.suscode.cn/share/` 时设置为：

```yaml
server:
  base_path: /share
```

`base_path` 只能使用单个前导斜杠的绝对路径前缀，不能带尾部斜杠、重复斜杠、反斜杠或 scheme。以下两种 Nginx 形态都支持。

Nginx 保留前缀转发：

```nginx
location = /share {
    return 308 /share/;
}

location /share/ {
    proxy_pass http://127.0.0.1:19876/share/;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Nginx 剥离前缀转发：

```nginx
location = /share {
    return 308 /share/;
}

location /share/ {
    proxy_pass http://127.0.0.1:19876/;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

两种形态下，页面都会从 `/share/style.css`、`/share/app.js` 和 `/share/api/v1/...` 取资源；会话 Cookie 也只作用于 `/share`。

## Web 可修改项

设置页面只能修改：

- 保留策略：1 小时、6 小时、12 小时、24 小时、3 天、7 天。
- 登录密码。

监听地址、端口、TLS、存储路径、文本上限、单文件上限、总容量和日志级别只能通过服务器上的 `/config/config.yaml` 修改。

运行时修改会结构化重写 `config.yaml`，管理员手写注释和格式不保证保留。写回使用同目录临时文件、`fsync` 和原子 `rename`，失败时保留旧配置。
