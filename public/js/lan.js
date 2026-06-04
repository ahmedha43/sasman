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
        if (res.ok) {
            showToast("تم إنشاء وتجهيز واجهة البريج المحلي بنجاح! 🌉", "success");
        } else {
            showToast(result.error || "حدث خطأ أثناء تهيئة البريج", "error");
        }
        loadInterfaces();
    } catch (e) {
        showToast("فشل الاتصال بالسيرفر لإتمام التهيئة", "error");
    }
}

async function purgeBridge() {
    if (!confirm("هل أنت متأكد من تصفير كافة إعدادات البريج والـ LAN؟ سيتم حذف الخوادم والمجموعات.")) return;
    try {
        const res = await fetch('/api/lan/purge', { method: 'POST' });
        const result = await res.json();
        if (res.ok) {
            showToast("تم حذف وتصفير جميع إعدادات البريج والمنافذ بنجاح 🗑", "success");
        } else {
            showToast(result.error || "حدث خطأ أثناء التصفير", "error");
        }
        loadAll();
    } catch (e) {
        showToast("فشل الاتصال بالسيرفر لإتمام التصفير", "error");
    }
}
