// ======= Data layer (API calls) =======
const api = {
	async getHabits() {
		const response = await fetch('/api/habits');
		return await response.json();
	},
	async createHabit(habit) {
		const response = await fetch('/api/habits', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(habit)
		});
		return await response.json();
	},
	async updateHabit(habit) {
		const response = await fetch(`/api/habits/${habit.id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(habit)
		});
		return await response.json();
	},
	async deleteHabit(id) {
		await fetch(`/api/habits/${id}`, { method: 'DELETE' });
	},
	async getLogs() {
		const response = await fetch('/api/logs');
		return await response.json();
	},
	async createLog(log) {
		const response = await fetch('/api/logs', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(log)
		});
		return await response.json();
	},
	async updateLog(log) {
		const response = await fetch(`/api/logs/${log.id}`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(log)
		});
		return await response.json();
	},
	async deleteLog(id) {
		await fetch(`/api/logs/${id}`, { method: 'DELETE' });
	}
};

function uid() { return Math.random().toString(36).slice(2, 10) }
function todayStr() { return getCurrentDateTimeLocal().slice(0, 10) } // Get just the date part
function fmt(n) { return Number(n).toLocaleString(undefined, { maximumFractionDigits: 2 }) }

// ======= State =======
let state = { habits: [], logs: [] };
let activeHabitId = null;

// ======= Local Storage for Habit Selection =======
const STORAGE_KEY = 'epoch-selected-habit';

function saveSelectedHabit(habitId) {
	if (habitId) {
		localStorage.setItem(STORAGE_KEY, habitId);
	} else {
		localStorage.removeItem(STORAGE_KEY);
	}
}

function loadSelectedHabit() {
	return localStorage.getItem(STORAGE_KEY);
}

function clearSelectedHabit() {
	localStorage.removeItem(STORAGE_KEY);
}

// Load data from API
async function loadData() {
	try {
		state.habits = await api.getHabits();
		state.logs = await api.getLogs();

		// Try to restore previously selected habit
		const savedHabitId = loadSelectedHabit();
		const savedHabitExists = savedHabitId && state.habits.find(h => h.id === savedHabitId);

		if (savedHabitExists) {
			activeHabitId = savedHabitId;
		} else {
			// Fall back to first habit or null
			activeHabitId = state.habits[0]?.id || null;
			// Save the new selection (or clear if no habits)
			saveSelectedHabit(activeHabitId);
		}

		renderAll();
	} catch (error) {
		console.error('Failed to load data:', error);
	}
}

// ======= UI References =======
const habitsList = document.getElementById('habitsList');
const noHabits = document.getElementById('noHabits');
const logsTable = document.getElementById('logsTable');
const noLogs = document.getElementById('noLogs');
const activeHabitTitle = document.getElementById('activeHabitTitle');
const activeHabitMeta = document.getElementById('activeHabitMeta');
const todayProgress = document.getElementById('todayProgress');
const todaySummary = document.getElementById('todaySummary');
const barChart = document.getElementById('barChart');
const lineChart = document.getElementById('lineChart');

const habitDialog = document.getElementById('habitDialog');
const habitName = document.getElementById('habitName');
const habitUnit = document.getElementById('habitUnit');
const habitGoal = document.getElementById('habitGoal');

const logDialog = document.getElementById('logDialog');
const logHabit = document.getElementById('logHabit');
const logQty = document.getElementById('logQty');
const logDate = document.getElementById('logDate');

// ======= Helpers =======
function formatDateForDisplay(isoDate, displayDate) {
	if (displayDate) return displayDate;

	const date = new Date(isoDate);
	return date.toLocaleDateString('en-US', {
		month: 'short',
		day: 'numeric',
		year: 'numeric',
		hour: 'numeric',
		minute: '2-digit',
		hour12: true
	});
}

function formatDateForForm(isoDate) {
	const date = new Date(isoDate);
	// Convert to local timezone for display in form
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	const hours = String(date.getHours()).padStart(2, '0');
	const minutes = String(date.getMinutes()).padStart(2, '0');
	return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function getCurrentDateTimeLocal() {
	const now = new Date();
	// Get current time in local timezone for form display
	const year = now.getFullYear();
	const month = String(now.getMonth() + 1).padStart(2, '0');
	const day = String(now.getDate()).padStart(2, '0');
	const hours = String(now.getHours()).padStart(2, '0');
	const minutes = String(now.getMinutes()).padStart(2, '0');
	return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function setActive(id) {
	activeHabitId = id;
	saveSelectedHabit(id);
	renderAll();
}
function getHabit(id) { return state.habits.find(h => h.id === id) }

function totalsByDate(habitId, days = 30) {
	const map = new Map();
	const today = new Date();
	today.setHours(0, 0, 0, 0);

	for (let i = days - 1; i >= 0; i--) {
		const d = new Date(today);
		d.setDate(today.getDate() - i);
		map.set(d.toISOString().slice(0, 10), 0)
	}

	state.logs.filter(l => !habitId || l.habitId === habitId).forEach(l => {
		const dateKey = new Date(l.date).toISOString().slice(0, 10);

		if (map.has(dateKey)) {
			map.set(dateKey, (map.get(dateKey) || 0) + Number(l.qty));
		}
	});
	const labels = [...map.keys()];
	const values = [...map.values()];
	return { labels, values };
}

function sumToday(habitId) {
	const today = todayStr(); // Get just the date part
	return state.logs.filter(l => {
		if (habitId && l.habitId !== habitId) return false;
		const logDate = new Date(l.date).toISOString().slice(0, 10);
		return logDate === today;
	}).reduce((a, b) => a + Number(b.qty), 0);
}

function humanUnit(unit) { return unit?.trim() ? unit : 'count' }

// ======= Render =======
function renderHabits() {
	habitsList.innerHTML = '';
	if (state.habits.length === 0) { noHabits.style.display = 'block'; return } else { noHabits.style.display = 'none' }
	state.habits.forEach(h => {
		const wrap = document.createElement('div'); wrap.className = 'habit' + (h.id === activeHabitId ? ' active' : '');
		wrap.innerHTML = `
        <div style="width:10px; height:10px; border-radius:999px; background:linear-gradient(135deg, var(--accent), var(--accent-2))"></div>
        <div style="flex:1">
          <div class="title">${h.name}</div>
          <small>${h.goal ? `Goal: ${fmt(h.goal)} ${humanUnit(h.unit)}/day` : 'No daily goal set'}</small>
          <div class="progress" style="margin-top:6px"><i style="width:${Math.min(100, (sumToday(h.id) / (h.goal || 1)) * 100)}%"></i></div>
        </div>
        <button class="ghost" onclick="event.stopPropagation(); editHabit('${h.id}')">Edit</button>
        <button class="danger" onclick="event.stopPropagation(); deleteHabit('${h.id}')">Delete</button>
      `;
		wrap.onclick = () => setActive(h.id);
		habitsList.appendChild(wrap);
	})
}

function renderOverview() {
	const h = getHabit(activeHabitId);
	if (!h) { activeHabitTitle.textContent = 'Overview'; activeHabitMeta.textContent = 'Select a habit'; todayProgress.style.width = '0%'; todaySummary.textContent = 'Today: –'; drawEmpty(barChart); drawEmpty(lineChart); return }
	activeHabitTitle.textContent = h.name;
	activeHabitMeta.textContent = `${h.goal ? `Daily goal: ${fmt(h.goal)} ${humanUnit(h.unit)}` : 'No daily goal'} · Unit: ${humanUnit(h.unit)}`;
	const todayTotal = sumToday(h.id);
	const pct = h.goal ? Math.min(100, (todayTotal / h.goal) * 100) : 0;
	todayProgress.style.width = pct + '%';
	todaySummary.textContent = `Today: ${fmt(todayTotal)} ${humanUnit(h.unit)}${h.goal ? ` (${Math.round(pct)}% of goal)` : ''}`;

	const { labels, values } = totalsByDate(h.id, 30);
	drawBar(barChart, labels, values, `${h.name} — last 30 days (${humanUnit(h.unit)})`);

	// cumulative line
	const cum = values.reduce((arr, v) => { arr.push((arr.at(-1) || 0) + v); return arr }, []);
	drawLine(lineChart, labels, cum, `${h.name} — cumulative (${humanUnit(h.unit)})`);
}

function renderLogs() {
	const h = getHabit(activeHabitId);
	const logs = state.logs
		.filter(l => !h || l.habitId === h.id)
		.sort((a, b) => new Date(b.date) - new Date(a.date))
		.slice(0, 50);

	logsTable.innerHTML = '';
	if (logs.length === 0) { noLogs.style.display = 'block'; return } else { noLogs.style.display = 'none' }

	logs.forEach(l => {
		const tr = document.createElement('tr');
		const habit = getHabit(l.habitId);
		tr.innerHTML = `
        <td>${formatDateForDisplay(l.date, l.dateDisplay)}</td>
        <td>${habit ? habit.name : '—'}</td>
        <td>${fmt(l.qty)} ${humanUnit(habit?.unit)}</td>
        <td style="text-align:right">
          <button class="ghost" onclick="editLog('${l.id}')">Edit</button>
          <button class="danger" onclick="deleteLog('${l.id}')">Delete</button>
        </td>`;
		logsTable.appendChild(tr);
	})
}

function renderAll() {
	if (!state.habits.find(h => h.id === activeHabitId)) activeHabitId = state.habits[0]?.id || null;
	renderHabits();
	renderOverview();
	renderLogs();
	refreshLogHabitOptions();
}

// ======= Charting (Canvas 2D, zero deps) =======
function drawEmpty(canvas) { const ctx = canvas.getContext('2d'); ctx.clearRect(0, 0, canvas.width, canvas.height); ctx.fillStyle = '#8b93a7'; ctx.font = '14px system-ui'; ctx.textAlign = 'center'; ctx.fillText('Select a habit to see charts', canvas.width / 2, canvas.height / 2) }

function scaleData(values, h) { const max = Math.max(1, ...values); const pad = 24; return values.map(v => (h - pad) * (v / max)); }

function drawAxes(ctx, w, h, title) {
	ctx.clearRect(0, 0, w, h);
	ctx.fillStyle = '#c7d2fe'; ctx.font = '16px system-ui'; ctx.textAlign = 'left'; ctx.fillText(title, 16, 26);
	ctx.strokeStyle = 'rgba(255,255,255,.12)';
	ctx.lineWidth = 1; ctx.beginPath();
	// horizontal grid lines
	const rows = 4; for (let i = 0; i <= rows; i++) { const y = 56 + (h - 80) * (i / rows); ctx.moveTo(12, y); ctx.lineTo(w - 12, y) }
	ctx.stroke();
}

function drawBar(canvas, labels, values, title = '') {
	const ctx = canvas.getContext('2d'); const w = canvas.width, h = canvas.height; drawAxes(ctx, w, h, title);
	const baseY = h - 24; const chartH = h - 80; const N = values.length; const gap = 4; const bw = Math.max(2, Math.floor((w - 40 - (N - 1) * gap) / N));
	const scaled = scaleData(values, chartH);
	const grad = ctx.createLinearGradient(0, 0, 0, chartH);
	grad.addColorStop(0, '#6ee7ff');
	grad.addColorStop(1, '#a78bfa');
	ctx.fillStyle = grad;
	for (let i = 0; i < N; i++) {
		const x = 20 + i * (bw + gap);
		const y = baseY - scaled[i];
		const r = 6; // rounded bars
		const wbar = bw; const hbar = scaled[i];
		roundRect(ctx, x, y, wbar, hbar, r); ctx.fill();
	}
	// X labels every 5th day
	ctx.fillStyle = '#8b93a7'; ctx.font = '11px system-ui'; ctx.textAlign = 'center';
	for (let i = 0; i < N; i += 5) { const x = 20 + i * (bw + gap) + bw / 2; ctx.fillText(labels[i].slice(5), x, h - 6) }
}

function drawLine(canvas, labels, values, title = '') {
	const ctx = canvas.getContext('2d'); const w = canvas.width, h = canvas.height; drawAxes(ctx, w, h, title);
	const chartH = h - 80; const chartW = w - 40; const baseX = 20, baseY = h - 24; const N = values.length;
	const max = Math.max(1, ...values);
	ctx.strokeStyle = '#6ee7ff'; ctx.lineWidth = 2; ctx.beginPath();
	for (let i = 0; i < N; i++) {
		const x = baseX + (chartW) * (i / (N - 1));
		const y = baseY - chartH * (values[i] / max);
		if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
	}
	ctx.stroke();
	// points
	ctx.fillStyle = '#a78bfa';
	for (let i = 0; i < N; i++) {
		const x = baseX + (chartW) * (i / (N - 1));
		const y = baseY - chartH * (values[i] / max);
		ctx.beginPath(); ctx.arc(x, y, 3, 0, Math.PI * 2); ctx.fill();
	}
	// X labels every 5th day
	ctx.fillStyle = '#8b93a7'; ctx.font = '11px system-ui'; ctx.textAlign = 'center';
	for (let i = 0; i < N; i += 5) { const x = baseX + (chartW) * (i / (N - 1)); ctx.fillText(labels[i].slice(5), x, h - 6) }
}

function roundRect(ctx, x, y, w, h, r) {
	r = Math.min(r, w / 2, h / 2);
	ctx.beginPath();
	ctx.moveTo(x + r, y);
	ctx.arcTo(x + w, y, x + w, y + h, r);
	ctx.arcTo(x + w, y + h, x, y + h, r);
	ctx.arcTo(x, y + h, x, y, r);
	ctx.arcTo(x, y, x + w, y, r);
	ctx.closePath();
}

function refreshChartsOnResize() {
	// keep CSS size, but set canvas internal resolution to device pixels for crispness
	[barChart, lineChart].forEach(c => {
		const rect = c.getBoundingClientRect();
		const dpr = Math.max(1, window.devicePixelRatio || 1);
		// Use the element's rendered size for the backing store to avoid blurry or tiny charts
		c.width = Math.max(1, Math.floor(rect.width * dpr));
		c.height = Math.max(1, Math.floor(rect.height * dpr));
	});
	renderOverview();
}

// ======= Actions =======
async function editHabit(id) {
	const h = getHabit(id);
	if (!h) return;
	habitName.value = h.name;
	habitUnit.value = h.unit || '';
	habitGoal.value = h.goal ?? '';
	habitDialog.returnValue = '';
	habitDialog.showModal();
	document.getElementById('saveHabitBtn').onclick = async () => {
		const updatedHabit = {
			id: h.id,
			name: habitName.value.trim() || h.name,
			unit: habitUnit.value.trim(),
			goal: habitGoal.value ? Number(habitGoal.value) : undefined
		};
		try {
			await api.updateHabit(updatedHabit);
			await loadData();
			habitDialog.close();
		} catch (error) {
			alert('Failed to update habit: ' + error.message);
		}
	};
}

async function deleteHabit(id) {
	if (!confirm('Delete this habit and its logs?')) return;
	try {
		await api.deleteHabit(id);
		// If we're deleting the currently selected habit, clear the cache
		if (id === activeHabitId) {
			clearSelectedHabit();
		}
		await loadData();
	} catch (error) {
		alert('Failed to delete habit: ' + error.message);
	}
}

async function editLog(id) {
	const l = state.logs.find(x => x.id === id);
	if (!l) return;
	logHabit.value = l.habitId;
	logQty.value = l.qty;
	logDate.value = formatDateForForm(l.date);
	logDialog.returnValue = '';
	logDialog.showModal();
	document.getElementById('saveLogBtn').onclick = async () => {
		const updatedLog = {
			id: l.id,
			habitId: logHabit.value,
			qty: Number(logQty.value),
			date: logDate.value
		};
		try {
			await api.updateLog(updatedLog);
			await loadData();
			logDialog.close();
		} catch (error) {
			alert('Failed to update log: ' + error.message);
		}
	}
}

async function deleteLog(id) {
	if (!confirm('Delete this log entry?')) return;
	try {
		await api.deleteLog(id);
		await loadData();
	} catch (error) {
		alert('Failed to delete log: ' + error.message);
	}
}

function refreshLogHabitOptions() {
	logHabit.innerHTML = state.habits.map(h => `<option value="${h.id}">${h.name}</option>`).join('');
}

// ======= Event wiring =======
document.getElementById('addHabitBtn').onclick = () => {
	habitName.value = ''; habitUnit.value = ''; habitGoal.value = '';
	habitDialog.showModal();
	document.getElementById('saveHabitBtn').onclick = async () => {
		const newHabit = {
			name: habitName.value.trim() || 'New Habit',
			unit: habitUnit.value.trim(),
			goal: habitGoal.value ? Number(habitGoal.value) : 0
		};
		try {
			await api.createHabit(newHabit);
			await loadData();
			habitDialog.close();
		} catch (error) {
			alert('Failed to create habit: ' + error.message);
		}
	}
};

document.getElementById('addLogBtn').onclick = () => {
	if (state.habits.length === 0) { alert('Create a habit first.'); return }
	refreshLogHabitOptions();
	logHabit.value = activeHabitId || state.habits[0].id;
	logQty.value = '';
	logDate.value = getCurrentDateTimeLocal();
	logDialog.showModal();
	document.getElementById('saveLogBtn').onclick = async () => {
		const newLog = {
			habitId: logHabit.value,
			qty: Number(logQty.value || 0),
			date: logDate.value
		};
		try {
			await api.createLog(newLog);
			await loadData();
			logDialog.close();
		} catch (error) {
			alert('Failed to create log: ' + error.message);
		}
	}
};

document.getElementById('exportBtn').onclick = () => {
	const blob = new Blob([JSON.stringify(state, null, 2)], { type: 'application/json' });
	const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = 'habits-export.json'; a.click();
	URL.revokeObjectURL(a.href);
};

document.getElementById('importInput').addEventListener('change', (e) => {
	const file = e.target.files?.[0]; if (!file) return;
	const reader = new FileReader();
	reader.onload = () => {
		try {
			const data = JSON.parse(reader.result);
			if (!data || !Array.isArray(data.habits) || !Array.isArray(data.logs)) throw new Error('Invalid file');
			state = data; save(); alert('Imported successfully.');
		} catch (err) { alert('Failed to import: ' + err.message) }
	};
	reader.readAsText(file);
});

document.getElementById('clearBtn').onclick = () => {
	if (!confirm('Reset all data?')) return;
	// Clear the habit selection cache when resetting data
	clearSelectedHabit();
	store.reset();
	state = store.get();
	activeHabitId = null;
	renderAll();
};

window.addEventListener('resize', debounce(refreshChartsOnResize, 150));

function debounce(fn, ms) { let t; return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms) } }

// ======= Initial render =======
refreshChartsOnResize();
loadData();
