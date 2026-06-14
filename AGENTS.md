# AGENTS.md — aws_server VPN 方案

本文档供 AI Agent 与维护者快速理解本仓库的自建 VPN 体系。**修改 VPN 相关代码前请先读本文。**

## 1. 项目定位

- **仓库**：Go + SQLite 后端，部署在 AWS Lightsail（东京 `ap-northeast-1`）
- **公网 IP 示例**：`54.150.9.209`（以 `/etc/go-sqlite-api/vpn.env` 中 `VPN_SERVER_HOST` 为准）
- **核心能力**：`go-sqlite-api` 提供 VPN 管理面板（用户/Apply/配置导出），并生成 **xray + Hysteria2** 服务端配置
- **目标用户场景**：中国大陆 Mac 使用 **Clash Verge Rev（Mihomo 内核）**；Android 可用 Clash Meta / sing-box；**不要用 sing-box 客户端替代 Mac 上的 Clash**（用户已明确）

## 2. 当前 VPN 架构（2026-06）

```
                    ┌─────────────────────────────────────┐
                    │  AWS Lightsail 54.150.9.209         │
                    │                                     │
  Clash Verge Rev   │  TCP 443 ──► xray (VLESS+REALITY)   │
  (Mihomo)          │  UDP 443 ──► hysteria2 (userpass)   │
       │            │                                     │
       │            │  :8080 go-sqlite-api (vpn-admin)     │
       └───────────►│  :80/:8443 nginx (管理页/HTTPS)     │
                    └─────────────────────────────────────┘

sing-box：已停用，仅保留 JSON 生成供旧客户端/兼容
```

### 2.1 双协议策略

| 层级 | 协议 | 传输 | 端口 | 角色 |
|------|------|------|------|------|
| 主选 | **Hysteria2** | QUIC/UDP | 443 | 高丢包环境更快；Clash 节点名 `{user}-hy2` |
| 备选 | **VLESS + REALITY + Vision** | TCP | 443 | 与 xray 443 共用端口；Clash 节点名 `{user}` |

Clash 的 `PROXY` 组为 **`url-test`**：先测 `owner-hy2`，失败再试 `owner`（VLESS）。

### 2.2 服务端进程

| 进程 | 配置文件 | 日志 |
|------|----------|------|
| `xray` | `/usr/local/etc/xray/config.json` | `/var/log/xray/access.log` |
| `hysteria-server` | `/etc/hysteria/config.yaml` | `journalctl -u hysteria-server` |
| `go-sqlite-api` | SQLite + `/etc/go-sqlite-api/vpn.env` | `journalctl -u go-sqlite-api` |

Apply 流程：`vpn-admin` 点 Apply → Go 写 candidate 文件 → `sudo vpn-admin-apply` → 校验 xray、安装 TLS 自签证书（hy2）、重启 xray + hysteria2、停止 sing-box。

### 2.3 认证与密钥

**REALITY（xray / VLESS 客户端）**

- 环境变量：`VPN_REALITY_PRIVATE_KEY`（服务端）、`VPN_REALITY_PUBLIC_KEY`、`VPN_REALITY_SHORT_ID`、`VPN_REALITY_SNI`（默认 `www.cloudflare.com`）
- 每用户 UUID 存于 `vpn_users` 表；flow 为 `xtls-rprx-vision`

**Hysteria2（userpass）**

- `VPN_HY2_AUTH_SECRET` + 用户 UUID → HMAC-SHA256 取前 24 位 hex 作为密码
- Clash 中 `auth: {name}:{password}`，例如 `owner:xxxxxxxxxxxxxxxxxxxxxxxx`
- `VPN_HY2_PORT`（默认 443）、`VPN_HY2_SNI`（masquerade，默认 `www.bing.com`）
- TLS 证书：`/etc/hysteria/server.crt`（Apply 时自签生成）

**管理面板**

- `VPN_ADMIN_EMAIL` / `VPN_ADMIN_PASSWORD`
- 路径：`/vpn-admin`（国内直连常 reset，需 SSH 隧道访问）

## 3. 客户端配置

### 3.1 Mac — Clash Verge Rev

- 数据目录：`~/Library/Application Support/io.github.clash-verge-rev.clash-verge-rev/`
- **必须从 vpn-admin 下载最新 Clash YAML**（含 `# vpn-config-version` / `# vpn-config-checksum` 头两行）
- 模板参考：`scripts/owner-clash-mac.yaml`（密码占位，勿直接当生产配置）
- 校验脚本：`scripts/check-mac-clash.sh`（需与当前 REALITY 公钥同步）
- 导入后：**设置 → Clash 内核 → 重启内核**
- DNS：`fake-ip`；服务器 IP 必须 `DIRECT`（`IP-CIDR,{host}/32,DIRECT`）

