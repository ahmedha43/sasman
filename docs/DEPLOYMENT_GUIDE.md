# دليل رفع وتشغيل الحاوية على MikroTik v7

اتباع هذه الخطوات سيسمح لك بتشغيل "نظام الإدارة" كحاوية (Container) داخل الراوتر والوصول إليه عبر المتصفح دون الحاجة لجهاز كمبيوتر يعمل باستمرار.

## 1. المتطلبات الأساسية
- راوتر مايكروتك يدعم الحاويات (v7.6 فأعلى، معالج ARM أو x86).
- مساحة تخزين (يُفضل استخدام USB خارجي أو MicroSD لزيادة عمر الفلاشة الداخلية).
- تفعيل وضع الحاويات في المايكروتك:
  ```bash
  /system/device-mode/update container=yes
  ```
  *(يتطلب الضغط على زر Reset في الراوتر أو إعادة تشغيل فيزيائية خلال 5 دقائق لتأكيد التفعيل).*

---

## 2. بناء صورة الحاوية (Docker Build)

### الطريقة الأولى: الرفع عبر Docker Hub (الأسهل للتحديث)
هذه الطريقة تقوم ببناء صورة تدعم كافة المعالجات (`arm64` و `arm v7`) وترفعها لحسابك.

1. **تسجيل الدخول:**
   ```bash
   docker login
   ```
2. **البناء والرفع:**
   ```bash
   docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t ahmedkin99/sasman-manager:v1 --push .
   ```
   *(هذا الأمر يجعل التطبيق يدعم كافة الأجهزة: CHR, x86, ARM الحديث والقديم).*

### الطريقة الثانية: التصدير كملف يدوياً (Manual Tar)
إذا كنت لا تريد استخدام الإنترنت للرفع، يمكنك تصدير ملف ورفعه عبر WinBox.

1. **تصدير ملف ARM64 (للراوترات الحديثة):**
   ```bash
   docker buildx build --platform linux/arm64 -t sasman-image --output type=tar,dest=sasman-manager.tar .
   ```
2. **تصدير ملف ARM v7 (للراوترات الأقدم):**
   ```bash
   docker buildx build --platform linux/arm/v7 -t sasman-image-arm --output type=tar,dest=sasman-manager-arm.tar .
   ```

---

## 3. إعدادات المايكروتك (Winbox Terminal)

يجب توفير شبكة افتراضية للحاوية:

```bash
# 1. إنشاء واجهة افتراضية (VETH)
/interface veth add name=veth-sasman address=172.17.0.2/24 gateway=172.17.0.1

# 2. إنشاء جسر (Bridge) لربط الحاوية
/interface bridge add name=bridge-container
/interface bridge port add bridge=bridge-container interface=veth-sasman

# 3. إضافة آيبي للجسر ليكن بوابة الحاوية
/ip address add address=172.17.0.1/24 interface=bridge-container

# 4. السماح للحاوية بالوصول للإنترنت عبر NAT
/ip firewall nat add chain=srcnat src-address=172.17.0.0/24 action=masquerade
```

---

## 4. إضافة وتشغيل الحاوية

### إذا استخدمت Docker Hub:
```bash
/container/add remote-image=ahmedkin99/sasman-manager:v1 interface=veth-sasman root-dir=disk1/sasman-data logging=yes
```

### إذا استخدمت ملف Tar (قم برفعه أولاً عبر Files في Winbox):
```bash
/container/add file=sasman-manager.tar interface=veth-sasman root-dir=disk1/sasman-data logging=yes
```

/container/config set registry-url=https://registry-1.docker.io  ضبط رابط السجل (Registry) ليؤشر إلى Docker Hub:

الحل النهائي (تغيير المنفذ الخارجي):
سنقوم بتغيير المنفذ الذي تفتحه في المتصفح إلى 8090 ليكون بعيداً عن خدمات المايكروتك:

1. أولاً، احذف قاعدة الـ NAT السابقة:

routeros
/ip/firewall/nat/remove [find where dst-port=8080]
2. أضف القاعدة الجديدة (تستقبل طلباتك على 8090 وترسلها للحاوية):

routeros
/ip/firewall/nat/add chain=dstnat dst-port=8090 protocol=tcp action=dst-nat to-addresses=172.17.0.2 to-ports=8080
3. الآن، افتح المتصفح واستخدم هذا الرابط: http://192.168.10.1:8090

**لبدء التشغيل:**
```bash
/container/start [find where status=stopped]
```

---

## 5. الوصول إلى واجهة الإدارة
بعد تشغيل الحاوية، يمكنك الوصول إليها من داخل الشبكة عبر:
`http://172.17.0.2:8080`

**للوصول عبر آيبي الراوتر (مثلاً 192.168.88.1):**
أضف قاعدة توجيه المنفذ (Port Forwarding):
```bash
/ip firewall nat add chain=dstnat dst-port=8080 protocol=tcp action=dst-nat to-addresses=172.17.0.2 to-ports=8080
```
الآن افتح: `http://192.168.88.1:8080`

---

## 6. حل مشكلة الشهادات (SSL Error)
إذا ظهر لك خطأ `no trusted CA certificate found` عند سحب الصورة، نفذ هذه الأوامر في المايكروتك لتحميل شهادات الثقة:
```bash
/tool fetch url=https://curl.se/ca/cacert.pem
/certificate import file-name=cacert.pem passphrase=""
```
بعدها أعد محاولة إضافة الحاوية (`/container/add`).

---

## ملاحظات هامة 🛡️
- **التخزين:** تأكد أن `root-dir` يشير إلى قرص خارجي (`disk1` أو `usb1`) وليس الفلاشة الداخلية الصغير لتجنب تلفها.
- **التحديث:** عند تحديث الكود، أعد بناء الصورة ورفعها، ثم احذف الحاوية القديمة وأضفها من جديد بالصورة المحدثة.


/container/add file=sasman-manager.tar interface=veth-sasman root-dir=disk1/sasman-data


docker buildx build --platform linux/arm64,linux/arm/v7 -t ahmedkin99/sasman-manager:v1 --push .

