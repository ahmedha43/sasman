async function handleSAS4Migration() {
    const url = document.getElementById('sas4-url').value.trim();
    const username = document.getElementById('sas4-user').value.trim();
    const password = document.getElementById('sas4-pass').value.trim();
    const statusDiv = document.getElementById('sas4-status');

    if (!url || !username || !password) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = '⚠️ يرجى إدخال الرابط واسم المستخدم وكلمة المرور.';
        return;
    }

    statusDiv.style.color = 'var(--primary)';
    statusDiv.innerText = '⏳ جارِ جلب البيانات من SAS 4... قد يستغرق هذا بعض الوقت.';

    try {
        const resp = await fetch('/radius/api/import/sas4', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('radius_token')}`
            },
            body: JSON.stringify({ url, username, password })
        });

        const result = await resp.json();

        if (resp.ok) {
            statusDiv.style.color = 'var(--success)';
            statusDiv.innerText = `✅ ${result.message}`;
            // Refresh stats to show new counts
            if (typeof fetchStats === 'function') fetchStats();
        } else {
            statusDiv.style.color = 'var(--danger)';
            statusDiv.innerText = `❌ فشل الجلب: ${result.error || 'خطأ غير معروف'}`;
        }
    } catch (err) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = `❌ خطأ في الاتصال بالسيرفر: ${err.message}`;
    }
}

async function handleSystemReset() {
    if (!confirm('🚨 هل أنت متأكد تماماً؟ سيتم حذف جميع المشتركين والباقات والسجلات نهائياً!')) {
        return;
    }

    if (!confirm('❗ هذه هي الفرصة الأخيرة، هل تريد حقاً تصفير النظام بالكامل؟')) {
        return;
    }

    try {
        const resp = await fetch('/radius/api/system/reset', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('radius_token')}`
            }
        });

        const result = await resp.json();

        if (resp.ok) {
            alert('✅ ' + result.message);
            window.location.reload();
        } else {
            alert('❌ فشل الحذف: ' + (result.error || 'خطأ غير معروف'));
        }
    } catch (err) {
        alert('❌ خطأ في الاتصال بالسيرفر: ' + err.message);
    }
}
