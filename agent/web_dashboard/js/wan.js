let wanPortConfigs = [];
let availableInterfaces = [];

async function loadWanStatus() {
    if (!isLicensed) return;
    const tbody = document.getElementById('wan-status-tbody');
    try {
        const res = await fetch('/api/status/wan');
        const data = await res.json();
        if (data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" style="padding:30px; text-align:center; color:var(--text-muted); font-size:0.95rem;"><i class="fa-solid fa-circle-exclamation" style="margin-left:6px;"></i>لا توجد خطوط مدموجة نشطة حالياً</td></tr>';
            return;
        }
        tbody.innerHTML = data.map(line => {
            const badgeClass = line.running ? 'badge-success' : 'badge-danger';
            const badgeText = line.running ? 'نشط (Running)' : 'متوقف';
            return `
                <tr>
                    <td style="padding:16px 20px;"><strong><i class="fa-solid fa-network-wired" style="color:var(--primary); margin-left:8px;"></i>${line.interface}</strong></td>
                    <td style="padding:16px 20px;"><span class="badge badge-secondary" style="font-weight:700;">${line.type}</span></td>
                    <td style="padding:16px 20px;">
                        <span class="badge ${badgeClass}">
                            ${badgeText}
                        </span>
                    </td>
                    <td style="padding:16px 20px;">
                        <button class="btn btn-delete btn-danger" onclick="deleteInterfaceManual('${line.interface}', '${line.type}', '${line.id}')" style="width:auto; padding:8px 14px; font-size:12px;">
                            <i class="fa-solid fa-trash-can"></i> حذف
                        </button>
                    </td>
                </tr>
            `;
        }).join('');
    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="4" style="padding:30px; text-align:center; color:var(--danger); font-weight:bold;"><i class="fa-solid fa-triangle-exclamation" style="margin-left:6px;"></i>حدث خطأ في تحميل البيانات</td></tr>';
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
                <label style="margin:0; font-weight:600; display:flex; align-items:center; gap:8px; cursor:pointer; background:#fff; padding:8px 12px; border-radius:var(--radius-sm); border:1px solid var(--border); transition:var(--transition-fast);" onmouseover="this.style.borderColor='var(--primary)'" onmouseout="this.style.borderColor='var(--border)'">
                    <input type="checkbox" class="lan-port-check" value="${i.name}" style="width:16px; height:16px; margin:0; cursor:pointer;"> 
                    <span>${i.name}</span>
                </label>
            `).join('');
        }

        // Build multi-gateway dropdown for routing tab
        buildGwOptions(data);

        ['pcc-lan'].forEach(id => {
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
        <div class="card" style="margin-bottom:20px; border:1px solid var(--border); position:relative; padding:24px;">
            <button onclick="removeWanPortCard(${config.id})" style="position:absolute; left:20px; top:20px; background:var(--danger-light); border:none; color:var(--danger); cursor:pointer; font-size:16px; width:30px; height:30px; border-radius:50%; display:flex; align-items:center; justify-content:center; transition:var(--transition-fast);" onmouseover="this.style.background='var(--danger)'; this.style.color='white';" onmouseout="this.style.background='var(--danger-light)'; this.style.color='var(--danger)';">
                <i class="fa-solid fa-xmark"></i>
            </button>
            <div class="grid">
                <div class="form-group" style="margin:0;">
                    <label style="margin-top:0;"><i class="fa-solid fa-ethernet" style="color:var(--primary); margin-left:6px;"></i>المنفذ الفيزيائي للراوتر:</label>
                    <select onchange="updateWanPort(${config.id}, 'interface', this.value)" style="margin-bottom:0;">
                        ${availableInterfaces.filter(i => i.type === 'ether').map(i => `
                            <option value="${i.name}" ${config.interface === i.name ? 'selected' : ''}>${i.name}</option>
                        `).join('')}
                    </select>
                </div>
                <div class="form-group" style="margin:0;">
                    <label style="margin-top:0;"><i class="fa-solid fa-circle-nodes" style="color:var(--primary); margin-left:6px;"></i>نوع بروتوكول الاتصال:</label>
                    <select onchange="updateWanPort(${config.id}, 'type', this.value)" style="margin-bottom:0;">
                        <option value="PPPOE" ${config.type === 'PPPOE' ? 'selected' : ''}>PPPoE (مكالمات متعددة/مشاركة)</option>
                        <option value="DHCP" ${config.type === 'DHCP' ? 'selected' : ''}>DHCP Client (تلقائي)</option>
                    </select>
                </div>
            </div>

            ${config.type === 'PPPOE' ? `
                <div style="margin-top:20px; background:#fafbfc; padding:20px; border-radius:var(--radius-md); border:1px solid var(--border);">
                    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:15px; border-bottom:1px dashed var(--border); padding-bottom:10px;">
                        <span style="font-size:13px; font-weight:700; color:var(--text-muted);"><i class="fa-solid fa-key" style="margin-left:6px;"></i>حسابات وجلسات الـ PPPoE (Sessions)</span>
                        <div style="display:flex; align-items:center; gap:10px;">
                            <span style="font-size:13px; font-weight:600;">عدد الخطوط:</span>
                            <input type="number" min="1" max="100" value="${config.sessions.length}" 
                                onchange="updateWanPort(${config.id}, 'sessions-count', this.value)" 
                                style="width:70px; height:34px; text-align:center; margin:0; padding:4px;">
                        </div>
                    </div>
                    <div id="sessions-list-${config.id}">
                        ${config.sessions.map((s, idx) => `
                            <div style="display:grid; grid-template-columns: 1fr 1fr 100px 70px; gap:12px; margin-bottom:12px; align-items:center;">
                                <input type="text" placeholder="اسم المستخدم (Username)" value="${s.user}" oninput="updateSession(${config.id}, ${idx}, 'user', this.value)" style="margin:0; height:38px;">
                                <input type="password" placeholder="كلمة المرور (Password)" value="${s.pass}" oninput="updateSession(${config.id}, ${idx}, 'pass', this.value)" style="margin:0; height:38px;">
                                <input type="number" min="1" value="${s.weight}" oninput="updateSession(${config.id}, ${idx}, 'weight', this.value)" placeholder="الوزن" style="margin:0; height:38px; text-align:center;">
                                <span id="pct-${config.id}-${idx}" class="badge badge-success" style="display:flex; align-items:center; justify-content:center; height:38px; font-size:12px; font-weight:bold; margin:0; border-radius:var(--radius-md);">%0</span>
                            </div>
                        `).join('')}
                    </div>
                </div>
            ` : `
                <div style="margin-top:20px; background:#fffbeb; border:1px solid rgba(245, 158, 11, 0.2); padding:16px; border-radius:var(--radius-md); display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px;">
                    <span style="font-size:13px; color:#b45309; font-weight:600;"><i class="fa-solid fa-circle-info" style="margin-left:6px;"></i>سيتم تفعيل DHCP Client تلقائياً وتوزيع الأوزان على خطوط التوجيه.</span>
                    <div style="display:flex; align-items:center; gap:10px;">
                        <label style="margin:0; font-weight:700;">الوزن:</label>
                        <input type="number" min="1" value="${config.sessions[0].weight}" oninput="updateSession(${config.id}, 0, 'weight', this.value)" style="width:70px; margin:0; text-align:center; height:38px;">
                        <span id="pct-${config.id}-0" class="badge badge-warning" style="padding:10px 14px; border-radius:var(--radius-md); font-size:12px; font-weight:bold; display:inline-flex; align-items:center; justify-content:center; height:38px; margin:0;">%0</span>
                    </div>
                </div>
            `}
        </div>
    `).join('');
    calculateWeights();
}

