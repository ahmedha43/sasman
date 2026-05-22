# MikroTik Web Management Container (v7) Plan

This plan outlines the architecture and deployment strategy for a self-hosted management container running directly on MikroTik RouterOS v7.

## 1. Architecture Overview
- **Host:** MikroTik RouterOS v7 (ARM/x86).
- **Runtime:** RouterOS Container Feature.
- **Backend:** Go (Golang) - Binary size < 20MB, minimal RAM footprint.
- **State Management:** In-memory with JSON persistence (to avoid Flash wear).
- **Internal API:** Communicates with RouterOS via **Binary API (Port 8728)** for maximum performance.
- **Memory Limit:** 128MB - 256MB (Realistic for RB4011).

---

## 2. Core Modules

### 🌐 Interface Manager
- **WAN Management**: Create PPPoE clients, DHCP Clients, or Static IPs.
- **LAN Management**: Bridge configuration, IP assignment, and DHCP Server setup.
- **Traffic Monitoring**: Real-time pps/bandwidth usage per interface using `/interface/monitor-traffic`.

### 🛣️ Smart Routing Engine (Production PBR)
- **Dynamic lists**: Automatic population of `address-list` via DNS hooks and local API calls.
- **L7 Logic**: Avoid heavy SNI inspection; focus on high-performance Mangle and Routing Table assignments.
- **Gateway Switching**: Map specific "App Groups" (e.g., Gaming, Social, SpeedTest) to specific WAN interfaces.

### 📊 System Dashboard
- Real-time CPU/RAM/Disk stats.
- Connected Users/Leases list.
- One-click "System Maintenance" (DNS Flush, Cache Clear).

---

## 3. Deployment Workflow

### Phase 1: MikroTik Preparation
1. Enable `container` mode (Requires physical interaction or specific boot command).
2. Configure **VETH** (Virtual Ethernet) and **Bridge** for internal communication.
3. Configure **NAT/Masquerade** to allow the container to access the internet/DNS.

### Phase 2: Container Development
1. Develop the REST API wrapper (Fastify or Fiber) to translate Web UI calls to MikroTik commands.
2. Build the Frontend (React/Vite) and serve it as a static asset from the backend.
3. **Multi-stage Docker Build**:
   ```dockerfile
   FROM node:18-alpine AS builder
   # Build frontend/backend
   FROM node:18-alpine
   # Copy only production assets
   CMD ["node", "server.js"]
   ```

### Phase 3: Deployment
1. Build and push image to Docker Hub (e.g., `sasman/mikrotik-manager:latest`).
2. Pull and start on MikroTik:
   ```bash
   /container/add remote-image=sasman/mikrotik-manager interface=veth1 root-dir=disk1/manager_data
   /container/start 0
   ```

---

## 4. Security Recommendations
- **Internal Only**: Do not expose the container port to the WAN; access only via LAN or VPN.
- **Restricted API User**: Create a dedicated MikroTik user with limited `read,write,api,rest-api` permissions.
- **Resource Limits**: Set hard limits on RAM (e.g., 64MB) and CPU to prevent the container from starving the routing process.

---

## 5. Potential Challenges
- **Storage**: Most MikroTik devices have limited internal flash (16MB-128MB). An external USB drive or microSD card is highly recommended for the `root-dir`.
- **Architecture**: Ensure the Docker image is built for the specific architecture (ARM or x86).



2. التشغيل داخل المايكروتك
بعد تفعيل ميزة الحاويات في الراوتر، ادخل على الراوتر وافتح Terminal جديد ثم نفذ الأوامر التالية:

routeros
# 1. إضافة واجهة وهمية للحاوية
/interface/veth/add name=veth-sasman address=172.17.0.2/24 gateway=172.17.0.1
# 2. إضافة جسر (Bridge) لربط الحاوية بالشبكة (اختياري، أو اربط ببريج محلي)
/interface/bridge/add name=br-container
/interface/bridge/port/add bridge=br-container interface=veth-sasman
# 3. سحب الصورة وتشغيل الحاوية (استخدم اسمك ahmedkin99)
/container/add remote-image=ahmedkin99/sasman-manager:v1 interface=veth-sasman root-dir=disk1/sasman-data logging=yes
# 4. بدء التشغيل
/container/start [find where remote-image~"sasman-manager"]
3. الوصول للواجهة
الآن ستعمل الواجهة على IP الحاوية (172.17.0.2). للوصول إليها من خارج الراوتر، يمكنك إضافة قاعدة NAT بسيطة:

routeros
/ip/firewall/nat/add chain=dstnat dst-port=8080 protocol=tcp action=dst-nat to-addresses=172.17.0.2 to-ports=8080
بهذه الطريقة، سيعمل نظام الإدارة بالكامل من داخل الراوتر ولن تحتاج لإبقاء جهاز الكمبيوتر الخاص بك يعمل!

هل واجهت أي صعوبة في تنفيذ أمر بناء الـ Docker؟