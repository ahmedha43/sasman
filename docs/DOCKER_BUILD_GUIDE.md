# دليل بناء ورفع صورة Docker إلى Docker Hub

## 📋 المتطلبات

1. حساب على Docker Hub: `ahmedkin99`
2. Docker Desktop مثبت على جهازك
3. تسجيل الدخول عبر Terminal

---

## 🔐 الخطوة 1: تسجيل الدخول

```bash
docker login
```

سيطلب منك:
- **Username**: `ahmedkin99`
- **Password**: كلمة مرور حسابك
- **Email**: بريدك المسجل

✅ **نجاح**: سترى رسالة `Login Succeeded`

---

## 🏗️ الخطوة 2: بناء ورفع الصورة

### الخيار 1: بناء لجميع المعماريات (موصى به)

```bash
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t ahmedkin99/sasman-manager:v2 --push .
```

**يدعم:**
- ✅ `linux/amd64` - أجهزة الكمبيوتر و CHR
- ✅ `linux/arm64` - راوترات ARM الحديثة (RB5009, CCR2004, hEX)
- ✅ `linux/arm/v7` - راوترات ARM القديمة (RB4011, RB3011)

⏱️ **الوقت المتوقع**: 5-15 دقيقة (حسب سرعة الإنترنت)

---

### الخيار 2: بناء لـ ARM فقط (أسرع)

```bash
docker buildx build --platform linux/arm64,linux/arm/v7 -t ahmedkin99/sasman-manager:v2 --push .
```

**متى تستخدمه:**
- إذا كنت تستخدم راوترات MikroTik فقط
- تريد بناء أسرع (2-5 دقائق)

---

### الخيار 3: بناء واختبار محلياً أولاً

```bash
# بناء للمعمارية المحلية
docker build -t sasman-test .

# تشغيل للاختبار
docker run -d --name sasman-test \
  -p 8080:8080 \
  -p 1812:1812/udp \
  -p 1813:1813/udp \
  sasman-test

# اختبار في المتصفح
# http://localhost:8080/admin
# http://localhost:8080/radius

# إيقاف بعد الاختبار
docker stop sasman-test
docker rm sasman-test
```

---

## 📊 الخطوة 3: التحقق من الرفع

### عبر المتصفح:
```
https://hub.docker.com/r/ahmedkin99/sasman-manager
```

### عبر Terminal:
```bash
docker pull ahmedkin99/sasman-manager:v2
```

---

## 🚀 الخطوة 4: استخدام الصورة في MikroTik

### في MikroTik Terminal:

```bash
# 1. إضافة الحاوية
/container/add \
  remote-image=ahmedkin99/sasman-manager:v2 \
  interface=veth-sasman \
  root-dir=disk1/sasman-data \
  logging=yes

# 2. تشغيل الحاوية
/container/start [find where status=stopped]

# 3. التحقق من التشغيل
/container/print
```

---

## 🌐 الخطوة 5: الوصول للواجهة

### من داخل الشبكة:
```
http://172.17.0.2:8080/admin
http://172.17.0.2:8080/radius
```

### من أي مكان (بعد Port Forwarding):

```bash
# إضافة Port Forwarding في MikroTik
/ip firewall nat add \
  chain=dstnat \
  dst-port=8080 \
  protocol=tcp \
  action=dst-nat \
  to-addresses=172.17.0.2 \
  to-ports=8080
```

الآن افتح:
```
http://192.168.88.1:8080/admin
http://192.168.88.1:8080/radius
```

---

## 🔄 إصدارات مختلفة (Versioning)

### عند كل تحديث، غيّر رقم الإصدار:

```bash
# أول إصدار
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v1 --push .

# بعد تعديل
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v2 --push .

# إصلاح bug
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v2.1 --push .

# ميزة جديدة
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v3.0 --push .
```

---

## 🏷️ إضافة Tag "latest"

```bash
# بناء مع latest و v2
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v2 \
  -t ahmedkin99/sasman-manager:latest \
  --push .
```

---

## ⚡ أوامر مفيدة

