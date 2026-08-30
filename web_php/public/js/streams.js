// streams.js - Management logic for source-based live channels

function showAddStreamModal() {
    document.getElementById('stream-modal-title').innerText = 'إضافة قناة بث مباشر';
    document.getElementById('stream-old-id').value = '';
    document.getElementById('stream-id').disabled = false;
    document.getElementById('stream-id').value = '';
    document.getElementById('stream-name').value = '';
    document.getElementById('stream-source').value = '';
    document.getElementById('stream-status').value = 'active';
    
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
    const local_relay = 0;
    
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

// Streams are loaded by core.js when the tab is opened.
