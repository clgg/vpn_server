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
    .configLayout { display: grid; grid-template-columns: minmax(0, 1fr) 280px; gap: 16px; align-items: start; }
    .methodList { margin: 0; padding-left: 20px; color: var(--text); line-height: 1.75; }
    .qrBox {
      display: grid;
      place-items: center;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel2);
      padding: 14px;
    }
    .qrBox img { width: 220px; height: 220px; image-rendering: pixelated; }
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
                <th>配置入口</th>
                <th id="opsHead">操作</th>
              </tr>
            </thead>
            <tbody id="users"></tbody>
          </table>
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
          api("/api/vpn/status"),
          api("/api/vpn/users"),
          api("/api/vpn/traffic?bucket=" + encodeURIComponent(currentBucket))
        ]);
        lastStatus = results[0];
        renderStatus(results[0]);
        renderUsers(results[1]);
        renderTraffic(results[2]);
        setMsg("message", "已刷新", false);
      } catch (err) {
        if (String(err.message).includes("401")) showLogin();
        setMsg("message", err.message, true);
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
    function renderUsers(list) {
      const isAdmin = currentUser && currentUser.role === "admin";
      usersById = new Map(list.users.map((u) => [String(u.id), u]));
      $("users").innerHTML = list.users.map((u) => {
        const ops = isAdmin
          ? '<td class="actions"><button data-edit="' + u.id + '">编辑</button><button data-toggle="' + u.id + '" data-enabled="' + u.enabled + '">' + (u.enabled ? "停用" : "启用") + '</button><button class="danger" data-delete="' + u.id + '">删除</button></td>'
          : '<td class="hidden"></td>';
        return '<tr>'
          + '<td><b>' + escapeHTML(u.name) + '</b><br><span class="muted">' + new Date(u.created_at).toLocaleString() + '</span></td>'
          + '<td>' + escapeHTML(u.login_email || "-") + '<br><span class="pill">' + (u.role === "admin" ? "管理员" : "普通用户") + '</span></td>'
          + '<td><code>' + escapeHTML(u.uuid) + '</code></td>'
          + '<td>' + (u.enabled ? '<span class="statusOk">启用</span>' : '<span class="statusBad">停用</span>') + '</td>'
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
	        side = '<div class="qrBox"><img alt="clg 小火箭导入二维码" src="' + base + 'rocket-import-qr?v=' + cacheKey + '"><p class="note">二维码内容是 clg 的小火箭专用导入链接，包含短 token；用户停用后配置接口会拒绝访问。</p></div>';
      } else if (kind === "clash") {
        actions = '<a id="openClashImport" class="button primary" href="#">一键导入 Clash Meta</a><button id="copyClashImport">复制导入链接</button><a class="button" href="' + base + 'clash">下载 Clash YAML</a>';
        methods = '<h3>安卓手机安装</h3>'
          + '<ol class="methodList"><li>优先下载 ARM64 APK；如果不确定手机架构，下载 Universal APK。</li><li>安装时如提示未知来源，需要在系统设置中允许本浏览器安装应用。</li><li>安装后回到本页，点击“一键导入 Clash Meta”或扫描右侧二维码导入配置。</li><li>进入 Clash Meta 后启动 VPN，并确认当前配置已选中。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases/download/v2.11.28/cmfa-2.11.28-meta-arm64-v8a-release.apk">下载 Android ARM64 APK</a><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases/download/v2.11.28/cmfa-2.11.28-meta-universal-release.apk">下载 Android Universal APK</a><a class="button" target="_blank" href="https://github.com/MetaCubeX/ClashMetaForAndroid/releases">更多 APK 版本</a></div>'
          + '<h3>电脑 Clash Verge</h3>'
          + '<ol class="methodList"><li>电脑端使用 Clash Verge Rev，先从发布页安装对应系统版本。</li><li>点击“下载 Clash YAML”。</li><li>打开 Clash Verge，进入“订阅/配置”。</li><li>选择从本地文件导入，选中刚下载的 YAML。</li><li>切换到该配置，并在代理组中选择本节点。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/clash-verge-rev/clash-verge-rev/releases">下载 Clash Verge Rev</a></div>'
          + '<h3>导入链接</h3><textarea id="clashImportText" readonly>正在读取导入链接...</textarea>';
	        side = '<div class="qrBox"><img alt="Clash Meta 导入二维码" src="' + base + 'clash-import-qr?v=' + cacheKey + '"><p class="note">二维码内容是 Clash Meta 配置导入链接，不是 VLESS 分享链接。</p></div>';
      } else if (kind === "sing-box") {
        actions = '<a id="openSingBoxImport" class="button primary" href="#">一键导入 sing-box</a><button id="copySingBoxImport">复制导入链接</button><a class="button" href="' + base + 'sing-box">下载 JSON 配置</a>';
        methods = '<h3>推荐导入方式</h3>'
          + '<ol class="methodList"><li>手机端优先扫描右侧二维码，这是 sing-box 官方远程配置导入链接。</li><li>如果无法唤起 App，点击“一键导入 sing-box”或复制导入链接到浏览器打开。</li><li>远程配置的好处是以后服务端配置更新后，客户端可以重新拉取配置。</li><li>导入后启用该配置，确认出口 IP 变为服务器所在地区。</li></ol>'
          + '<h3>Android 安装</h3>'
          + '<ol class="methodList"><li>Android 5.0+ 可用；新手机优先下载 ARM64 APK。</li><li>如果不确定手机架构，下载 Universal APK。</li><li>安装时如提示未知来源，需要在系统设置中允许本浏览器安装应用。</li><li>安装完成后回到本页，扫描右侧二维码或点击“一键导入 sing-box”。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/SagerNet/sing-box/releases/download/v1.13.12/SFA-1.13.12-arm64-v8a.apk">下载 Android ARM64 APK</a><a class="button" target="_blank" href="https://github.com/SagerNet/sing-box/releases/download/v1.13.12/SFA-1.13.12-universal.apk">下载 Android Universal APK</a><a class="button" target="_blank" href="https://github.com/SagerNet/sing-box/releases">更多 APK 版本</a></div>'
          + '<h3>iPhone / iPad / Mac</h3>'
          + '<ol class="methodList"><li>优先从 App Store 或 TestFlight 安装 sing-box Apple 平台客户端。</li><li>安装完成后，用系统相机或 sing-box 内的导入功能扫描右侧二维码。</li><li>如果扫描后没有自动跳转，复制导入链接后用 Safari 打开。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://sing-box.sagernet.org/clients/">打开官方客户端页面</a><a class="button" target="_blank" href="https://sing-box.sagernet.org/clients/android/">Android 官方说明</a></div>'
          + '<h3>桌面 / 手动导入</h3>'
          + '<ol class="methodList"><li>如果客户端不支持远程导入，点击“下载 JSON 配置”。</li><li>在 sing-box 客户端中新建本地配置，导入下载的 JSON 文件。</li><li>保存后启用配置；如果无法连接，先确认系统 VPN 权限已允许。</li></ol>'
          + '<h3>导入链接</h3><textarea id="singBoxImportText" readonly>正在读取导入链接...</textarea>';
	        side = '<div class="qrBox"><img alt="sing-box 导入二维码" src="' + base + 'sing-box-import-qr?v=' + cacheKey + '"><p class="note">二维码内容是 sing-box 远程配置导入链接，不是 VLESS 分享链接。</p></div>';
      } else {
        actions = '<button id="copyVless" class="primary">复制 VLESS 链接</button><a class="button" href="' + base + 'vless">下载链接文本</a>';
        methods = '<h3>适用客户端</h3>'
          + '<ol class="methodList"><li>这个页面适用于支持 VLESS Reality 分享链接的软件，例如 Hiddify、NekoBox、v2rayN、Shadowrocket 等。</li><li>手机端优先使用客户端内的扫码导入，直接扫描右侧二维码。</li><li>如果客户端没有“粘贴/从剪贴板导入”，不要复制链接，直接扫码或下载链接文本保存。</li><li>官方 sing-box 通常使用 JSON 配置，请返回列表进入 sing-box 配置页。</li></ol>'
          + '<div class="actions" style="margin:12px 0 18px;"><a class="button" target="_blank" href="https://github.com/hiddify/hiddify-app/releases">Hiddify Android 发布页</a><a class="button" target="_blank" href="https://github.com/MatsuriDayo/NekoBoxForAndroid/releases">NekoBox Android 发布页</a></div>'
          + '<h3>VLESS 链接内容</h3><textarea id="vlessText" readonly>正在读取链接...</textarea>';
	        side = '<div class="qrBox"><img alt="VLESS 分享二维码" src="' + base + 'qr?v=' + cacheKey + '"><p class="note">二维码内容是通用 VLESS 分享链接。sing-box JSON 导入失败时，可以用 Hiddify 或 NekoBox 扫这个二维码验证账号是否可用。</p></div>';
      }
      $("configBody").innerHTML = '<div class="configLayout"><div><div class="actions" style="margin-bottom:14px;">' + actions + '</div>' + methods + '</div>' + side + '</div>';
      showView("config");
      if (kind === "vless") {
        fetch(base + "vless", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("vlessText").value = text;
          $("copyVless").onclick = () => copyText(text, "copyVless");
        });
      }
      if (kind === "sing-box") {
        fetch(base + "sing-box-import", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("singBoxImportText").value = text;
          $("openSingBoxImport").href = text;
          $("copySingBoxImport").onclick = () => copyTextFrom("singBoxImportText", "copySingBoxImport", "sing-box 导入链接已复制到剪切板");
        });
      }
      if (kind === "rocket") {
        fetch(base + "rocket-import", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
          $("rocketImportText").value = text;
          $("copyRocketImport").onclick = () => copyTextFrom("rocketImportText", "copyRocketImport", "clg 小火箭导入链接已复制到剪切板");
        });
      }
      if (kind === "clash") {
        fetch(base + "clash-import", { credentials: "same-origin" }).then((res) => res.text()).then((text) => {
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