### 3.2 Android / 其他

- **Clash Meta**：`clash-android` 配置（含 geosite 分流、`redir-host` DNS）
- **sing-box**：仍生成 VLESS REALITY JSON（**本地导入**，禁止国内扫远程 `sing-box.json` URL）
- **小火箭 / 通用链接**：VLESS 分享链接 + 二维码

### 3.3 配置版本

- 逻辑在 `vpn_config_manifest.go`：`vpn-config-version` / checksum 防 stale 配置
- 改用户、Apply、或改 `vpn.env` 后应重新下载

## 4. 代码地图（VPN）

| 文件 | 职责 |
|------|------|
| `vpn_server.go` | `xrayServerConfig`、`hysteria2ServerConfig`、Clash hy2 代理与 url-test 组 |
| `vpn_admin.go` | 用户 CRUD、`apply()`、Clash/sing-box/rocket 配置生成、`currentVPNRuntime()` |
| `vpn_admin_ui.go` | 管理页 HTML/JS（部分文案仍写 sing-box，待更新） |
| `vpn_config_manifest.go` | 配置 artifact、版本号、checksum |
| `vpn_config_validate.go` | 下载前校验 |
| `vpn_session_tracker.go` | 解析 xray access log（fallback sing-box 旧日志） |
| `deploy/vpn-admin-apply.sh` | 实际上线 xray + hy2 |
| `deploy/vpn-healthcheck.sh` | systemd timer 健康检查 |
| `deploy/vpn.env.example` | 环境变量模板 |

## 5. 部署与运维

```bash
# 本地部署 Go 服务 + 脚本
REMOTE_HOST=54.150.9.209 ./deploy/deploy.sh

# 服务器上编辑密钥（勿提交 git）
sudo nano /etc/go-sqlite-api/vpn.env

# 安装 hysteria2 二进制（若未装）
# /usr/local/bin/hysteria — 见官方 release v2.x

# 管理页 Apply 或手动：
sudo /usr/local/sbin/vpn-admin-apply

# 状态
systemctl status xray hysteria-server go-sqlite-api
ss -tlnp | grep 443    # xray TCP
ss -ulnp | grep 443    # hy2 UDP
```

**Lightsail 防火墙**：必须同时放行 **TCP 443** 与 **UDP 443**（Hysteria2 依赖 UDP）。

**SSH 隧道访问管理页**（国内）：

```bash
ssh -L 18080:127.0.0.1:8080 -i ~/.ssh/LightsailDefaultKey-ap-northeast-1.pem ubuntu@54.150.9.209
# 浏览器打开 http://127.0.0.1:18080/vpn-admin
```

## 6. 已知问题（实测）

| 现象 | 结论 |
|------|------|
| Mac 测 VLESS 延迟 Timeout | 从国内 IP 到 AWS 的 REALITY 握手失败率极高（~99%+），非 UUID/密钥错误 |
| 服务端 xray 自测 | 本机 VLESS 客户端 → 204 OK，说明服务端配置正确 |
| Mac 测 hy2 Timeout | 可能 **UDP 443 被 ISP/GFW/Lightsail 拦截**；服务端 localhost hy2 自测 OK |
| sing-box 时代偶能上网 | 同一 REALITY 路径，成功率 ~0.5%，换 xray 后端不能根治 TCP REALITY 问题 |
| 管理页 / 远程 sing-box 导入 | 国内未翻墙前访问 `http://54.150.9.209/...` 易 connection reset |
| nginx :8443 | 管理 HTTPS；2053/8444 等端口在 Lightsail 上可能未开放 |

**诊断顺序**

1. Lightsail：UDP/TCP 443 是否入站放行  
2. Clash 是否最新 yaml（含 `owner-hy2`）  
3. 单独测 `owner-hy2` vs `owner`  
4. 服务器 `journalctl -u hysteria-server` 是否有来自用户公网 IP 的连接  
5. xray：`tail -f /var/log/xray/access.log`

## 7. 更好方案评估（稳定 + 快）

目标：**中国大陆 → 自建 AWS JP**，客户端 **仅 Clash Verge Rev**。

### 7.1 当前方案内优先（改动小）

