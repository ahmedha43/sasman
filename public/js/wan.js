let wanPortConfigs = [];
let availableInterfaces = [];

async function loadWanStatus() {
    if (!isLicensed) return;
    const tbody = document.getElementById('wan-status-tbody');
    try {
        const res = await fetch('/api/status/wan');
        const data = await res.json();
        if (data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" style="padding:20px; text-align:center; color:#64748b;">لا توجد خطوط مدموجة حالياً</td></tr>';
            return;
        }
        tbody.innerHTML = data.map(line => `
            <tr style="border-bottom:1px solid #e2e8f0;">
                <td style="padding:12px;"><strong>${line.interface}</strong></td>
                <td style="padding:12px;"><span style="background:#e0f2fe; color:#0369a1; padding:4px 8px; border-radius:6px; font-size:12px;">${line.type}</span></td>
                <td style="padding:12px;">
                    <span style="background:${line.running ? '#dcfce7' : '#fee2e2'}; color:${line.running ? '#166534' : '#991b1b'}; padding:4px 8px; border-radius:6px; font-size:12px;">
                        ${line.running ? 'نشط (Running)' : 'متوقف'}
                    </span>
                </td>
                <td style="padding:12px;">
                    <button class="btn btn-danger" onclick="deleteInterfaceManual('${line.interface}', '${line.type}', '${line.id}')" style="width:auto; padding:5px 10px; font-size:12px;">حذف</button>
                </td>
            </tr>
        `).join('');
    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="4" style="padding:20px; text-align:center; color:#ef4444;">خطأ في تحميل البيانات</td></tr>';
    }
}

async function loadInterfaces() {
    if (!isLicensed) return;
    try {
        const res = await fetch('/api/interfaces');
        const data = await res.json();
        if (!res.ok) return;
        availableInterfaces = data;

        const lanContainer = document.getElementById('lan-ports-selection');
        const localPorts = data.filter(i => i.name.includes('ether') || i.name.includes('wlan') || i.name.includes('sfp'));
        if (lanContainer) {
            lanContainer.innerHTML = localPorts.map(i => `
                <label style="margin:0; font-weight:normal; display:flex; align-items:center; gap:8px;">
                    <input type="checkbox" class="lan-port-check" value="${i.name}"> ${i.name}
                </label>
            `).join('');
        }

        ['gateway-list-routing', 'pcc-lan'].forEach(id => {
            const el = document.getElementById(id);
            if (el) {
                const current = el.value;
                el.innerHTML = data.map(i => `<option value="${i.name}" ${i.name == current ? 'selected' : ''}>${i.name}</option>`).join('');
            }
        });

        if (wanPortConfigs.length === 0) addWanPortCard();
    } catch (e) { }
}

function addWanPortCard() {
    const id = Date.now();
    const config = {
        id: id,
        interface: availableInterfaces.find(i => i.type === 'ether')?.name || 'ether1',
        type: 'PPPOE',
        sessions: [{ user: '', pass: '', weight: 1 }]
    };
    wanPortConfigs.push(config);
    renderWanPorts();
}

function removeWanPortCard(id) {
    wanPortConfigs = wanPortConfigs.filter(c => c.id !== id);
    renderWanPorts();
    calculateWeights();
}

function updateWanPort(id, field, value) {
    const config = wanPortConfigs.find(c => c.id === id);
    if (!config) return;
    if (field === 'sessions-count') {
        const count = parseInt(value);
        while (config.sessions.length < count) config.sessions.push({ user: '', pass: '', weight: 1 });
        while (config.sessions.length > count) config.sessions.pop();
    } else {
        config[field] = value;
    }
    renderWanPorts();
}

function updateSession(portId, sessionIndex, field, value) {
    const config = wanPortConfigs.find(c => c.id === portId);
    if (!config) return;
    config.sessions[sessionIndex][field] = field === 'weight' ? parseInt(value) || 1 : value;
    calculateWeights();
}

function calculateWeights() {
    let total = 0;
    wanPortConfigs.forEach(c => c.sessions.forEach(s => total += s.weight));
    const totalDisplay = document.getElementById('total-weight-display');
    if (totalDisplay) totalDisplay.innerText = total;

    wanPortConfigs.forEach(c => {
        c.sessions.forEach((s, idx) => {
            const pct = total > 0 ? Math.round((s.weight / total) * 100) : 0;
            const el = document.getElementById(`pct-${c.id}-${idx}`);
            if (el) el.innerText = `%${pct}`;
        });
    });
}