### 1. إزالة صور قديمة من الجهاز:
```bash
docker image prune -a
```

### 2. إيقاف جميع الحاويات:
```bash
docker stop $(docker ps -q)
```

### 3. حذف جميع الحاويات:
```bash
docker rm $(docker ps -aq)
```

### 4. مراقبة البناء:
```bash
docker buildx build --platform linux/arm64 -t ahmedkin99/sasman-manager:v2 --push . 2>&1 | tee build.log
```

### 5. اختبار صورة محلية:
```bash
docker run -it --rm -p 8080:8080 ahmedkin99/sasman-manager:v2
```

---

## 🐛 حل المشاكل

### ❌ المشكلة: `buildx` غير موجود

```bash
# تثبيت buildx
docker buildx create --name mybuilder --use
docker buildx inspect --bootstrap
```

---

### ❌ المشكلة: `permission denied`

```bash
# في Linux
sudo usermod -aG docker $USER
newgrp docker
```

---

### ❌ المشكلة: `no space left on device`

```bash
# تنظيف Docker
docker system prune -a

# أو زيادة المساحة في Docker Desktop settings
```

---

### ❌ المشكلة: `login expired`

```bash
# إعادة تسجيل الدخول
docker login
```

---

### ❌ المشكلة: `image already exists`

```bash
# استخدم رقم إصدار مختلف
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v2.1 --push .
```

---

## 📦 معلومات الصورة

### المنافذ:
| البورت | الاستخدام | البروتوكول |
|--------|-----------|------------|
| `8080` | واجهة الويب (Admin + RADIUS) | TCP |
| `1812` | FreeRADIUS Authentication | UDP |
| `1813` | FreeRADIUS Accounting | UDP |

### المسارات:
```
/admin          → لوحة تحكم MikroTik
/radius         → نظام RADIUS Billing
/radius/login.html → صفحة تسجيل الدخول
/api/*          → API للوحة Admin
/radius/api/*   → API لنظام RADIUS
```

### بيانات الدخول الافتراضية:
- **Username**: `admin`
- **Password**: `admin`

⚠️ **مهم**: غيّر كلمة المرور فوراً من صفحة "حسابي"!

---

## 🎯 السيناريو الكامل (Quick Start)

```bash
# 1. تسجيل الدخول
docker login

# 2. بناء ورفع
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ahmedkin99/sasman-manager:v2 --push .

# 3. في MikroTik
/container/add remote-image=ahmedkin99/sasman-manager:v2 \
  interface=veth-sasman root-dir=disk1/sasman-data logging=yes
/container/start [find where status=stopped]

# 4. فتح في المتصفح
http://192.168.88.1:8080/admin
```

---

## 📝 ملاحظات مهمة

1. **التخزين**: استخدم `disk1` أو `usb1` وليس الفلاشة الداخلية
2. **التحديث**: احذف الحاوية القديمة قبل إضافة الجديدة
3. **الأمان**: غيّر كلمة المرور الافتراضية فوراً
4. **النسخ الاحتياطي**: نزّل قاعدة البيانات من صفحة "حسابي"
5. **الشهادات**: إذا ظهر خطأ SSL، نفّذ:
   ```bash
   /tool fetch url=https://curl.se/ca/cacert.pem
   /certificate import file-name=cacert.pem passphrase=""
   ```

---

## 📞 الدعم

- **المشاكل العامة**: تحقق من logs
  ```bash
  /container/logging/print
  ```

- **إعادة التشغيل**:
  ```bash
  /container/stop [find]
  /container/start [find]
  ```

- **إعادة الإنشاء**: احذف وأضف من جديد
  ```bash
  /container/remove [find]
  /container/add remote-image=ahmedkin99/sasman-manager:v2 ...
  ```

---

بناء محلي 
docker build -t sasman-test .

تشغيل محلي
docker run -d --name sasman-test -p 80:80 -p 1812:1812/udp -p 1813:1813/udp sasman-test

امر عمل ترخيص 
go run tools/license_generator/main.go

**آخر تحديث**: 2026-04-22  
**الإصدار**: v2 (Unified Port)
