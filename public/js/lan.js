async function setupBridge() {
    const checked = document.querySelectorAll('.lan-port-check:checked');
    const ports = Array.from(checked).map(c => c.value);

    const payload = {
        name: document.getElementById('bridge-name').value,
        ports: ports,
        ip: document.getElementById('bridge-ip').value,
        enablePppoe: document.getElementById('lan-opt-pppoe').checked,
        pppoeIp: document.getElementById('pppoe-ip').value,
        enableHotspot: document.getElementById('lan-opt-hotspot').checked,
        hotspotIp: document.getElementById('hotspot-ip').value
    };
    try {
        const res = await fetch('/api/lan/bridge', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        alert(result.message || result.error);
        loadInterfaces();
    } catch (e) {
        alert("فشل الاتصال بالسيرفر");
    }
}

async function purgeBridge() {
    if (!confirm("هل أنت متأكد من تصفير كافة إعدادات البريج والـ LAN؟ سيتم حذف الخوادم والمجموعات.")) return;
    try {
        const res = await fetch('/api/lan/purge', { method: 'POST' });
        const result = await res.json();
        alert(result.message || result.error);
        loadAll();
    } catch (e) {
        alert("فشل الاتصال بالسيرفر");
    }
}
