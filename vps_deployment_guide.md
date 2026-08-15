# دليل رفع السيرفر المركزي (Central Gateway) على VPS

> [!IMPORTANT]
> السيرفر المركزي هو الجزء الذي يعمل على الإنترنت ويستقبل الاتصالات من جميع العملاء (Agents). يجب رفعه **مرة واحدة فقط** على VPS خاص بك.

---

## 1. المتطلبات الأساسية للـ VPS

| المتطلب | الحد الأدنى | الموصى به |
|:---|:---|:---|
| نظام التشغيل | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| RAM | 512 MB | 1 GB |
| Storage | 10 GB | 20 GB |
| CPU | 1 Core | 2 Cores |
| دومين مربوط | `sas-man.net` | أي دومين تمتلكه |

---

## 2. ضبط DNS للدومين الرئيسي

في لوحة تحكم DNS الخاصة بالدومين، أضف السجلات التالية:

```
# السيرفر الرئيسي
A     @              → IP_VPS_HERE
A     *              → IP_VPS_HERE   ← (Wildcard لكل الـ Subdomains)

# مثال إذا دومينك sas-man.net وـIP للـ VPS هو 1.2.3.4:
A     sas-man.net         1.2.3.4
A     *.sas-man.net       1.2.3.4
```

> [!IMPORTANT]
> سجل `*.sas-man.net` ضروري جداً حتى يصل كل `client1.sas-man.net` و `client2.sas-man.net` إلى نفس الـ VPS وتتم إعادة التوجيه للعميل الصحيح.

---

## 3. إنشاء Dockerfile خاص بالسيرفر المركزي

أنشئ ملف جديد في مجلد `server/` باسم `Dockerfile`:

```dockerfile
# server/Dockerfile
FROM golang:alpine AS builder

WORKDIR /app

RUN sed -i 's/https/http/g' /etc/apk/repositories && \
    apk add --no-cache build-base sqlite-dev git ca-certificates

ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# نسخ ملفات المشروع الكاملة (مطلوبة لأن server/ يستخدم pkg/ من root)
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg
RUN go mod download -x -modcacherw

COPY . .

# بناء binary السيرفر المركزي فقط
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o central-server ./server

# ─── Final Image ───
FROM alpine:3.19
WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite curl && \
    mkdir -p /app/data

COPY --from=builder /app/central-server ./central-server

EXPOSE 8080

ENV ADDR=:8080
ENV SASMAN_CENTRAL_DOMAIN=sas-man.net

ENTRYPOINT ["./central-server"]
```

---

## 4. إنشاء `docker-compose.yml` للسيرفر المركزي

أنشئ مجلداً جديداً على الـ VPS مثلاً `~/sasman-central/` وضع فيه الملف التالي:

```yaml
# ~/sasman-central/docker-compose.yml
version: '3.8'

services:
  central:
    image: ghcr.io/your-username/sasman-central:latest
    # أو build من المشروع مباشرة:
    # build:
    #   context: .
    #   dockerfile: server/Dockerfile
    container_name: sasman-central
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - ADDR=:8080
      - SASMAN_CENTRAL_DOMAIN=sas-man.net    # ← غيّر لدومينك
      - SASMAN_DB_PATH=/app/data/sasman-central.db
    volumes:
      - ./data:/app/data
    networks:
      - sasman-net

networks:
  sasman-net:
    driver: bridge
```

---

## 5. إعداد Nginx كـ Reverse Proxy مع SSL

### تثبيت Nginx و Certbot على الـ VPS:
```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### ملف إعداد Nginx:
```bash
sudo nano /etc/nginx/sites-available/sasman-central
```

ضع هذا المحتوى:
```nginx
# /etc/nginx/sites-available/sasman-central

# الدومين الرئيسي + كل الـ Subdomains
server {
    listen 80;
    server_name sas-man.net *.sas-man.net;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_http_version 1.1;

        # ضروري جداً لـ WebSocket
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeout مناسب لاتصالات WebSocket الطويلة
        proxy_read_timeout  3600s;
        proxy_send_timeout  3600s;
        proxy_connect_timeout 60s;
    }
}
```

### تفعيل الموقع:
```bash
sudo ln -s /etc/nginx/sites-available/sasman-central /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### تثبيت SSL مجاني (Let's Encrypt):
```bash
# للدومين الرئيسي
sudo certbot --nginx -d sas-man.net

# لإضافة Wildcard SSL (يتطلب DNS challenge)
sudo certbot certonly --manual --preferred-challenges dns \
  -d sas-man.net -d "*.sas-man.net"
```

> [!TIP]
> بعد تثبيت SSL، تغيّر عنوان الـ WebSocket في إعدادات العميل من `ws://` إلى `wss://`.

---

## 6. خطوات الرفع على الـ VPS

```bash
# 1. ادخل لـ VPS
ssh root@your-vps-ip

# 2. ثبّت Docker و Docker Compose
curl -fsSL https://get.docker.com | sh
apt install -y docker-compose-plugin

# 3. أنشئ المجلد وانسخ ملف docker-compose
mkdir -p ~/sasman-central/data
cd ~/sasman-central

# 4. انسخ docker-compose.yml (من الخطوة 4 أعلاه)
nano docker-compose.yml

# 5. بناء وتشغيل السيرفر
# إذا تريد البناء من الكود مباشرة:
git clone https://github.com/YOUR_REPO/mikrotik_manager.git .
docker compose up -d --build

# أو تشغيل فقط إذا عندك الـ image جاهز:
docker compose up -d

# 6. تحقق من التشغيل
docker compose logs -f
curl http://localhost:8080/health
```

---

## 7. اختبار الاتصال بعد الرفع

بعد رفع السيرفر، تحقق من:

```bash
# 1. صحة السيرفر
curl https://sas-man.net/health
# Expected: {"status":"ok","service":"central-server",...}

# 2. واجهة الإدارة
# افتح في المتصفح: https://sas-man.net/admin

# 3. قائمة الـ Agents
curl https://sas-man.net/api/agents
```

---

## 8. إعداد العميل (Agent) للاتصال بالسيرفر المرفوع

في واجهة الويب الخاصة بكل عميل (تبويب الترخيص):

| الحقل | القيمة |
|:---|:---|
| وضع التشغيل | `Agent` |
| Subdomain | مثلاً `client1` (يصبح `client1.sas-man.net`) |
| Token | التوكن الذي أنشأته من `/admin` |
| Gateway URL | `wss://sas-man.net/ws` |

---

## 9. ملاحظات مهمة

> [!WARNING]
> - تأكد أن بورت `8080` مفتوح في Firewall الـ VPS أو استخدم `ufw allow 8080`
> - بعد تشغيل Nginx، يمكنك إغلاق البورت `8080` وفتح `80` و `443` فقط
> - قاعدة بيانات SQLite تُحفظ في `./data/sasman-central.db` — قم بعمل Backup منتظم لهذا الملف

> [!TIP]
> لجعل السيرفر يعمل تلقائياً عند إعادة تشغيل الـ VPS:
> ```bash
> systemctl enable docker
> docker compose up -d  # يعمل تلقائياً بسبب restart: unless-stopped
> ```
