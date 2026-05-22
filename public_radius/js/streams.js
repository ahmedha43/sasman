// streams.js - Management logic for SASMAN Local Channels

function showAddStreamModal() {
    document.getElementById('stream-modal-title').innerText = 'إضافة قناة بث مباشر';
    document.getElementById('stream-old-id').value = '';
    document.getElementById('stream-id').disabled = false;
    document.getElementById('stream-id').value = '';
    document.getElementById('stream-name').value = '';
    document.getElementById('stream-source').value = 'publisher';
    document.getElementById('stream-status').value = 'active';
    document.getElementById('stream-local-relay').checked = false;
    
    document.getElementById('stream-modal').classList.add('active');
}

function showEditStreamModal(stream) {
    document.getElementById('stream-modal-title').innerText = 'تعديل قناة البث';
    document.getElementById('stream-old-id').value = stream.id;
    document.getElementById('stream-id').disabled = true;
    document.getElementById('stream-id').value = stream.id;
    document.getElementById('stream-name').value = stream.name;
    document.getElementById('stream-source').value = stream.source;
    document.getElementById('stream-status').value = stream.status;
    document.getElementById('stream-local-relay').checked = stream.local_relay === 1;
    
    document.getElementById('stream-modal').classList.add('active');
}

async function loadStreams() {
    try {
        const res = await apiFetch('/radius/api/streams');
        const streams = await res.json();
        
        const tbody = document.getElementById('streams-list-tbody');
        tbody.innerHTML = '';
        
        if (!streams || streams.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; padding:20px;">لا توجد قنوات حالياً...</td></tr>';
            return;
        }
        
        streams.forEach(s => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td><strong>${s.id}</strong></td>
                <td>${s.name}</td>
                <td>
                    <code>${s.source}</code>
                    ${s.local_relay === 1 ? '<br><span style="font-size:11px; background:#dcfce7; color:#15803d; padding:2px 6px; border-radius:4px; font-weight:bold; display:inline-block; margin-top:4px;">🔁 إعادة بث محلي (Relay)</span>' : ''}
                </td>
                <td>
                    <span class="badge ${s.status === 'active' ? 'badge-active' : 'badge-expired'}">
                        ${s.status === 'active' ? 'نشط' : 'معطل'}
                    </span>
                </td>
                <td>${new Date(s.created_at).toLocaleString('ar-IQ')}</td>
                <td>
                    <button class="btn btn-secondary" style="width:auto; display:inline-block; padding:6px 12px; font-size:12px; margin-left:5px;" onclick='showEditStreamModal(${JSON.stringify(s)})'>تعديل ✏️</button>
                    <button class="btn btn-danger" style="width:auto; display:inline-block; padding:6px 12px; font-size:12px;" onclick="deleteStream('${s.id}')">حذف 🗑️</button>
                </td>
            `;
            tbody.appendChild(tr);
        });
    } catch (e) {
        console.error(e);
    }
}

async function saveStream() {
    const oldId = document.getElementById('stream-old-id').value;
    const id = document.getElementById('stream-id').value.trim().toLowerCase();
    const name = document.getElementById('stream-name').value.trim();
    const source = document.getElementById('stream-source').value.trim();
    const status = document.getElementById('stream-status').value;
    const local_relay = document.getElementById('stream-local-relay').checked ? 1 : 0;
    
    if (!id || !name || !source) {
        return alert('يرجى ملء جميع الحقول المطلوبة');
    }
    
    const isEdit = oldId !== '';
    const url = isEdit ? `/radius/api/streams/${oldId}` : '/radius/api/streams';
    const method = isEdit ? 'PUT' : 'POST';
    
    try {
        const res = await apiFetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, name, source, status, local_relay })
        });
        
        const data = await res.json();
        if (data.error) {
            alert(data.error);
        } else {
            closeModal('stream-modal');
            loadStreams();
        }
    } catch (e) {
        alert('فشل حفظ القناة');
    }
}

async function deleteStream(id) {
    if (!confirm('هل أنت متأكد من رغبتك في حذف هذه القناة نهائياً؟')) return;
    
    try {
        const res = await apiFetch(`/radius/api/streams/${id}`, {
            method: 'DELETE'
        });
        const data = await res.json();
        if (data.error) {
            alert(data.error);
        } else {
            loadStreams();
        }
    } catch (e) {
        alert('فشل حذف القناة');
    }
}

let currentServerStatus = 'stopped';

async function checkMediaMTXStatus() {
    const badge = document.getElementById('server-status-badge');
    const btn = document.getElementById('server-toggle-btn');
    if (!badge || !btn) return;

    try {
        const res = await apiFetch('/radius/api/streams/server/status');
        const data = await res.json();
        
        currentServerStatus = data.status;
        
        if (data.status === 'running') {
            badge.style.background = '#16a34a';
            badge.style.color = '#ffffff';
            badge.innerText = 'يعمل بنشاط 🟢';
            
            btn.style.background = '#dc2626';
            btn.innerText = 'إيقاف الخادم 🛑';
        } else {
            badge.style.background = '#475569';
            badge.style.color = '#cbd5e1';
            badge.innerText = 'متوقف وموفر للموارد 🔴';
            
            btn.style.background = '#16a34a';
            btn.innerText = 'تشغيل الخادم 🚀';
        }
    } catch (e) {
        console.error('Error checking MediaMTX status:', e);
    }
}

async function toggleMediaMTXServer() {
    const btn = document.getElementById('server-toggle-btn');
    if (!btn) return;
    
    btn.disabled = true;
    const action = currentServerStatus === 'running' ? 'stop' : 'start';
    btn.innerText = action === 'start' ? 'جاري التشغيل... ⏳' : 'جاري الإيقاف... ⏳';
    
    try {
        const res = await apiFetch('/radius/api/streams/server/control', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: action })
        });
        const data = await res.json();
        if (data.error) {
            alert('فشل التحكم في الخادم: ' + data.error);
        }
    } catch (e) {
        alert('فشل الاتصال بالخادم الرئيسي');
    } finally {
        btn.disabled = false;
        await checkMediaMTXStatus();
    }
}

// Streams are loaded by core.js when the tab is opened.
