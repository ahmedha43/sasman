let logInterval = null;

async function fetchLogs() {
    const area = document.getElementById('logs-area');
    if (!area) return;
    try {
        const fetcher = (typeof apiFetch === 'function') ? apiFetch('/radius/api/logs') : fetch('/radius/api/logs', {
            headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('radius_token') || '') }
        });
        const res = await fetcher;
        if (!res.ok) throw new Error('Failed to fetch logs');
        const text = await res.text();

        // Only update if changed to avoid cursor jumping
        if (area.value !== text) {
            const shouldScroll = area.scrollTop + area.clientHeight >= area.scrollHeight - 20;
            area.value = text;
            if (shouldScroll) {
                area.scrollTop = area.scrollHeight;
            }
        }
    } catch (e) {
        // Silent catch to avoid console spam
    }
}

function startAutoRefresh() {
    stopAutoRefresh();
    fetchLogs();
    logInterval = setInterval(fetchLogs, 2000); // Refresh every 2 seconds
}

function stopAutoRefresh() {
    if (logInterval) {
        clearInterval(logInterval);
        logInterval = null;
    }
}

async function clearLogs() {
    if (!confirm('هل أنت متأكد من تصفير سجل RADIUS؟ سيتم حذف كل البيانات الحالية من السجل.')) return;
    try {
        const fetcher = (typeof apiFetch === 'function') 
            ? apiFetch('/radius/api/logs', { method: 'DELETE' }) 
            : fetch('/radius/api/logs', {
                method: 'DELETE',
                headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('radius_token') || '') }
            });
        const res = await fetcher;
        const result = await res.json();
        alert(result.message || result.error || 'تم تصفير السجل بنجاح');
        fetchLogs();
    } catch (e) {
        alert('خطأ: ' + e.message);
    }
}

function copyLogs() {
    const area = document.getElementById('logs-area');
    area.select();
    document.execCommand('copy');
    alert('تم نسخ السجل إلى الحافظة');
}

// Auto-fetch logs when entering the tab
window.addEventListener('tabChanged', (e) => {
    if (e.detail.tab === 'logs') {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
});
