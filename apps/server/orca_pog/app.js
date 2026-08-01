const DATA_FILE = 'matilda_bay_pod_supply.cdv';
const TARGET_DATE = '2026-07-04';

const DISPLAY_FIELDS = [
  'pod_name',
  'population',
  'distance_from_hub_km',
  'peacock_disruption',
  'overall_status',
];

const statusEl = document.getElementById('status');
const tbody = document.getElementById('pod-tbody');

function setStatus(message, isError) {
  statusEl.textContent = message;
  statusEl.classList.toggle('error', Boolean(isError));
  statusEl.classList.remove('loading');
}

function parseCsv(text) {
  const rows = [];
  let field = '';
  let row = [];
  let inQuotes = false;

  for (let i = 0; i < text.length; i += 1) {
    const char = text[i];

    if (inQuotes) {
      if (char === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 1;
        } else {
          inQuotes = false;
        }
      } else {
        field += char;
      }
    } else if (char === '"') {
      inQuotes = true;
    } else if (char === ',') {
      row.push(field);
      field = '';
    } else if (char === '\n') {
      row.push(field);
      if (row.some((cell) => cell.trim() !== '')) {
        rows.push(row);
      }
      row = [];
      field = '';
    } else if (char !== '\r') {
      field += char;
    }
  }

  row.push(field);
  if (row.some((cell) => cell.trim() !== '')) {
    rows.push(row);
  }

  return rows;
}

function csvToObjects(rows) {
  if (rows.length === 0) return [];

  const header = rows[0].map((cell) => cell.trim());
  return rows.slice(1).map((cells) => {
    const obj = {};
    header.forEach((name, index) => {
      obj[name] = (cells[index] ?? '').trim();
    });
    return obj;
  });
}

function buildRow(pod) {
  const tr = document.createElement('tr');

  DISPLAY_FIELDS.forEach((field) => {
    const td = document.createElement('td');

    if (field === 'overall_status') {
      const pill = document.createElement('span');
      pill.className = `status-pill ${pod[field] || 'unknown'}`;
      pill.textContent = pod[field] || 'unknown';
      td.appendChild(pill);
    } else if (field === 'population') {
      td.textContent = Number(pod[field]) || 0;
    } else if (field === 'distance_from_hub_km') {
      td.textContent = `${Number(pod[field]) || 0} km`;
    } else {
      td.textContent = pod[field] || '';
    }

    tr.appendChild(td);
  });

  return tr;
}

function renderPods(pods) {
  tbody.replaceChildren();

  if (pods.length === 0) {
    setStatus(`No pod reports found for ${TARGET_DATE}.`);
    return;
  }

  pods.forEach((pod) => tbody.appendChild(buildRow(pod)));
  setStatus(`Showing ${pods.length} pod${pods.length === 1 ? '' : 's'} for ${TARGET_DATE}.`);
}

async function loadPods() {
  statusEl.classList.add('loading');
  setStatus('Loading pod supply report…');

  try {
    const response = await fetch(DATA_FILE);
    if (!response.ok) {
      throw new Error(`Could not load ${DATA_FILE} (HTTP ${response.status}).`);
    }

    const text = await response.text();
    const records = csvToObjects(parseCsv(text));
    const pods = records.filter((record) => record.report_date === TARGET_DATE);

    renderPods(pods);
  } catch (error) {
    setStatus(error instanceof Error ? error.message : 'Could not load pod report.', true);
  }
}

loadPods();
