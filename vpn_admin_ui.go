package main

import (
	"fmt"
	"net/http"
)

func (a *app) vpnAdminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, vpnAdminHTML)
}

const vpnAdminHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>VPN Control</title>
  <style>
    :root {
      --bg: #eef2f7;
      --panel: #ffffff;
      --panel2: #f8fafc;
      --text: #142033;
      --muted: #66758d;
      --line: #d9e0ea;
      --blue: #2563eb;
      --blue2: #1d4ed8;
      --green: #0f8a4b;
      --red: #d92d20;
      --shadow: 0 18px 50px rgba(16,24,40,.10);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--text);
      background: linear-gradient(180deg, #f7f9fc 0, #eef2f7 260px, #eef2f7 100%);
      letter-spacing: 0;
    }
    button, input, select, textarea { font: inherit; }
    button, a.button {
      min-height: 36px;
      border-radius: 7px;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      padding: 0 13px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      text-decoration: none;
      cursor: pointer;
      white-space: nowrap;
    }
    button.primary, a.primary { background: var(--blue); color: #fff; border-color: var(--blue); }
    button.primary:hover, a.primary:hover { background: var(--blue2); }
    button.danger { color: var(--red); }
    button.subtle { color: var(--muted); }
    input, select, textarea {
      border-radius: 7px;
      border: 1px solid var(--line);
      padding: 0 11px;
      background: #fff;
      color: var(--text);
      min-width: 0;
    }
    input, select { height: 38px; }
    textarea { width: 100%; min-height: 92px; padding: 10px 11px; resize: vertical; }
    label { display: grid; gap: 6px; color: var(--muted); font-size: 12px; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; word-break: break-all; }
    .hidden { display: none !important; }
    .login {
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
    }
    .loginBox {
      width: min(430px, 100%);
      background: rgba(255,255,255,.94);
      border: 1px solid rgba(217,224,234,.9);
      border-radius: 8px;
      box-shadow: var(--shadow);
      padding: 28px;
    }
    .brand { display: flex; align-items: center; gap: 12px; margin-bottom: 24px; }
    .mark {
      width: 42px;
      height: 42px;
      border-radius: 9px;
      background: #132238;
      color: #fff;
      display: grid;
      place-items: center;
      font-weight: 800;
    }
    .brand h1 { margin: 0; font-size: 22px; }
    .brand p { margin: 4px 0 0; color: var(--muted); font-size: 13px; }
    .loginForm { display: grid; gap: 14px; }
    .passwordRow { display: grid; grid-template-columns: 1fr auto; gap: 8px; }
    .passwordRow input { width: 100%; }
    .passwordRow button { height: 38px; }
    .shell { min-height: 100vh; }
    header {
      min-height: 64px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 14px;
      padding: 10px 24px;
      border-bottom: 1px solid var(--line);
      background: rgba(255,255,255,.9);
      backdrop-filter: blur(10px);
      position: sticky;
      top: 0;
      z-index: 2;
    }
    .title { display: flex; align-items: center; gap: 12px; }
    .title h1 { margin: 0; font-size: 18px; }
    .title span { color: var(--muted); font-size: 13px; }
    .userbar { display: flex; align-items: center; gap: 10px; }
    .pill {
      min-height: 28px;
      border-radius: 999px;
      padding: 4px 10px;
      background: var(--panel2);
      border: 1px solid var(--line);
      color: var(--muted);
      display: inline-flex;
      align-items: center;
      font-size: 12px;
    }
    main {
      max-width: 1200px;
      margin: 0 auto;
      padding: 22px;
      display: grid;
      gap: 16px;
    }
    .stats { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
    .chartGrid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
    .card, .panel, .chartCard {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: 0 1px 2px rgba(16,24,40,.04);
    }
    .card { padding: 16px; display: grid; gap: 8px; }
    .card span, .chartCard span { color: var(--muted); font-size: 12px; }
    .card b { font-size: 18px; }
    .panel { padding: 16px; }
    .chartCard { padding: 14px; min-width: 0; }
    .chartCard b { display: block; margin: 4px 0 10px; font-size: 16px; }
    canvas { width: 100%; height: 220px; display: block; }
    .panelHead {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      margin-bottom: 14px;
    }
    .panelHead h2 { margin: 0; font-size: 15px; }
    .headActions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
    .trafficSummary { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 10px; }
    .summaryPill {
      min-height: 30px;
      border-radius: 999px;
      padding: 5px 11px;
      background: var(--panel2);
      border: 1px solid var(--line);
      color: var(--muted);
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 12px;
    }
    .summaryPill b { color: var(--text); font-size: 13px; }
    .segmented {
      display: inline-flex;
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
      background: #fff;
    }
    .segmented button {
      border: 0;
      border-radius: 0;
      border-right: 1px solid var(--line);
      min-height: 34px;
    }
    .segmented button:last-child { border-right: 0; }
    .segmented button.active { background: var(--blue); color: #fff; }
    .formGrid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .formGrid .full { grid-column: 1 / -1; }
    table { width: 100%; border-collapse: collapse; }
    th, td { border-bottom: 1px solid var(--line); padding: 11px 8px; text-align: left; vertical-align: top; font-size: 13px; }
    th { color: var(--muted); font-weight: 650; background: var(--panel2); }
    tr:last-child td { border-bottom: none; }
    .actions { display: flex; gap: 7px; flex-wrap: wrap; }
    .statusOk { color: var(--green); font-weight: 650; }
    .statusBad { color: var(--red); font-weight: 650; }
    .muted { color: var(--muted); }
    .toast { min-height: 22px; color: var(--muted); font-size: 13px; }
    .toast.ok { color: var(--green); }
    .toast.bad { color: var(--red); }
    .note { color: var(--muted); font-size: 12px; margin: 10px 0 0; line-height: 1.5; }
    .deviceList { display: grid; gap: 10px; margin-top: 8px; }
    .deviceItem {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px;
      background: var(--panel2);
      display: grid;
      gap: 8px;
    }
    .deviceItem.online { border-color: #9ec5ff; background: #f8fbff; }
    .deviceMeta { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
    .configLayout { display: grid; grid-template-columns: minmax(0, 1fr) 280px; gap: 16px; align-items: start; }
    .configMeta {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px 14px;
      background: var(--panel2);
      margin-bottom: 14px;
    }
    .configMeta h3 { margin: 0 0 8px; font-size: 14px; }
    .configMeta table { margin: 8px 0 0; }
    .configMeta td code { font-size: 12px; word-break: break-all; }
    .configMeta .actions { margin-top: 0; }
    .configValidate {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px 14px;
      background: #fff;
      margin-bottom: 14px;
    }
    .configValidate h3 { margin: 0 0 8px; font-size: 14px; }
    .validateItem { border-top: 1px solid var(--line); padding-top: 10px; margin-top: 10px; }
    .validateItem:first-of-type { border-top: 0; padding-top: 0; margin-top: 0; }
    .validateHead { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 6px; }
    .validateChecks { margin: 0; padding-left: 18px; line-height: 1.6; font-size: 12px; }
    .validateChecks .checkOk { color: var(--green); }
    .validateChecks .checkBad { color: var(--red); }
    .yamlValidateBox { margin-top: 14px; display: grid; gap: 8px; }
    .yamlValidateBox textarea { min-height: 120px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
    .methodList { margin: 0; padding-left: 20px; color: var(--text); line-height: 1.75; }
    .qrBox {
      display: grid;
      place-items: center;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel2);
      padding: 14px;
    }
    .qrBox img { width: 220px; height: 220px; image-rendering: pixelated; background: #fff; }
    .qrBox img.loading { opacity: 0.35; }
    .qrBox .qrStatus { color: var(--muted); font-size: 12px; margin: 0; }
    .modal {
      position: fixed;
      inset: 0;
      background: rgba(20,32,51,.34);
      display: grid;
      place-items: center;
      padding: 20px;
      z-index: 5;
    }
    .dialog {
      width: min(460px, 100%);
      background: #fff;
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      padding: 18px;
      display: grid;
      gap: 12px;
    }
    .dialog h2 { margin: 0; font-size: 16px; }
    .dialogActions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
    @media (max-width: 860px) {
      header { padding: 10px 14px; }
      main { padding: 14px; }
      .stats, .chartGrid, .formGrid, .configLayout { grid-template-columns: 1fr; }
      .userbar { gap: 6px; }
      .title span { display: none; }
      table, thead, tbody, tr, th, td { display: block; }
      thead { display: none; }
      tr { border-bottom: 1px solid var(--line); padding: 10px 0; }
      td { border-bottom: 0; padding: 6px 0; }
    }
  </style>
</head>
<body>
  <div id="login" class="login">
    <div class="loginBox">
      <div class="brand">
        <div class="mark">VPN</div>
        <div>
          <h1>VPN Control</h1>
          <p>账户登录后管理节点、人员和客户端配置</p>
        </div>
      </div>
      <div class="loginForm">
        <label>账号<input id="email" type="email" autocomplete="username" placeholder="请输入账号邮箱"></label>
        <label>密码
          <div class="passwordRow">
            <input id="password" type="password" autocomplete="current-password" placeholder="请输入密码">
            <button id="togglePassword" type="button">显示</button>
          </div>
        </label>
        <button id="loginBtn" class="primary">登录</button>
        <div id="loginMsg" class="toast"></div>
      </div>
    </div>
  </div>

  <div id="app" class="shell hidden">
    <header>
      <div class="title">
        <div class="mark">VPN</div>
        <div><h1>VPN Control</h1><span>VLESS Reality / TCP 443</span></div>
      </div>
      <div class="userbar">
        <span id="who" class="pill">-</span>
        <button id="logout" class="subtle">退出</button>
      </div>
    </header>
    <main>
      <section id="dashboardView">
        <div id="runtimeStats" class="stats">
          <div class="card"><span>服务器</span><b id="server">-</b></div>
          <div class="card"><span>Reality SNI</span><b id="sni">-</b></div>
        </div>

        <section class="panel" style="margin-top:16px;">
          <div class="panelHead">
            <div>
              <h2 id="trafficTitle">流量趋势</h2>
              <div class="trafficSummary">
                <span class="summaryPill">累计接收 <b id="rx">-</b></span>
                <span class="summaryPill">累计发送 <b id="tx">-</b></span>
              </div>
            </div>
            <div class="segmented">
              <button data-bucket="hour" class="active">小时</button>
              <button data-bucket="6h">6小时</button>
              <button data-bucket="day">天</button>
            </div>
          </div>
          <div class="chartGrid">
            <div class="chartCard"><span>接收流量</span><b id="rxChartTotal">-</b><canvas id="rxChart"></canvas></div>
            <div class="chartCard"><span>发送流量</span><b id="txChartTotal">-</b><canvas id="txChart"></canvas></div>
          </div>
          <p id="trafficNote" class="note"></p>
        </section>

        <section class="panel" style="margin-top:16px;">
          <div class="panelHead">
            <h2 id="tableTitle">人员与配置</h2>
            <div class="headActions">
              <button id="createOpen" class="primary">新增人员</button>
              <button id="apply" class="subtle">重新应用配置</button>
              <button id="refresh">刷新</button>
            </div>
          </div>
          <div id="message" class="toast"></div>
          <table>
            <thead>
              <tr>
                <th>人员</th>
                <th>账号 / 角色</th>
                <th>UUID</th>
                <th>状态</th>
                <th>在线设备</th>
                <th>配置入口</th>
                <th id="opsHead">操作</th>
              </tr>
            </thead>
            <tbody id="users"></tbody>
          </table>
          <p id="devicesNote" class="note"></p>
          <p id="statsNote" class="note"></p>
        </section>
      </section>

      <section id="createView" class="panel hidden">
        <div class="panelHead">
          <h2>新增人员</h2>
          <button id="createBack">返回</button>
        </div>
        <div class="formGrid">
          <label>名称<input id="newName" placeholder="请输入人员名称"></label>
          <label>登录邮箱<input id="newEmail" type="email" placeholder="请输入登录邮箱"></label>
          <label>初始密码<input id="newPassword" type="password" placeholder="不需要登录可留空"></label>
          <label>角色<select id="newRole"><option value="user">普通用户</option><option value="admin">管理员</option></select></label>
          <div class="full actions">
            <button id="createUser" class="primary">创建人员并应用配置</button>
            <button id="createCancel">取消</button>
          </div>
        </div>
        <p class="note">创建后会自动写入 sing-box 服务端配置，新增用户可以到对应配置页面下载文件或扫码导入。</p>
      </section>

      <section id="configView" class="panel hidden">
        <div class="panelHead">
          <div>
            <h2 id="configTitle">配置详情</h2>
            <p id="configSub" class="note"></p>
          </div>
          <button id="configBack">返回</button>
        </div>
        <div id="configBody"></div>
      </section>
    </main>
  </div>

  <div id="deviceModal" class="modal hidden">
    <div class="dialog" style="width:min(560px,100%);">
      <h2 id="deviceModalTitle">在线设备</h2>
      <p id="deviceModalSub" class="note"></p>
      <div id="deviceModalList" class="deviceList"></div>
      <div class="dialogActions">
        <button id="deviceModalClose">关闭</button>
      </div>
    </div>
  </div>

  <div id="editModal" class="modal hidden">
    <div class="dialog">
      <h2>编辑人员</h2>
      <label>名称<input id="editName"></label>
      <label>登录邮箱<input id="editEmail" type="email"></label>
      <label>新密码<input id="editPassword" type="password" placeholder="留空则不修改密码"></label>
      <label>角色<select id="editRole"><option value="user">普通用户</option><option value="admin">管理员</option></select></label>
      <div class="dialogActions">
        <button id="editCancel">取消</button>
        <button id="editSave" class="primary">保存</button>
      </div>
    </div>
  </div>

  <script>
    const $ = (id) => document.getElementById(id);
    let currentUser = null;
    let usersById = new Map();
    let editingId = null;
    let managingDevicesUserId = null;
    let currentBucket = "hour";
    let lastStatus = null;

    function setMsg(id, text, bad) {
      const el = $(id);
      el.textContent = text;
      el.className = bad ? "toast bad" : "toast ok";
    }
    function api(path, options = {}) {
      return fetch(path, {
        ...options,
        credentials: "same-origin",
        headers: { "Content-Type": "application/json", ...(options.headers || {}) }
      }).then(async (res) => {
        if (!res.ok) {
          const text = await res.text();
          throw new Error(text || res.statusText);
        }
        if (res.status === 204) return null;
        return res.json();
      });
    }
    async function apiRetry(path, options = {}, attempts = 3) {
      let lastErr = null;
      for (let i = 1; i <= attempts; i++) {
        try {
          return await api(path, options);
        } catch (err) {
          lastErr = err;
          if (String(err.message).includes("401")) throw err;
          if (i < attempts) {
            await new Promise((resolve) => setTimeout(resolve, 300 * i));
          }
        }
      }
      throw lastErr;
    }
    function fmtBytes(n) {
      if (!n) return "0 B";
      const units = ["B", "KB", "MB", "GB", "TB"];
      let v = n, i = 0;
      while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
      return v.toFixed(i ? 2 : 0) + " " + units[i];
    }
    function escapeHTML(value) {
      return String(value || "").replace(/[&<>"']/g, (ch) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"
      }[ch]));
    }
    function mountConfigQR(imgId, qrURL) {
      const img = $(imgId);
      if (!img) return;
      const status = img.parentElement && img.parentElement.querySelector(".qrStatus");
      if (status) status.textContent = "二维码加载中...";
      img.removeAttribute("src");
      img.classList.add("loading");
      const tryLoad = (attempt) => fetch(qrURL, { credentials: "same-origin", cache: "no-store" })
        .then((res) => {
          if (!res.ok) throw new Error(String(res.status));
          return res.blob();
        })
        .then((blob) => {
          if (!blob.type.startsWith("image/")) throw new Error("invalid-image");
          if (img.dataset.objectUrl) URL.revokeObjectURL(img.dataset.objectUrl);
          const objectUrl = URL.createObjectURL(blob);
          img.dataset.objectUrl = objectUrl;
          img.src = objectUrl;
          img.classList.remove("loading");
          if (status) status.textContent = "";
        })
        .catch(() => {
          if (attempt < 4) {
            return new Promise((resolve) => setTimeout(resolve, 250 * attempt)).then(() => tryLoad(attempt + 1));
          }
          img.classList.remove("loading");
          if (status) status.textContent = "二维码加载失败，请返回后重试，或使用下方导入链接。";
        });
      return tryLoad(1);
    }
    function showView(name) {
      $("dashboardView").classList.toggle("hidden", name !== "dashboard");
      $("createView").classList.toggle("hidden", name !== "create");
      $("configView").classList.toggle("hidden", name !== "config");
      window.scrollTo({ top: 0, behavior: "smooth" });
    }
    function showApp(user) {
      currentUser = user;
      $("login").classList.add("hidden");
      $("app").classList.remove("hidden");
      $("who").textContent = user.login_email + " / " + (user.role === "admin" ? "管理员" : "普通用户");
      $("createOpen").classList.toggle("hidden", user.role !== "admin");
      $("apply").classList.toggle("hidden", user.role !== "admin");
      $("runtimeStats").classList.toggle("hidden", user.role !== "admin");
      $("opsHead").classList.toggle("hidden", user.role !== "admin");
      $("tableTitle").textContent = user.role === "admin" ? "人员与配置" : "我的配置";
      $("trafficTitle").textContent = user.role === "admin" ? "服务器流量趋势" : "我的流量趋势";
      showView("dashboard");
      load();
    }
    function showLogin() {
      currentUser = null;
      $("app").classList.add("hidden");
      $("login").classList.remove("hidden");
    }
    async function load() {
      try {
        const results = await Promise.all([
          apiRetry("/api/vpn/status"),
          apiRetry("/api/vpn/users"),
          apiRetry("/api/vpn/traffic?bucket=" + encodeURIComponent(currentBucket))
        ]);
        lastStatus = results[0];
        renderStatus(results[0]);
        renderUsers(results[1]);
        renderTraffic(results[2]);
        setMsg("message", "已刷新", false);
      } catch (err) {
        if (String(err.message).includes("401")) showLogin();
        const msg = String(err.message || err);
        if (msg === "Failed to fetch" || msg.includes("NetworkError")) {
          setMsg("message", "加载失败：请关闭 Clash 全局/TUN，或重新导入最新 Clash 配置后再刷新", true);
        } else {
          setMsg("message", msg, true);
        }
      }
    }
    function renderStatus(status) {
      $("server").textContent = status.runtime.server_host + ":" + status.runtime.port;
      $("sni").textContent = status.runtime.sni;
      $("rx").textContent = fmtBytes(status.traffic_total.rx_bytes);
      $("tx").textContent = fmtBytes(status.traffic_total.tx_bytes);
      $("statsNote").textContent = status.stats_note;
    }
    function renderTraffic(history) {
      const rxTotal = history.points.reduce((sum, p) => sum + (p.rx_bytes || 0), 0);
      const txTotal = history.points.reduce((sum, p) => sum + (p.tx_bytes || 0), 0);
      $("rxChartTotal").textContent = fmtBytes(rxTotal);
      $("txChartTotal").textContent = fmtBytes(txTotal);
      $("trafficNote").textContent = history.points_note;
      drawChart("rxChart", history.points, "rx_bytes", "#2563eb", history.bucket);
      drawChart("txChart", history.points, "tx_bytes", "#0f8a4b", history.bucket);
    }
    function drawChart(canvasId, points, key, color, bucket) {
      const canvas = $(canvasId);
      const rect = canvas.getBoundingClientRect();
      const ratio = window.devicePixelRatio || 1;
      const width = Math.max(320, Math.floor(rect.width));
      const height = 220;
      canvas.width = width * ratio;
      canvas.height = height * ratio;
      canvas.style.height = height + "px";
      const ctx = canvas.getContext("2d");
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
      ctx.clearRect(0, 0, width, height);
      const pad = { left: 42, right: 14, top: 14, bottom: 34 };
      const plotW = width - pad.left - pad.right;
      const plotH = height - pad.top - pad.bottom;
      ctx.strokeStyle = "#d9e0ea";
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let i = 0; i <= 4; i++) {
        const y = pad.top + plotH * i / 4;
        ctx.moveTo(pad.left, y);
        ctx.lineTo(width - pad.right, y);
      }
      ctx.stroke();
      const values = points.map((p) => p[key] || 0);
      const max = Math.max(1, ...values);
      ctx.fillStyle = "#66758d";
      ctx.font = "11px -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
      ctx.textAlign = "right";
      for (let i = 0; i <= 4; i++) {
        const value = max * (4 - i) / 4;
        const y = pad.top + plotH * i / 4 + 4;
        ctx.fillText(fmtBytes(value), pad.left - 8, y);
      }
      if (points.length === 0) return;
      ctx.beginPath();
      points.forEach((p, i) => {
        const x = pad.left + (points.length === 1 ? plotW : plotW * i / (points.length - 1));
        const y = pad.top + plotH - ((p[key] || 0) / max) * plotH;
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
      });
      ctx.strokeStyle = color;
      ctx.lineWidth = 2.5;
      ctx.stroke();
      ctx.lineTo(width - pad.right, pad.top + plotH);
      ctx.lineTo(pad.left, pad.top + plotH);
      ctx.closePath();
      const gradient = ctx.createLinearGradient(0, pad.top, 0, pad.top + plotH);
      gradient.addColorStop(0, color + "33");
      gradient.addColorStop(1, color + "00");
      ctx.fillStyle = gradient;
      ctx.fill();
      ctx.fillStyle = "#66758d";
      ctx.textAlign = "center";
      const labelIndexes = [0, Math.floor((points.length - 1) / 2), points.length - 1].filter((v, i, a) => a.indexOf(v) === i);
      labelIndexes.forEach((idx) => {
        const p = points[idx];
        const x = pad.left + (points.length === 1 ? plotW : plotW * idx / (points.length - 1));
        ctx.fillText(formatTime(p.time, bucket), x, height - 10);
      });
    }
    function formatTime(value, bucket) {
      const d = new Date(value);
      if (bucket === "day") return String(d.getMonth() + 1) + "/" + d.getDate();
      return String(d.getMonth() + 1) + "/" + d.getDate() + " " + String(d.getHours()).padStart(2, "0") + ":00";
    }
    function deviceTypeOptions(selected) {
      const options = [
        ["unknown", "未知设备"],
        ["phone", "手机"],
        ["mac", "Mac"],
        ["pc", "台式机/Windows"],
        ["tablet", "平板"],
        ["tv", "电视/盒子"],
        ["other", "其他"]
      ];
      return options.map(([value, label]) => '<option value="' + value + '"' + (selected === value ? " selected" : "") + '>' + label + '</option>').join("");
    }
    function renderOnlineDevicesCell(user) {
      const count = user.online_device_count || 0;
      const devices = user.online_devices || [];
      if (count === 0) {
        return '<span class="muted">0 台</span><br><button data-devices="' + user.id + '">管理设备</button>';
      }
      const labels = devices.slice(0, 2).map((d) => escapeHTML(d.display_label)).join(" · ");
      const more = count > 2 ? " 等" : "";
      return '<span class="statusOk">' + count + ' 台在线</span><br><span class="muted">' + labels + more + '</span><br><button data-devices="' + user.id + '">管理设备</button>';
    }
    function renderDeviceModal(user) {
      managingDevicesUserId = String(user.id);
      $("deviceModalTitle").textContent = user.name + " 的设备";
      $("deviceModalSub").textContent = "VPN 协议无法自动识别 Mac/手机型号；首次上线会显示为「未知设备」，你可以手动改名和选择类型。";
      const known = user.known_devices || [];
      if (known.length === 0) {
        $("deviceModalList").innerHTML = '<p class="note">暂无记录。账号连接 VPN 后会在这里出现。</p>';
        return;
      }
      $("deviceModalList").innerHTML = known.map((device) => {
        const status = device.online
          ? '<span class="pill statusOk">在线 · ' + device.active_connections + " 连接</span>"
          : '<span class="pill">离线</span>';
        return '<div class="deviceItem' + (device.online ? " online" : "") + '">'
          + '<div class="deviceMeta"><b>' + escapeHTML(device.display_label) + '</b>' + status + '</div>'
          + '<div class="muted">IP ' + escapeHTML(device.source_ip) + ' · 最近活跃 ' + new Date(device.last_seen_at).toLocaleString() + '</div>'
          + '<label>设备名称<input data-device-name="' + escapeHTML(device.source_ip) + '" value="' + escapeHTML(device.display_name || "") + '" placeholder="例如：iPhone 15、办公室 Mac"></label>'
          + '<label>设备类型<select data-device-type="' + escapeHTML(device.source_ip) + '">' + deviceTypeOptions(device.device_type || "unknown") + '</select></label>'
          + '<div class="actions"><button class="primary" data-save-device="' + user.id + '" data-device-ip="' + escapeHTML(device.source_ip) + '">保存标注</button></div>'
          + '</div>';
      }).join("");
    }
    function renderUsers(list) {
      const isAdmin = currentUser && currentUser.role === "admin";
      usersById = new Map(list.users.map((u) => [String(u.id), u]));
      $("devicesNote").textContent = list.devices_note || "";
      $("users").innerHTML = list.users.map((u) => {
        const ops = isAdmin
          ? '<td class="actions"><button data-edit="' + u.id + '">编辑</button><button data-toggle="' + u.id + '" data-enabled="' + u.enabled + '">' + (u.enabled ? "停用" : "启用") + '</button><button class="danger" data-delete="' + u.id + '">删除</button></td>'
          : '<td class="hidden"></td>';
        return '<tr>'
          + '<td><b>' + escapeHTML(u.name) + '</b><br><span class="muted">' + new Date(u.created_at).toLocaleString() + '</span></td>'
          + '<td>' + escapeHTML(u.login_email || "-") + '<br><span class="pill">' + (u.role === "admin" ? "管理员" : "普通用户") + '</span></td>'
          + '<td><code>' + escapeHTML(u.uuid) + '</code></td>'
          + '<td>' + (u.enabled ? '<span class="statusOk">启用</span>' : '<span class="statusBad">停用</span>') + '</td>'
          + '<td>' + renderOnlineDevicesCell(u) + '</td>'
          + '<td class="actions">'
          + '<button data-config="vless" data-user="' + u.id + '">扫码/通用链接</button>'
          + '<button data-config="rocket" data-user="' + u.id + '">clg 小火箭</button>'
          + '<button data-config="clash" data-user="' + u.id + '">Clash Meta</button>'
          + '<button data-config="sing-box" data-user="' + u.id + '">sing-box</button>'
          + '</td>'
          + ops
          + '</tr>';
      }).join("");
    }
    function renderValidationItems(items) {
      if (!Array.isArray(items) || !items.length) return "";
      return items.map((item) => {
        const checks = (item.checks || []).map((check) => {
          return '<li class="' + (check.ok ? "checkOk" : "checkBad") + '">'
            + escapeHTML(check.name) + "：" + escapeHTML(check.message || "")
            + '</li>';
        }).join("");
        return '<div class="validateItem">'
          + '<div class="validateHead"><b>' + escapeHTML(item.label || item.kind) + '</b>'
          + '<span class="' + (item.ok ? "statusOk" : "statusBad") + '">' + escapeHTML(item.status || (item.ok ? "可用" : "不可用")) + '</span></div>'
          + '<ul class="validateChecks">' + checks + '</ul></div>';
      }).join("");
    }
    function mountConfigValidation(base, kind, cacheKey) {
      const host = $("configValidate");
      if (!host) return;
      const queryKind = kind === "clash" ? "clash" : kind;
      host.innerHTML = '<p class="note">正在检测配置可用性...</p>';
      fetch(base + "validate?kind=" + encodeURIComponent(queryKind) + "&v=" + cacheKey, { credentials: "same-origin" })
        .then((res) => {
          if (!res.ok) throw new Error("validate HTTP " + res.status);
          return res.json();
        })
        .then((result) => {
          const summary = result.ok
            ? '<p class="statusOk" style="margin:0 0 8px;">服务端当前配置检测通过，可以导入。</p>'
            : '<p class="statusBad" style="margin:0 0 8px;">服务端当前配置存在问题，请先修复后再导入。</p>';
          host.innerHTML = '<h3>配置可用性检测</h3>' + summary + renderValidationItems(result.items || []);
        })
        .catch(() => {
          host.innerHTML = '<p class="note">无法完成配置检测，请刷新后重试。</p>';
        });
    }
    function mountYamlPasteValidation(base, kind) {
      const btn = $("yamlValidateBtn");
      const input = $("yamlValidateInput");
      const result = $("yamlValidateResult");
      if (!btn || !input || !result) return;
      btn.onclick = async () => {
        const content = input.value.trim();
        if (!content) {
          setMsg("message", "请先粘贴 YAML 内容", true);
          return;
        }
        const validateKind = kind === "clash" ? "clash-android" : kind;
        btn.disabled = true;
        result.innerHTML = '<p class="note">检测中...</p>';
        try {
          const res = await fetch(base + "validate-yaml", {
            method: "POST",
            credentials: "same-origin",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ kind: validateKind, content })
          });
          const data = await res.json();
          if (!res.ok) throw new Error(data.error || ("HTTP " + res.status));
          const item = (data.items || [])[0];
          if (!item) {
            result.innerHTML = '<p class="note">未返回检测结果</p>';
            return;
          }
          const head = item.ok
            ? '<p class="statusOk" style="margin:0 0 8px;">这份 YAML 可以使用。</p>'
            : '<p class="statusBad" style="margin:0 0 8px;">这份 YAML 存在问题，请勿导入。</p>';
          result.innerHTML = head + renderValidationItems([item]);
        } catch (err) {
          result.innerHTML = '<p class="statusBad">检测失败：' + escapeHTML(err.message || String(err)) + '</p>';
        } finally {
          btn.disabled = false;
        }
      };
    }
    function manifestKindsForUI(kind) {
      if (kind === "clash") return ["clash-android", "clash"];
      return [kind];
    }
    function renderConfigManifest(manifest, kind) {
      if (!manifest || !Array.isArray(manifest.items)) return "";
      const wanted = new Set(manifestKindsForUI(kind));
      const items = manifest.items.filter((item) => wanted.has(item.kind));
      if (!items.length) return "";
      const rows = items.map((item) => {
        return '<tr>'
          + '<td>' + escapeHTML(item.label || item.kind) + '</td>'
          + '<td><code>' + escapeHTML(item.version || "-") + '</code></td>'
          + '<td><code>' + escapeHTML(item.checksum || "-") + '</code>'
          + ' <button type="button" data-copy-checksum="' + escapeHTML(item.checksum || "") + '">复制</button></td>'
          + '</tr>';
      }).join("");
      return '<div class="configMeta">'
        + '<h3>配置版本号 / 校验和</h3>'
        + '<p class="note">导入前请核对下载内容与下表一致。YAML/文本文件前两行含 <code># vpn-config-version</code> 与 <code># vpn-config-checksum</code>；小火箭 JSON 含 <code>config_version</code> / <code>config_checksum</code> 字段；sing-box 远程导入请对照本页版本号，或先下载 JSON 并查看响应头。</p>'
        + '<table><thead><tr><th>类型</th><th>版本号</th><th>校验和</th></tr></thead><tbody>' + rows + '</tbody></table>'
        + '<p class="note">模板修订 <code>' + escapeHTML(manifest.template_revision || "-") + '</code>'
        + ' · 运行时指纹 <code>' + escapeHTML(manifest.runtime_checksum || "-") + '</code>'
        + (manifest.user_updated_at ? ' · 账号更新 ' + escapeHTML(manifest.user_updated_at) : '')
        + '</p></div>';
    }
    function bindConfigManifestCopy(root) {
      if (!root) return;
      root.querySelectorAll("[data-copy-checksum]").forEach((button) => {
        button.addEventListener("click", async () => {
          const checksum = button.getAttribute("data-copy-checksum") || "";
          if (!checksum) return;
          let ok = false;
          if (navigator.clipboard && window.isSecureContext) {
            try {
              await navigator.clipboard.writeText(checksum);
              ok = true;
            } catch (err) {
              ok = false;
            }
          }
          if (ok) {
            const oldText = button.textContent;
            button.textContent = "已复制";
            setMsg("message", "校验和已复制", false);
            setTimeout(() => { button.textContent = oldText; }, 1600);
          } else {
            setMsg("message", "复制失败，请手动选择校验和", true);
          }
        });
      });
    }
    function mountConfigManifest(base, kind, cacheKey) {
      const host = $("configMeta");
      if (!host) return;
      host.innerHTML = '<p class="note">正在读取配置版本信息...</p>';
      fetch(base + "manifest?v=" + cacheKey, { credentials: "same-origin" })
        .then((res) => {
          if (!res.ok) throw new Error("manifest HTTP " + res.status);
          return res.json();
        })
        .then((manifest) => {
          host.innerHTML = renderConfigManifest(manifest, kind) || '<p class="note">暂无版本信息</p>';
          bindConfigManifestCopy(host);
        })
        .catch(() => {
          host.innerHTML = '<p class="note">无法读取配置版本信息，请刷新后重试。</p>';
        });
    }
    function openConfig(userId, kind) {
      const user = usersById.get(String(userId));
      if (!user) return;
      const labels = { vless: "扫码/通用链接", rocket: "clg 小火箭", clash: "Clash Meta", "sing-box": "sing-box" };
	      $("configTitle").textContent = labels[kind] + " 配置";
	      $("configSub").textContent = user.name + " / " + user.uuid;
	      const base = "/api/vpn/configs/" + user.id + "/";
	      const cacheKey = encodeURIComponent(user.updated_at || Date.now());
      let actions = "";
      let methods = "";
      let side = "";
      if (kind === "rocket") {
        actions = '<button id="copyRocketImport" class="primary">复制导入链接</button><a class="button" href="' + base + 'rocket">下载配置包 JSON</a>';
        methods = '<h3>clg 的小火箭导入</h3>'
          + '<ol class="methodList"><li>打开 clg 的小火箭 App，点击“扫码”。</li><li>扫描右侧二维码后，App 会自动下载并保存这个用户的配置。</li><li>回到 App 首页，选择该配置，点击“连接”。</li><li>首次连接时系统会弹出 VPN 权限；Android 13+ 还会请求通知权限。</li></ol>'
          + '<h3>无法扫码时</h3>'
          + '<ol class="methodList"><li>点击“复制导入链接”。</li><li>在 App 的配置入口粘贴链接后点击“导入”。</li><li>如果网络环境拦截远程拉取，可以下载配置包 JSON 后粘贴到 App。</li></ol>'
          + '<h3>导入链接</h3><textarea id="rocketImportText" readonly>正在读取导入链接...</textarea>';
	        side = '<div class="qrBox"><img id="configQr" class="loading" alt="clg 小火箭导入二维码"><p class="qrStatus">二维码加载中...</p><p class="note">二维码内容是 clg 的小火箭专用导入链接，包含短 token；用户停用后配置接口会拒绝访问。</p></div>';
      } else if (kind === "clash") {
        actions = '<a id="openClashImport" class="button primary" href="#">手机扫码/一键导入</a><a class="button" href="' + base + 'clash-android">下载 Android YAML</a><button id="copyClashImport">复制导入链接</button><a class="button" href="' + base + 'clash">下载电脑 YAML</a>';
        methods = '<h3>安卓手机（推荐）</h3>'
          + '<ol class="methodList"><li>先在 Clash Meta 里删除所有旧配置，并关闭 VPN。</li><li>优先扫描右侧二维码，或点击“手机扫码/一键导入”。</li><li>如果必须手动导入，请在手机浏览器直接打开本页，下载 <b>Android YAML</b>；不要用电脑下载后再转发到手机。</li><li>导入后确认节点名是 <code>' + escapeHTML(user.name) + '</code>，UUID 以 <code>' + escapeHTML(user.uuid.slice(0, 8)) + '</code> 开头。</li><li>Android 版使用<b>规则模式</b>：国内网站/App 直连，国外走代理。请勿在 Clash Meta 里手动切成全局模式。</li><li>启动 VPN 后，先测百度/微信是否正常，再测 Google 验证代理。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases/download/v2.11.28/cmfa-2.11.28-meta-arm64-v8a-release.apk">下载 Android ARM64 APK</a><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases/download/v2.11.28/cmfa-2.11.28-meta-universal-release.apk">下载 Android Universal APK</a><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases">更多 APK 版本</a></div>'
          + '<h3>电脑 Clash Verge</h3>'
          + '<ol class="methodList"><li>电脑端使用 Clash Verge Rev，先从发布页安装对应系统版本。</li><li>点击“下载电脑 YAML”。</li><li>打开 Clash Verge，进入“订阅/配置”。</li><li>选择从本地文件导入，选中刚下载的 YAML。</li><li>切换到该配置，并在代理组中选择本节点。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/clash-verge-rev/clash-verge-rev/releases">下载 Clash Verge Rev</a></div>'
          + '<h3>导入链接</h3><textarea id="clashImportText" readonly>正在读取导入链接...</textarea>'
          + '<div class="yamlValidateBox"><h3>检测本地 YAML</h3>'
          + '<p class="note">导入前可粘贴 YAML 内容（或下载后打开复制），点击检测确认节点、UUID、Reality 参数是否正确。</p>'
          + '<textarea id="yamlValidateInput" placeholder="粘贴 Clash YAML..."></textarea>'
          + '<div class="actions"><button type="button" id="yamlValidateBtn">检测 YAML 是否可用</button></div>'
          + '<div id="yamlValidateResult"></div></div>';
	        side = '<div class="qrBox"><img id="configQr" class="loading" alt="Clash Meta 导入二维码"><p class="qrStatus">二维码加载中...</p><p class="note">二维码会导入 Android 专用 Clash 配置（规则模式：国内直连，国外走代理）。不是 VLESS 分享链接。</p></div>';
      } else if (kind === "sing-box") {
        actions = '<a class="button primary" href="' + base + 'sing-box">下载 JSON 配置</a><button id="copySingBoxJson">复制 JSON 到剪贴板</button>';
        methods = '<div class="note" style="margin-bottom:12px;border:1px solid #f0ad4e;background:#fff8e6;padding:10px;"><strong>不要扫远程导入二维码，也不要选「远程配置」。</strong>扫码会去拉 <code>http://54.150.9.209/.../sing-box.json</code>，国内未连 VPN 前几乎必报 <code>connection reset by peer</code>。这是网络限制，不是账号坏了。</div>'
          + '<h3>Android 正确导入（本地配置）</h3>'
          + '<ol class="methodList"><li>在<b>电脑或手机浏览器</b>登录本管理页，点「下载 JSON 配置」。</li><li>用文本编辑器打开 JSON，确认 <code>uuid</code> 与上方清单一致（以 <code>' + escapeHTML(user.uuid.slice(0, 8)) + '</code> 开头）。</li><li>把 JSON 保存到手机（微信传文件 / 文件管理器 / 浏览器下载目录）。</li><li>sing-box → 配置文件 → 右上角 <b>+</b> → 类型选「<b>本地</b>」→ 从文件导入。</li><li><strong>关闭「自动更新」</strong>（本地配置不需要远程拉取）。</li><li>启用配置，允许 VPN 权限，先测 YouTube App。</li></ol>'
          + '<h3>或用剪贴板</h3>'
          + '<ol class="methodList"><li>点上方「复制 JSON 到剪贴板」（需在管理页操作）。</li><li>sing-box → + → 本地 → 粘贴/从剪贴板导入（若 App 支持）。</li></ol>'
          + '<h3>Android 安装</h3>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/SagerNet/sing-box/releases/download/v1.13.12/SFA-1.13.12-arm64-v8a.apk">下载 Android ARM64 APK</a><a class="button" target="_blank" href="https://github.com/SagerNet/sing-box/releases/download/v1.13.12/SFA-1.13.12-universal.apk">下载 Android Universal APK</a></div>'
          + '<h3>JSON 配置内容（可核对）</h3><textarea id="singBoxJsonText" readonly>正在读取 JSON 配置...</textarea>';
	        side = '<div class="qrBox"><p class="note" style="padding:16px;"><strong>Android 不提供远程导入二维码。</strong><br><br>请使用左侧「下载 JSON 配置」做本地导入。<br><br>若仍想用 Clash，请返回列表进入 Clash Meta 页面下载 Android YAML。</p></div>';
      } else {
        actions = '<button id="copyVless" class="primary">复制 VLESS 链接</button><a class="button" href="' + base + 'vless">下载链接文本</a>';
        methods = '<h3>适用客户端</h3>'
          + '<ol class="methodList"><li>这个页面适用于支持 VLESS Reality 分享链接的软件，例如 Hiddify、NekoBox、v2rayN、Shadowrocket 等。</li><li>手机端优先使用客户端内的扫码导入，直接扫描右侧二维码。</li><li>如果客户端没有“粘贴/从剪贴板导入”，不要复制链接，直接扫码或下载链接文本保存。</li><li>官方 sing-box 通常使用 JSON 配置，请返回列表进入 sing-box 配置页。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/hiddify/hiddify-app/releases">Hiddify Android 发布页</a><a class="button" target="_blank" href="https://github.com/MatsuriDayo/NekoBoxForAndroid/releases">NekoBox Android 发布页</a></div>'
          + '<h3>VLESS 链接内容</h3><textarea id="vlessText" readonly>正在读取链接...</textarea>';
	        side = '<div class="qrBox"><img id="configQr" class="loading" alt="VLESS 分享二维码"><p class="qrStatus">二维码加载中...</p><p class="note">二维码内容是通用 VLESS 分享链接。sing-box JSON 导入失败时，可以用 Hiddify 或 NekoBox 扫这个二维码验证账号是否可用。</p></div>';
      }
      $("configBody").innerHTML = '<div id="configMeta" class="configMeta"></div><div id="configValidate" class="configValidate"></div><div class="configLayout"><div><div class="actions" style="margin-bottom:14px;">' + actions + '</div>' + methods + '</div>' + side + '</div>';
      showView("config");
      mountConfigManifest(base, kind, cacheKey);
      mountConfigValidation(base, kind, cacheKey);
      if (kind === "clash") mountYamlPasteValidation(base, kind);
      const qrKinds = {
        rocket: "rocket-import-qr",
        clash: "clash-android-import-qr",
        vless: "qr"
      };
      if (qrKinds[kind]) {
        mountConfigQR("configQr", base + qrKinds[kind] + "?v=" + cacheKey);
      }
      if (kind === "vless") {
        fetch(base + "vless", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("vlessText").value = text;
          $("copyVless").onclick = () => copyText(text, "copyVless");
        });
      }
      if (kind === "sing-box") {
        fetch(base + "sing-box", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("singBoxJsonText").value = text;
          $("copySingBoxJson").onclick = () => copyTextFrom("singBoxJsonText", "copySingBoxJson", "sing-box JSON 已复制，请在 App 中创建「本地」配置并粘贴");
        });
      }
      if (kind === "rocket") {
        fetch(base + "rocket-import", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("rocketImportText").value = text;
          $("copyRocketImport").onclick = () => copyTextFrom("rocketImportText", "copyRocketImport", "clg 小火箭导入链接已复制到剪切板");
        });
      }
      if (kind === "clash") {
        fetch(base + "clash-android-import", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("clashImportText").value = text;
          $("openClashImport").href = text;
          $("copyClashImport").onclick = () => copyTextFrom("clashImportText", "copyClashImport", "Clash Meta 导入链接已复制到剪切板");
        });
      }
    }
    async function copyText(text, buttonId, textareaId) {
      let ok = false;
      if (navigator.clipboard && window.isSecureContext) {
        try {
          await navigator.clipboard.writeText(text);
          ok = true;
        } catch (err) {
          ok = false;
        }
      }
      if (!ok) {
        const area = $(textareaId || "vlessText");
        area.focus();
        area.select();
        area.setSelectionRange(0, area.value.length);
        ok = document.execCommand("copy");
        window.getSelection().removeAllRanges();
      }
      if (ok) {
        const button = $(buttonId);
        const oldText = button.textContent;
        button.textContent = "已复制";
        setMsg("message", "VLESS 链接已复制到剪切板", false);
        setTimeout(() => { button.textContent = oldText; }, 1600);
      } else {
        setMsg("message", "自动复制失败，请手动选中文本复制", true);
      }
    }
    async function copyTextFrom(textareaId, buttonId, successText) {
      await copyText($(textareaId).value, buttonId, textareaId);
      if (successText) setMsg("message", successText, false);
    }
    $("loginBtn").onclick = async () => {
      try {
        const data = await api("/api/auth/login", {
          method: "POST",
          body: JSON.stringify({ email: $("email").value.trim(), password: $("password").value.trim() })
        });
        setMsg("loginMsg", "登录成功", false);
        showApp(data.user);
      } catch (err) {
        setMsg("loginMsg", "登录失败，请检查账号密码", true);
      }
    };
    $("password").addEventListener("keydown", (event) => {
      if (event.key === "Enter") $("loginBtn").click();
    });
    $("togglePassword").onclick = () => {
      const input = $("password");
      const showing = input.type === "text";
      input.type = showing ? "password" : "text";
      $("togglePassword").textContent = showing ? "显示" : "隐藏";
    };
    $("logout").onclick = async () => {
      await api("/api/auth/logout", { method: "POST", body: "{}" }).catch(() => {});
      showLogin();
    };
    $("refresh").onclick = load;
    $("createOpen").onclick = () => showView("create");
    $("createBack").onclick = () => showView("dashboard");
    $("createCancel").onclick = () => showView("dashboard");
    $("configBack").onclick = () => showView("dashboard");
    document.querySelectorAll("[data-bucket]").forEach((button) => {
      button.onclick = async () => {
        currentBucket = button.dataset.bucket;
        document.querySelectorAll("[data-bucket]").forEach((b) => b.classList.toggle("active", b === button));
        const history = await api("/api/vpn/traffic?bucket=" + encodeURIComponent(currentBucket));
        renderTraffic(history);
      };
    });
    $("createUser").onclick = async () => {
      try {
        await api("/api/vpn/users", {
          method: "POST",
          body: JSON.stringify({
            name: $("newName").value,
            login_email: $("newEmail").value,
            password: $("newPassword").value,
            role: $("newRole").value
          })
        });
        $("newName").value = "";
        $("newEmail").value = "";
        $("newPassword").value = "";
        showView("dashboard");
        await load();
      } catch (err) {
        setMsg("message", err.message, true);
      }
    };
    $("apply").onclick = async () => {
      try {
        await api("/api/vpn/apply", { method: "POST", body: "{}" });
        setMsg("message", "配置已应用", false);
      } catch (err) {
        setMsg("message", err.message, true);
      }
    };
    $("editCancel").onclick = () => {
      editingId = null;
      $("editModal").classList.add("hidden");
    };
    $("deviceModalClose").onclick = () => {
      managingDevicesUserId = null;
      $("deviceModal").classList.add("hidden");
    };
    $("editSave").onclick = async () => {
      if (!editingId) return;
      const body = {
        name: $("editName").value,
        login_email: $("editEmail").value,
        role: $("editRole").value
      };
      if ($("editPassword").value) body.password = $("editPassword").value;
      try {
        await api("/api/vpn/users/" + editingId, { method: "PATCH", body: JSON.stringify(body) });
        editingId = null;
        $("editModal").classList.add("hidden");
        $("editPassword").value = "";
        await load();
      } catch (err) {
        setMsg("message", err.message, true);
      }
    };
    document.addEventListener("click", async (event) => {
      const configKind = event.target.dataset.config;
      const configUser = event.target.dataset.user;
      if (configKind && configUser) {
        openConfig(configUser, configKind);
        return;
      }
      const devicesUser = event.target.dataset.devices;
      if (devicesUser) {
        const user = usersById.get(String(devicesUser));
        if (!user) return;
        renderDeviceModal(user);
        $("deviceModal").classList.remove("hidden");
        return;
      }
      const saveDeviceUser = event.target.dataset.saveDevice;
      const saveDeviceIP = event.target.dataset.deviceIp;
      if (saveDeviceUser && saveDeviceIP) {
        const displayName = document.querySelector('[data-device-name="' + saveDeviceIP + '"]');
        const deviceType = document.querySelector('[data-device-type="' + saveDeviceIP + '"]');
        try {
          await api("/api/vpn/devices/" + saveDeviceUser + "/" + encodeURIComponent(saveDeviceIP), {
            method: "PATCH",
            body: JSON.stringify({
              display_name: displayName ? displayName.value : "",
              device_type: deviceType ? deviceType.value : "unknown"
            })
          });
          setMsg("message", "设备标注已保存", false);
          await load();
          const user = usersById.get(String(saveDeviceUser));
          if (user) renderDeviceModal(user);
        } catch (err) {
          setMsg("message", err.message, true);
        }
        return;
      }
      const edit = event.target.dataset.edit;
      if (edit) {
        const user = usersById.get(String(edit));
        if (!user) return;
        editingId = edit;
        $("editName").value = user.name || "";
        $("editEmail").value = user.login_email || "";
        $("editPassword").value = "";
        $("editRole").value = user.role === "admin" ? "admin" : "user";
        $("editModal").classList.remove("hidden");
      }
      const del = event.target.dataset.delete;
      if (del && confirm("确认删除这个人员？")) {
        await api("/api/vpn/users/" + del, { method: "DELETE" });
        await load();
      }
      const toggle = event.target.dataset.toggle;
      if (toggle) {
        const enabled = event.target.dataset.enabled !== "true";
        await api("/api/vpn/users/" + toggle, { method: "PATCH", body: JSON.stringify({ enabled: enabled }) });
        await load();
      }
    });
    window.addEventListener("resize", () => {
      if (lastStatus) api("/api/vpn/traffic?bucket=" + encodeURIComponent(currentBucket)).then(renderTraffic).catch(() => {});
    });
    api("/api/auth/me").then((data) => showApp(data.user)).catch(showLogin);
  </script>
</body>
</html>`
