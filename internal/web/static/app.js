async function refresh() {
  try {
    const res = await fetch('/api/dashboard');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const data = await res.json();
    render(data);
  } catch (e) {
    document.getElementById('err').textContent = '加载失败: ' + e.message;
  }
}

function render(d) {
  const cards = document.getElementById('cards');
  const defs = [
    ['进行中批次', d.open_batches],
    ['待处理原料批', d.open_lots],
    ['复检挂起批', d.held_batches],
    ['超期巡检', d.due_inspections],
    ['待保养设备', d.service_due_equipment]
  ];
  cards.innerHTML = '';
  for (const [label, value] of defs) {
    const el = document.createElement('div');
    el.className = 'card';
    el.innerHTML = '<div class="label">' + label + '</div><div class="value">' + value + '</div>';
    cards.appendChild(el);
  }
  const latest = document.getElementById('latest');
  latest.innerHTML = '<strong>各线最新含水率</strong><br>';
  const entries = d.latest_by_station || {};
  for (const key of Object.keys(entries)) {
    const span = document.createElement('span');
    span.textContent = key + ': ' + Number(entries[key]).toFixed(2) + '%';
    latest.appendChild(span);
  }
}

refresh();
setInterval(refresh, 10000);