async function submitBatchWAN() {
    const btn = event.target;
    const originalText = btn.innerHTML;
    btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin"></i> جاري إرسال الإعدادات...`;
    btn.disabled = true;

    const mode = document.getElementById('wan-mode')?.value || 'pcc';

    const payload = {
        configs: wanPortConfigs,
        lan: document.getElementById('pcc-lan').value,
        classifier: document.getElementById('wan-classifier').value,
        mode: mode
    };

    try {
        const res = await fetch('/api/wan/setup-batch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        if (res.ok) {
            showToast("تم تطبيق وحفظ إعدادات دمج الخطوط بنجاح! 🌐", "success");
        } else {
            showToast(result.error, "error");
        }
        loadInterfaces();
    } catch (e) {
        showToast("حدث خطأ أثناء إرسال البيانات للراوتر", "error");
    } finally {
        btn.innerHTML = originalText;
        btn.disabled = false;
    }
}

async function deleteInterfaceManual(name, type, id) {
    if (!confirm(`هل أنت متأكد من حذف ${name}؟`)) return;
    try {
        const endpoint = type === 'PPPOE' ? `/api/wan/pppoe/${encodeURIComponent(name)}` : `/api/wan/dhcp-client/${encodeURIComponent(id)}`;
        const res = await fetch(endpoint, { method: 'DELETE' });
        const result = await res.json();
        if (res.ok) {
            showToast(`تم إزالة خط الـ ${name} بنجاح`, "success");
        } else {
            showToast(result.error, "error");
        }
        loadAll();
    } catch (e) {
        showToast("فشل الاتصال لإتمام الحذف بالراوتر", "error");
    }
}

function closeOptModal() {
    const modal = document.getElementById('optimization-modal');
    if (modal) {
        modal.classList.remove('active');
    }
    loadWanStatus();
    loadInterfaces();
}

