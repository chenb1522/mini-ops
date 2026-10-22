// mini-ops 前端逻辑

let statsTimer = null;

async function api(path, opts = {}) {
  const resp = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  if (resp.status === 401) {
    showToast('未授权，请刷新页面重新登录', 'err');
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
  if (b === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(1024));
  return (b / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
}

// ========== 系统信息 ==========
async function loadSystem() {
  const data = await api('/api/system');
  document.getElementById('hostname').textContent = data.hostname;

  const items = [
    ['操作系统', data.distro || data.os],
    ['内核版本', data.kernel],
    ['架构', data.arch],
    ['CPU 型号', data.cpu_model || '未知'],
    ['CPU 核心数', data.cpu_cores + ' 核'],
    ['内存总量', fmtBytes(data.memory_total)],
    ['运行时间', data.uptime],
    ['负载 (1/5/15min)', data.load_avg],
    ['本地时间', data.time],
  ];

  const netText = (data.networks || []).map(n => `${n.name}: ${n.ip}`).join('，') || '无';
  items.push(['网络接口', netText]);

  const grid = document.getElementById('system-info');
  grid.innerHTML = items.map(([k, v]) => `
    <div class="info-item">
      <span class="info-label">${k}</span>
      <span class="info-value">${v}</span>
    </div>`).join('');
}

// ========== 实时统计 ==========
async function loadStats() {
  const data = await api('/api/stats');
  const cpu = data.cpu_percent.toFixed(1);
  const mem = data.mem_percent.toFixed(1);

  document.getElementById('cpu-val').textContent = cpu + '%';
  document.getElementById('mem-val').textContent = mem + '%';

  const cpuBar = document.getElementById('cpu-bar');
  const memBar = document.getElementById('mem-bar');
  cpuBar.style.width = cpu + '%';
  memBar.style.width = mem + '%';
  cpuBar.className = 'bar-fill bar-cpu' + (cpu > 80 ? ' bar-high' : '');
  memBar.className = 'bar-fill bar-mem' + (mem > 80 ? ' bar-high' : '');

  document.getElementById('stat-detail').innerHTML = [
    `内存：已用 ${fmtBytes(data.mem_used)} / 总量 ${fmtBytes(data.mem_total)}（可用 ${fmtBytes(data.mem_available)}）`,
    data.swap_total > 0 ? `Swap：已用 ${fmtBytes(data.swap_used)} / 总量 ${fmtBytes(data.swap_total)}` : '',
    `进程数：${data.process_count}`,
    `本工具自身占用：${fmtBytes(data.self_mem_kb * 1024)}`,
  ].filter(Boolean).join('<br>');

  // 磁盘表
  const disks = await api('/api/system').then(d => d.disks);
  renderDisks(disks);
}

function renderDisks(disks) {
  const tbody = document.querySelector('#disk-table tbody');
  tbody.innerHTML = (disks || []).map(d => `
    <tr>
      <td>${d.mount}</td>
      <td>${fmtBytes(d.total)}</td>
      <td>${fmtBytes(d.used)}</td>
      <td>${fmtBytes(d.free)}</td>
      <td>${d.percent}</td>
    </tr>`).join('') || '<tr><td colspan="5" style="color:#9ca3af">无数据</td></tr>';
}

// ========== Nginx ==========
async function loadNginxStatus() {
  const data = await api('/api/nginx/status');
  const el = document.getElementById('nginx-status');
  document.getElementById('config-path').textContent = data.config_path;

  if (data.installed) {
    el.className = 'nginx-status ok';
    el.textContent = `✅ nginx 已安装（包管理器：${data.pkg_manager}）`;
    document.getElementById('btn-install').disabled = true;
    document.getElementById('btn-restart').disabled = false;
    document.getElementById('btn-test').disabled = false;
  } else {
    el.className = 'nginx-status missing';
    el.textContent = '⚠️ nginx 未安装';
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
  loadNginxStatus();
  loadConfig();
  statsTimer = setInterval(loadStats, 2000);
}

init();
