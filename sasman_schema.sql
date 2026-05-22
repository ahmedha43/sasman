CREATE TABLE IF NOT EXISTS radius_profile_meta (
    groupname VARCHAR(64) PRIMARY KEY,
    validity_days INT NOT NULL DEFAULT 0,
    price DECIMAL(10,2) NOT NULL DEFAULT 0,
    admin_id INT DEFAULT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS radius_user_meta (
    username VARCHAR(64) PRIMARY KEY,
    expiration_unix BIGINT DEFAULT NULL,
    renewal_enabled INT NOT NULL DEFAULT 0,
    renewal_profile VARCHAR(64) NOT NULL DEFAULT '',
    full_name VARCHAR(200) NOT NULL DEFAULT '',
    phone VARCHAR(30) NOT NULL DEFAULT '',
    balance DECIMAL(10,2) NOT NULL DEFAULT 0,
    enabled INT NOT NULL DEFAULT 1,
    admin_id INT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_reminder_at TIMESTAMP NULL DEFAULT NULL,
    reminder_sent_for BIGINT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS radius_user_transactions (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    transaction_type VARCHAR(20) NOT NULL DEFAULT '',
    amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    notes VARCHAR(500) NOT NULL DEFAULT '',
    admin_id INT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS radius_whatsapp_config (
    id INT PRIMARY KEY AUTO_INCREMENT,
    admin_id INT NOT NULL UNIQUE,
    enabled INT NOT NULL DEFAULT 0,
    phone_number VARCHAR(30) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    reminder_enabled INT NOT NULL DEFAULT 0,
    reminder_hours INT NOT NULL DEFAULT 24
);

CREATE TABLE IF NOT EXISTS radius_vouchers (
    id INT PRIMARY KEY AUTO_INCREMENT,
    batch_id VARCHAR(40) NOT NULL DEFAULT '',
    code VARCHAR(32) NOT NULL UNIQUE,
    profile_name VARCHAR(64) NOT NULL,
    validity_days INT NOT NULL,
    price DOUBLE NOT NULL DEFAULT 0,
    created_by INT NOT NULL,
    used_by VARCHAR(64) DEFAULT NULL,
    used_at TIMESTAMP NULL DEFAULT NULL,
    is_used INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_radius_vouchers_batch ON radius_vouchers(batch_id);

CREATE TABLE IF NOT EXISTS radius_message_templates (
    id INT PRIMARY KEY AUTO_INCREMENT,
    _template_key VARCHAR(64) NOT NULL UNIQUE,
    template_text TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS radius_admins (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL DEFAULT '',
    email VARCHAR(128) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT 'superadmin',
    parent_id INT DEFAULT NULL,
    balance DECIMAL(10,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS radius_admin_sessions (
    token VARCHAR(64) PRIMARY KEY,
    admin_id INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS radius_system_settings (
    `key` VARCHAR(64) PRIMARY KEY,
    value VARCHAR(253) NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_meta_username ON radius_user_meta(username);
CREATE INDEX idx_transactions_username ON radius_user_transactions(username);
CREATE INDEX idx_transactions_created ON radius_user_transactions(created_at);
CREATE INDEX radius_admin_sessions_admin ON radius_admin_sessions(admin_id);
CREATE INDEX radius_admin_sessions_exp ON radius_admin_sessions(expires_at);

-- Insert Default Message Templates if they don't exist
INSERT IGNORE INTO radius_message_templates (_template_key, template_text, updated_at) VALUES 
('renew_paid', 'تم تجديد اشتراكك بنجاح ✅\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nحالة الدفع: مدفوع', CURRENT_TIMESTAMP),
('renew_debt', 'تم تجديد اشتراكك ⏳\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nملاحظة: تمت إضافة المبلغ كديون\nرصيدك الحالي: {balance} د.ع', CURRENT_TIMESTAMP),
('add_debt', 'تم إضافة ديون 📋\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع', CURRENT_TIMESTAMP),
('payment', 'تم تسديد ديون ✅\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع', CURRENT_TIMESTAMP),
('expiry_reminder', 'تنبيه انتهاء الاشتراك ⚠️\nعزيزي {username}، نود إعلامك أن اشتراكك في باقة {profile} سينتهي قريباً.\nتاريخ الانتهاء: {expiry_date}\nيرجى التجديد لضمان استمرار الخدمة.', CURRENT_TIMESTAMP);

-- Ensure NAS table has custom columns for SASMAN
ALTER TABLE nas ADD COLUMN IF NOT EXISTS profile_nas_ip VARCHAR(15) DEFAULT '' AFTER secret;
ALTER TABLE nas ADD COLUMN IF NOT EXISTS admin_id INT DEFAULT NULL AFTER profile_nas_ip;