| 措施 | 预期 | 实现成本 |
|------|------|----------|
| **确认 UDP 443 放行** | hy2 可用则体验最佳 | 低：Lightsail 控制台 |
| **hy2 改端口**（如 UDP 8443/3478） | 绕过对 443/UDP 的 QoS | 低：改 `VPN_HY2_PORT` + 防火墙 + Apply |
| **url-test 保留 hy2 + VLESS** | 自动选可用协议 | 已实现 |
| **换 REALITY 目标 SNI/指纹** | 对 TCP REALITY 略有帮助，难根治 | 低 |
| **多端口 REALITY**（8443 TCP） | 若 443 TCP 被干扰可试 | 中：xray 多 inbound + Clash 多节点 |

### 7.2 值得考虑的协议扩展（需开发）

| 方案 | 优点 | 缺点 | 与 Clash 关系 |
|------|------|------|----------------|
| **Hysteria2 多端口/多 SNI** | 已有代码基础 | 仍依赖 UDP | Mihomo 支持 hy2 |
| **TUIC v5** | QUIC，部分网络比 hy2 表现不同 | 同样依赖 UDP；需新增 inbound 生成 | Mihomo 支持 tuic |
| **NaïveProxy**（HTTP/2 CONNECT） | **纯 TCP**，抗 UDP 封锁 | 需域名 + 可信 TLS 或自签；部署与 REALITY 不同 | 需查 Mihomo naive 支持 |
| **VLESS + WS/gRPC + CDN**（Cloudflare） | 流量像正常 HTTPS | 需域名、CDN 配置、延迟可能升 | Clash 支持 |
| **ShadowTLS / Trojan** | TCP 伪装 | 特征与 REALITY 不同，需新 inbound | Clash 支持 |

**不建议作为首选**

- 继续押宝 **单 REALITY 443 TCP** 作为唯一通道（已证明不稳定）  
- 国内 **远程拉配置**（sing-box import URL、未隧道访问 admin）  
- 换回 **sing-box 服务端**（除非有明确原因；当前已迁 xray）

### 7.3 架构级改进（稳定性的上限）

| 方向 | 说明 |
|------|------|
| **换线路/机房** | 部分 AWS IP 段对 GFW 更敏感；可试新加坡/香港其他 ASN（仍自建） |
| **前置 relay** | 境外 cheap VPS 中继 → 内网到 Lightsail（复杂，双跳延迟） |
| **域名 + 443 共用** | nginx SNI 分流：REALITY / hy2 / 正常网站，降低 IP 裸连特征 |
| **监控与自动切换** | 服务端探测 + 通知用户换端口；Clash url-test 已做客户端侧 |

### 7.4 推荐路线图

1. **短期（现在）**：确保 UDP 443 + 最新 Clash；以 **hy2 为主、VLESS 为辅**  
2. **若 hy2 仍 Timeout**：开 **UDP 8443**（或 2083/3478）并生成第二 hy2 节点  
3. **若 UDP 全不可用**：增加 **NaïveProxy 或 WS+CDN** 的 TCP  inbound（需域名）  
4. **长期**：域名 + 多 inbound + url-test/fallback 组（hy2 / tuic / naive / vless）

## 8. Agent 修改规范

1. **最小改动**：VPN 逻辑集中在 `vpn_server.go` / `vpn_admin.go`，Apply 脚本同步更新  
2. **密钥**：只通过 `vpn.env` 或 DB，**禁止写入 git**  
3. **客户端**：改 Clash 模板时同步 `vpn_config_validate.go` 校验规则  
4. **Apply 后**：candidate 文件权限 644（xray 以 nobody 读配置）  
5. **文案**：`vpn_admin_ui.go` 中 sing-box 服务端描述应逐步改为 xray + hy2  
6. **测试**：无 go 环境时可 SSH 到服务器 `xray run -test`、hy2 本机 client 测 204  

## 9. 账号结构（示例）

| name | 用途 |
|------|------|
| owner | Mac 主账号 |
| windows | Windows |
| clg_phone | 手机 |

用户 UUID 与 hy2 密码均在 DB；Apply 后从 vpn-admin 按用户下载配置，**不要手工拼 hy2 密码**（除非调试 `hy2UserPassword` 逻辑）。

---

**维护记录**：2026-06 从 sing-box 迁至 xray TCP 443 + Hysteria2 UDP 443；Clash 双节点 url-test。后续协议扩展优先在 `vpn_server.go` 增加 generator，并在 `vpn-admin-apply.sh` 中上线。
