# Safari HTTPS 与服务器安全加固

## Context

用户要求解决 iOS Safari 地址栏“不安全”，随后进行源代码防护及域名、服务器防攻击，并最终发布全部待上线改动到 `https://212131.cc`。

已确认：用户设备的最终入口为 HTTP 主域名；公网检查中，apex/www 的 HTTP 请求（含 iPhone Safari UA）均返回 301，主域名及 www 的 Let's Encrypt 证书有效，TLS 1.3、证书链和 `wss` 正常。线上已有安全头但部分重复；没有证据表明证书失效，因此不盲目换证。公网未发现 Go 源码或数据库配置泄露，未知静态路径目前会回退到首页。

## 实施方案

### 1. 备份并读取真实运行配置

- 通过 SSH 获取 `nginx -T`、`systemctl cat gofly.service`、Certbot、UFW、fail2ban 和监听端口状态。
- 备份 MySQL、上传目录、Go 项目、全部待发布静态文件、Nginx、Let's Encrypt、systemd 配置；记录文件校验值并确认备份可读取。

### 2. 统一 Safari 的 HTTPS 入口

- 以远端实际 Nginx 配置为准，将所有 apex/www 的 HTTP 入口统一为一跳 `301 https://212131.cc$request_uri`，HTTPS www 同样跳转主域名，仅保留 ACME 续期挑战例外。
- 保持 TLS 1.2/1.3 和完整证书链，检查 `certbot renew --dry-run`、自动续期 timer 与 Nginx reload hook；不因 Safari 提示直接更换有效证书。
- HSTS、CSP、`X-Content-Type-Options`、`X-Frame-Options` 等仅由一个层级发送，避免当前重复响应头；暂不提交 HSTS preload。
- 发布后清除该域名在 iPhone 上的旧缓存与网站数据，用私密窗口验证最终地址为 HTTPS。若仍停留 HTTP，通过 Safari Web Inspector 检查实际请求链，而不是继续修改证书。

### 3. 建立可重复的前端发布防护

- 新增本地发布脚本，将 HTML/CSS、内联 JavaScript 和独立 JavaScript 生成压缩、轻混淆的发布产物到独立 `dist` 目录；保留 `/workspace` 中可读源文件，不生成或上传 source map。
- 混淆保留全局接口和登录、WebSocket、卡片消息等公共函数行为，先保留现有 CSP 兼容配置，避免一次性收紧导致页面失效。
- Nginx 禁止目录浏览，敏感文件、源码、配置、日志、备份及 `.map` 精确返回 404；未知静态资源不再回退首页；上传目录禁止脚本执行。
- 明确边界：前端混淆只能提高阅读成本，真实权限、密钥和防攻击必须在后端及服务器完成。

### 4. 加固应用与高频接口

- `tools/jwt.go`：移除硬编码 `golivechat` 密钥，改为从权限为 600 的服务器环境配置读取至少 32 字节随机密钥，并显式限制 HS256；发布后旧 token 失效，客服重新登录。
- `router/api.go`：核对现有调用后，将 `/install`、`/register`、`/clear_online_tcp`、`/visitors_online` 纳入合适的 JWT/RBAC 权限，避免公开安装、建号、清理连接和读取访客信息。
- `controller/message.go`：保留现有上传功能，同时校验图片真实内容/MIME、扩展名和大小，使用不可预测文件名；保持图片副标题及文件上传兼容。
- `middleware/cross.go`：移除通配 Origin 与 credentials 的危险组合，按实际同源/嵌入需求配置允许来源。
- Nginx 对 `/check`、上传和消息接口分别设置按 IP 的请求频率限制与 429 响应；设置连接数、请求体和超时限制。上传位置保持约 92MB 上限，Go 继续限制 90MiB，WebSocket 保留 Upgrade 与长连接参数。

### 5. 主机与网络防护

- 隐藏 Nginx 版本，启用 UFW/云安全组最小开放策略，仅开放 SSH、80、443；确认 Go、MySQL 仅供本机访问。
- fail2ban 先启用 SSH 和明确的 Nginx 滥用规则，并加入管理端固定 IP 白名单。
- 不直接切断当前 SSH 访问。先创建并验证 sudo 用户及新 SSH 密钥，再禁用 root 密码登录和密码认证；未提供新公钥前保留可回退登录方式。

### 6. 备份发布、验证与回滚

- 将全部已完成但尚未发布的本地功能与安全改动一起构建；执行 Go 测试、Linux 构建、前端脚本及混淆产物检查、`nginx -t`。
- 先在本机健康检查新 Go 二进制，再重启 `gofly.service` 并 reload Nginx；异常时恢复旧二进制、静态文件和配置。
- 验证所有 HTTP 入口均跳转、证书可信、安全头仅一份、敏感路径 404、限流返回 429 且正常请求可恢复。
- 使用实体 iPhone Safari 验证地址栏不再显示“不安全”；同时回归双客服登录、访客接入、双向 WebSocket、断线重连、图片/文件及链接卡片、后台 iframe 和消息上传。

## 关键文件与配置

- 本地：`cmd/server.go`、`router/api.go`、`middleware/cross.go`、`middleware/jwt.go`、`tools/jwt.go`、`controller/message.go`、四个主要 HTML 页面及 `assets/auth.js`。
- 新增本地发布脚本与独立 `dist` 产物；脚本和源文件不部署到公开静态目录。
- 远端：`/etc/nginx/sites-enabled/nfra-kefu` 及实际引用的 Nginx 配置、`/etc/letsencrypt`、`gofly.service`、安全环境文件、SSH、UFW 和 fail2ban 配置。

## 执行前置条件

需要 `210.209.125.44` 的 root/sudo SSH 权限、维护窗口、两台客服测试账号及实体 iPhone 配合验收。若执行 SSH 密钥加固，还需要用户的新 SSH 公钥和管理端固定出口 IP。
