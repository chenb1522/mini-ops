// mini-ops 前端逻辑

let statsTimer = null;

async function api(path, opts = {}) {
  const resp = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  if (resp.status === 401) {
    location.href = '/login';
    throw new Error('unauthorized');
  }
  return resp.json();
}

function showToast(msg, type = 'ok') {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast show ' + type;
  clearTimeout(t._timer);
  t._timer = setTimeout(() => t.classList.remove('show'), 4000);
}

function fmtBytes(b) {
  if (!b || b === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(1024));
  return (b / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
}

// ========== 系统信息 ==========
async function loadSystem() {
  const data = await api('/api/system');
  document.getElementById('hostname').textContent = data.hostname;
  document.getElementById('uptime-val').textContent = data.uptime;

  const items = [
    ['操作系统', data.distro || data.os],
    ['内核版本', data.kernel],
    ['架构', data.arch],
    ['CPU 型号', data.cpu_model || '未知'],
    ['CPU 核心数', data.cpu_cores + ' 核'],
    ['内存总量', fmtBytes(data.memory_total)],
    ['负载 (1/5/15min)', data.load_avg],
    ['本地时间', data.time],
  ];

  const netText = (data.networks || []).map(n => `${n.name}: ${n.ip}`).join('，') || '无';
  items.push(['网络接口', netText]);

  const grid = document.getElementById('system-info');
  grid.innerHTML = items.map(([k, v]) => `
    <div class="info-item">
      <div class="info-label">${k}</div>
      <div class="info-value">${v}</div>
    </div>`).join('');
}

// ========== 实时统计 ==========
async function loadStats() {
  const data = await api('/api/stats');
  const cpu = data.cpu_percent.toFixed(1);
  const mem = data.mem_percent.toFixed(1);

  // 概览指标卡
  document.getElementById('cpu-val').textContent = cpu;
  document.getElementById('mem-val').textContent = mem;

  // 进度条
  const cpuBar = document.getElementById('cpu-bar');
  const memBar = document.getElementById('mem-bar');
  cpuBar.style.width = cpu + '%';
  memBar.style.width = mem + '%';
  cpuBar.className = 'p-fill cpu' + (cpu > 80 ? ' high' : '');
  memBar.className = 'p-fill mem' + (mem > 80 ? ' high' : '');
  document.getElementById('cpu-pct').textContent = cpu + '%';
  document.getElementById('mem-pct').textContent = mem + '%';

  // 详情
  document.getElementById('stat-detail').innerHTML = [
    `内存：已用 ${fmtBytes(data.mem_used)} / 总量 ${fmtBytes(data.mem_total)}（可用 ${fmtBytes(data.mem_available)}）`,
    data.swap_total > 0 ? `Swap：已用 ${fmtBytes(data.swap_used)} / 总量 ${fmtBytes(data.swap_total)}` : '',
    `进程数：${data.process_count}`,
    `本工具自身占用：${fmtBytes(data.self_mem_kb * 1024)}`,
  ].filter(Boolean).join('<br>');
}

// ========== 磁盘 ==========
async function loadDisks() {
  const data = await api('/api/system');
  const disks = data.disks || [];
  renderDisks(disks);

  // 概览卡片：磁盘已用（所有挂载点总量）
  let total = 0, used = 0;
  disks.forEach(d => { total += d.total; used += d.used; });
  if (total > 0) {
    document.getElementById('disk-val').textContent = fmtBytes(used) + ' / ' + fmtBytes(total);
  }
}

function renderDisks(disks) {
  const tbody = document.querySelector('#disk-table tbody');
  if (!disks.length) {
    tbody.innerHTML = '<tr><td colspan="5" style="color:#9ca3af;text-align:center;padding:24px;">无数据</td></tr>';
    return;
  }
  tbody.innerHTML = disks.map(d => `
    <tr>
      <td style="font-weight:600;">${d.mount}</td>
      <td>${fmtBytes(d.total)}</td>
      <td>${fmtBytes(d.used)}</td>
      <td>${fmtBytes(d.free)}</td>
      <td>
        <div style="display:flex;align-items:center;gap:10px;">
          <div class="p-track" style="flex:1;height:8px;">
            <div class="p-fill" style="width:${d.percent};height:8px;background:${parseInt(d.percent) > 85 ? 'linear-gradient(90deg,#f5576c,#fb7185)' : 'linear-gradient(90deg,#f59e0b,#fbbf24)'};"></div>
          </div>
          <span style="font-size:12px;font-weight:600;width:42px;text-align:right;font-variant-numeric:tabular-nums;">${d.percent}</span>
        </div>
      </td>
    </tr>`).join('');
}

// ========== Nginx ==========
async function loadNginxStatus() {
  const data = await api('/api/nginx/status');
  const el = document.getElementById('nginx-status');
  document.getElementById('config-path').textContent = data.config_path;

  if (data.installed) {
    el.className = 'badge ok';
    el.innerHTML = '<span class="status-dot"></span>nginx 已安装（' + data.pkg_manager + '）';
    document.getElementById('btn-install').disabled = true;
    document.getElementById('btn-restart').disabled = false;
    document.getElementById('btn-test').disabled = false;
  } else {
    el.className = 'badge missing';
    el.innerHTML = '<span class="status-dot"></span>nginx 未安装';
    document.getElementById('btn-install').disabled = false;
    document.getElementById('btn-restart').disabled = true;
    document.getElementById('btn-test').disabled = true;
  }
}

async function installNginx() {
  const btn = document.getElementById('btn-install');
  btn.disabled = true;
  btn.textContent = '安装中...';
  showToast('正在安装 nginx，可能需要几分钟...', 'ok');
  try {
    const data = await api('/api/nginx/install', { method: 'POST', body: '{}' });
    showToast(data.message, data.ok ? 'ok' : 'err');
  } catch (e) {}
  btn.textContent = '安装 Nginx';
  loadNginxStatus();
}

async function restartNginx() {
  const data = await api('/api/nginx/restart', { method: 'POST', body: '{}' });
  showToast(data.message, data.ok ? 'ok' : 'err');
}

async function testNginx() {
  const data = await api('/api/nginx/test', { method: 'POST', body: '{}' });
  showToast(data.message, data.ok ? 'ok' : 'err');
}

async function loadConfig() {
  const data = await api('/api/nginx/config');
  document.getElementById('config-editor').value = data.content || '';
}

async function saveConfig() {
  const content = document.getElementById('config-editor').value;
  const data = await api('/api/nginx/config', {
    method: 'POST',
    body: JSON.stringify({ content }),
  });
  showToast(data.message, data.ok ? 'ok' : 'err');
}

// ========== 时钟 ==========
function tick() {
  const now = new Date();
  document.getElementById('clock').textContent = now.toLocaleString('zh-CN');
}

// ========== 初始化 ==========
async function init() {
  tick();
  setInterval(tick, 1000);
  loadSystem();
  loadStats();
  loadDisks();
  loadNginxStatus();
  loadConfig();
  statsTimer = setInterval(loadStats, 2000);
  setInterval(loadDisks, 10000);
}

init();