function renderWanPorts() {
    const container = document.getElementById('wan-ports-container');
    if (!container) return;

    container.innerHTML = wanPortConfigs.map(config => `
        <div class="card" style="margin-bottom:15px; border:1px solid #e2e8f0; position:relative;">
            <button onclick="removeWanPortCard(${config.id})" style="position:absolute; left:15px; top:15px; background:none; border:none; color:#ef4444; cursor:pointer; font-size:24px;">×</button>
            <div class="grid">
                <div>
                    <label>المنفذ الفيزيائي:</label>
                    <select onchange="updateWanPort(${config.id}, 'interface', this.value)">
                        ${availableInterfaces.filter(i => i.type === 'ether').map(i => `
                            <option value="${i.name}" ${config.interface === i.name ? 'selected' : ''}>${i.name}</option>
                        `).join('')}
                    </select>
                </div>
                <div>
                    <label>نوع الاتصال:</label>
                    <select onchange="updateWanPort(${config.id}, 'type', this.value)">
                        <option value="PPPOE" ${config.type === 'PPPOE' ? 'selected' : ''}>PPPoE (Multi-Sessions)</option>
                        <option value="DHCP" ${config.type === 'DHCP' ? 'selected' : ''}>DHCP Client</option>
                    </select>
                </div>
            </div>

            ${config.type === 'PPPOE' ? `
                <div style="margin-top:15px; background:#f8fafc; padding:15px; border-radius:12px;">
                    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:10px;">
                        <span style="font-size:13px; font-weight:bold; color:#64748b;">قائمة الاشتراكات (Sessions)</span>
                        <div style="display:flex; align-items:center; gap:10px;">
                            <span style="font-size:12px;">عدد الخطوط:</span>
                            <input type="number" min="1" max="100" value="${config.sessions.length}" 
                                onchange="updateWanPort(${config.id}, 'sessions-count', this.value)" 
                                style="width:60px; height:30px; text-align:center;">
                        </div>
                    </div>
                    <div id="sessions-list-${config.id}">
                        ${config.sessions.map((s, idx) => `
                            <div style="display:grid; grid-template-columns: 1fr 1fr 80px 60px; gap:10px; margin-bottom:8px;">
                                <input type="text" placeholder="اسم المستخدم" value="${s.user}" oninput="updateSession(${config.id}, ${idx}, 'user', this.value)">
                                <input type="password" placeholder="كلمة المرور" value="${s.pass}" oninput="updateSession(${config.id}, ${idx}, 'pass', this.value)">
                                <input type="number" min="1" value="${s.weight}" oninput="updateSession(${config.id}, ${idx}, 'weight', this.value)" placeholder="الوزن">
                                <span id="pct-${config.id}-${idx}" style="display:flex; align-items:center; justify-content:center; background:#e0f2fe; color:#0369a1; border-radius:6px; font-size:11px; font-weight:bold;">%0</span>
                            </div>
                        `).join('')}
                    </div>
                </div>
            ` : `
                <div style="margin-top:15px; background:#fff7ed; padding:15px; border-radius:12px; display:flex; justify-content:space-between; align-items:center;">
                    <span style="font-size:13px; color:#c2410c;">سيتم تفعيل DHCP Client تلقائياً وتوزيع الأوزان.</span>
                    <div style="display:flex; align-items:center; gap:10px;">
                        <label style="margin:0;">الوزن:</label>
                        <input type="number" min="1" value="${config.sessions[0].weight}" oninput="updateSession(${config.id}, 0, 'weight', this.value)" style="width:60px;">
                        <span id="pct-${config.id}-0" style="background:#ffedd5; color:#9a3412; padding:5px 10px; border-radius:6px; font-size:11px; font-weight:bold;">%0</span>
                    </div>
                </div>
            `}
        </div>
    `).join('');
    calculateWeights();
}

async function submitBatchWAN() {
    const btn = event.target;
    const originalText = btn.innerText;
    btn.innerText = "جارِ المعالجة...";
    btn.disabled = true;

    const payload = {
        configs: wanPortConfigs,
        lan: document.getElementById('pcc-lan').value,
        classifier: document.getElementById('wan-classifier').value
    };

    try {
        const res = await fetch('/api/wan/setup-batch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        alert(result.message || result.error);
        loadInterfaces();
    } catch (e) {
        alert("حدث خطأ في الاتصال");
    } finally {
        btn.innerText = originalText;
        btn.disabled = false;
    }
}

async function deleteInterfaceManual(name, type, id) {
    if (!confirm(`هل أنت متأكد من حذف ${name}؟`)) return;
    const endpoint = type === 'PPPOE' ? `/api/wan/pppoe/${encodeURIComponent(name)}` : `/api/wan/dhcp-client/${encodeURIComponent(id)}`;
    const res = await fetch(endpoint, { method: 'DELETE' });
    const result = await res.json();
    alert(result.message || result.error);
    loadAll();
}
