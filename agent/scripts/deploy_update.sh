#!/bin/bash
# ============================================================
# SASMAN VPS Update Script - Safe Container Update
# يحدث حاوية SASMAN مع الحفاظ على البيانات
# ============================================================

set -e

echo "============================================"
echo "  SASMAN Container Update Script"
echo "============================================"

# 1. معرفة اسم الحاوية الحالية
echo ""
echo "🔍 الحاويات الحالية على السيرفر:"
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'

echo ""
echo "🔍 البحث عن حاوية SASMAN..."
CONTAINER_NAME=$(docker ps --format '{{.Names}}' | grep -i sasman | head -1)

if [ -z "$CONTAINER_NAME" ]; then
    echo "❌ لم يتم إيجاد حاوية SASMAN نشطة!"
    echo "الحاويات الموجودة:"
    docker ps -a --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
    exit 1
fi

echo "✅ وجدنا الحاوية: $CONTAINER_NAME"

# 2. معرفة المجلد والإعدادات الحالية
echo ""
echo "🔍 فحص إعدادات الحاوية..."
IMAGE_NAME=$(docker inspect $CONTAINER_NAME --format '{{.Config.Image}}')
DATA_MOUNT=$(docker inspect $CONTAINER_NAME --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{.Source}}{{end}}{{end}}')
PORTS=$(docker inspect $CONTAINER_NAME --format '{{range $p, $conf := .HostConfig.PortBindings}}{{(index $conf 0).HostPort}}->{{$p}} {{end}}')
RESTART_POLICY=$(docker inspect $CONTAINER_NAME --format '{{.HostConfig.RestartPolicy.Name}}')
ENV_VARS=$(docker inspect $CONTAINER_NAME --format '{{range .Config.Env}}--env {{.}} {{end}}')

echo "  الصورة الحالية: $IMAGE_NAME"
echo "  مجلد البيانات: $DATA_MOUNT"
echo "  المنافذ: $PORTS"
echo "  سياسة الإعادة: $RESTART_POLICY"

# 3. البحث عن docker-compose.yml
echo ""
COMPOSE_DIR=""
for dir in ~/sasman-central ~/sasman /root/sasman-central /root/sasman /opt/sasman; do
    if [ -f "$dir/docker-compose.yml" ] || [ -f "$dir/compose.yml" ]; then
        COMPOSE_DIR="$dir"
        break
    fi
done

echo "📁 مجلد docker-compose: ${COMPOSE_DIR:-'لم يوجد'}"

# 4. نسخ احتياطي للبيانات
echo ""
echo "💾 أخذ نسخة احتياطية من البيانات..."
BACKUP_DIR="/root/sasman-backup-$(date +%Y%m%d-%H%M%S)"
if [ -n "$DATA_MOUNT" ] && [ -d "$DATA_MOUNT" ]; then
    cp -r "$DATA_MOUNT" "$BACKUP_DIR"
    echo "✅ نسخة احتياطية محفوظة في: $BACKUP_DIR"
else
    echo "⚠️  لم يتم إيجاد volume مباشر، سنأخذ نسخة من داخل الحاوية..."
    mkdir -p "$BACKUP_DIR"
    docker cp $CONTAINER_NAME:/app/data "$BACKUP_DIR/"
    echo "✅ نسخة احتياطية محفوظة في: $BACKUP_DIR"
fi

# 5. البحث عن أحدث image متاح للتحديث
echo ""
echo "🔄 محاولة سحب أحدث صورة..."

# محاولة سحب الصورة الحالية أولاً
if docker pull "$IMAGE_NAME" 2>/dev/null; then
    echo "✅ تم سحب أحدث نسخة من: $IMAGE_NAME"
    NEW_IMAGE="$IMAGE_NAME"
else
    # البحث عن tar في /tmp
    if [ -f "/tmp/sasman-new.tar" ]; then
        echo "📦 وجدنا صورة جديدة في /tmp/sasman-new.tar، جاري التحميل..."
        docker load -i /tmp/sasman-new.tar
        NEW_IMAGE=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep sasman | head -1)
        echo "✅ صورة جديدة محملة: $NEW_IMAGE"
    else
        echo "⚠️  لا توجد صورة جديدة. سيتم إعادة تشغيل الحاوية الحالية فقط بعد بناء محلي."
        NEW_IMAGE="$IMAGE_NAME"
    fi
fi

# 6. التحقق من docker-compose وتحديثه
if [ -n "$COMPOSE_DIR" ]; then
    echo ""
    echo "🐳 تحديث عبر docker-compose في: $COMPOSE_DIR"
    cd "$COMPOSE_DIR"
    
    # إيقاف حاوية SASMAN فقط
    docker-compose stop sasman 2>/dev/null || docker compose stop sasman 2>/dev/null || true
    docker-compose rm -f sasman 2>/dev/null || docker compose rm -f sasman 2>/dev/null || true
    
    # تشغيل الحاوية الجديدة
    docker-compose up -d sasman 2>/dev/null || docker compose up -d sasman 2>/dev/null
    
    echo "✅ تم تحديث SASMAN عبر docker-compose"
else
    # 7. تحديث يدوي بدون docker-compose
    echo ""
    echo "🔧 تحديث يدوي بدون docker-compose..."
    
    # استخراج كامل إعدادات الحاوية
    NETWORK=$(docker inspect $CONTAINER_NAME --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}')
    
    echo "  إيقاف الحاوية القديمة..."
    docker stop $CONTAINER_NAME
    docker rm $CONTAINER_NAME
    
    echo "  تشغيل الحاوية الجديدة..."
    docker run -d \
        --name $CONTAINER_NAME \
        --restart ${RESTART_POLICY:-unless-stopped} \
        --network ${NETWORK:-bridge} \
        -p 80:80 \
        -p 1812:1812/udp \
        -p 1813:1813/udp \
        -v "${DATA_MOUNT:-/root/sasman-data}:/app/data" \
        $NEW_IMAGE
    
    echo "✅ تم تشغيل الحاوية الجديدة"
fi

# 8. التحقق من النتيجة
echo ""
echo "⏳ انتظار بدء التشغيل (15 ثانية)..."
sleep 15

echo ""
echo "📊 حالة الحاوية بعد التحديث:"
docker ps --filter name=$CONTAINER_NAME --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'

echo ""
echo "📋 آخر 30 سطر من السجل:"
docker logs $CONTAINER_NAME --tail 30

echo ""
echo "============================================"
echo "✅ تم التحديث بنجاح!"
echo "   النسخة الاحتياطية محفوظة في: $BACKUP_DIR"
echo "============================================"