async function optimizeWanQuality() {
    const modal = document.getElementById('optimization-modal');
    const body = document.getElementById('opt-modal-body');
    if (!modal || !body) return;

    modal.classList.add('active');
    body.innerHTML = `
        <div style="text-align:center; padding:30px 10px;">
            <div class="loading-spinner"></div>
            <h4 style="margin-top:15px; color:#1e293b; font-weight:800;">جاري تشغيل الفحص والتحسين التلقائي...</h4>
            <p style="color:#64748b; font-size:0.9rem; margin-top:8px;">نقوم الآن بقياس زمن الاستجابة (Latency)، التذبذب (Jitter)، وفقدان الحزم (Packet Loss) لكل خط WAN نشط بشكل معزول ومتوازي.</p>
            <p style="color:#0ea5e9; font-size:0.8rem; font-weight:bold; margin-top:12px; animation: pulse 1.5s infinite;">⏳ الفحص متوازي وسريع جداً، يرجى الانتظار ثانية واحدة...</p>
        </div>
    `;

    try {
        const res = await fetch('/api/wan/optimize', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                classifier: document.getElementById('wan-classifier')?.value || 'both-addresses-and-ports',
                lan: document.getElementById('pcc-lan')?.value || 'bridge'
            })
        });

        const data = await res.json();
        
        if (!res.ok || data.status === 'error') {
            body.innerHTML = `
                <div style="text-align:center; padding:20px 10px;">
                    <i class="fa-solid fa-circle-xmark" style="font-size:3rem; color:var(--danger); margin-bottom:15px;"></i>
                    <h4 style="color:var(--danger); font-weight:800;">فشل فحص وتحسين الشبكة</h4>
                    <p style="color:#64748b; margin-top:5px;">${data.message || 'حدث خطأ غير متوقع أثناء الاتصال بالخادم.'}</p>
                    <button class="btn" style="margin-top:20px; width:auto; background:#64748b; color:white; border:none;" onclick="closeOptModal()">إغلاق</button>
                </div>
            `;
            return;
        }

        let itemsHtml = data.report.map(line => {
            let badgeClass = 'quality-high';
            let dotColor = '#10b981'; // green
            if (line.score <= 30) {
                badgeClass = 'quality-low';
                dotColor = '#ef4444'; // red
            } else if (line.score <= 60) {
                badgeClass = 'quality-medium';
                dotColor = '#f59e0b'; // orange
            }

            return `
                <div class="report-item" style="padding:18px; border-radius:var(--radius-md); border:1px solid var(--border); background:#fafbfc; display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                    <div>
                        <div style="display:flex; align-items:center; gap:8px; margin-bottom:6px;">
                            <span style="height:10px; width:10px; background-color:${dotColor}; border-radius:50%; display:inline-block;"></span>
                            <strong style="font-size:1.05rem; color:var(--text-main);"><i class="fa-solid fa-globe" style="color:var(--primary); margin-left:6px;"></i>${line.name} (${line.interface})</strong>
                        </div>
                        <div style="font-size:0.85rem; color:var(--text-muted); display:flex; gap:15px; flex-wrap:wrap; font-weight:600;">
                            <span>الاستجابة: <strong style="color:var(--text-main);">${line.latency_ms} ms</strong></span>
                            <span>التذبذب: <strong style="color:var(--text-main);">${line.jitter_ms} ms</strong></span>
                            <span>الفقد: <strong style="color:var(--text-main);">${line.packet_loss}%</strong></span>
                            <span>الوزن الجديد: <strong style="color:var(--primary);">${line.new_weight}</strong></span>
                        </div>
                        <p style="margin:6px 0 0 0; font-size:0.85rem; font-weight:bold; color:${dotColor};"><i class="fa-solid fa-wand-magic-sparkles" style="margin-left:4px;"></i>${line.message}</p>
                    </div>
                    <div>
                        <span class="quality-badge ${badgeClass}" style="padding:6px 12px; font-weight:700;">الجودة ${line.score}%</span>
                    </div>
                </div>
            `;
        }).join('');

        body.innerHTML = `
            <div style="margin-bottom:20px;">
                <p style="color:var(--text-muted); font-size:0.95rem; line-height:1.6;"><i class="fa-solid fa-circle-check" style="color:var(--success); margin-left:6px;"></i>تم قياس جودة جميع مسارات WAN وإعادة توزيع الأوزان ديناميكياً لحماية المستخدمين وعزل الخطوط التالفة فوراً.</p>
            </div>
            <div style="max-height:350px; overflow-y:auto; padding-right:5px; margin-bottom:25px;">
                ${itemsHtml}
            </div>
            <div style="text-align:left;">
                <button class="btn btn-success" style="width:auto; padding:10px 28px; font-weight:bold; height:42px;" onclick="closeOptModal()">
                    <i class="fa-solid fa-circle-check"></i> تطبيق وإنهاء ⚡
                </button>
            </div>
        `;
    } catch (e) {
        body.innerHTML = `
            <div style="text-align:center; padding:20px 10px;">
                <i class="fa-solid fa-triangle-exclamation" style="font-size:3rem; color:var(--danger); margin-bottom:15px;"></i>
                <h4 style="color:var(--danger); font-weight:800;">خطأ في الاتصال بالخادم</h4>
                <p style="color:#64748b; margin-top:5px;">فشل إرسال طلب التحسين الفوري. تأكد من أن السيرفر يعمل وحاول مجدداً.</p>
                <button class="btn" style="margin-top:20px; width:auto; background:#64748b; color:white; border:none;" onclick="closeOptModal()">إغلاق</button>
            </div>
        `;
    }
}
