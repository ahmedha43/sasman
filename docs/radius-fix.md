MMM      MMM       KKK                          TTTTTTTTTTT  
    KKK
  MMMM    MMMM       KKK                          TTTTTTTTTTT  
    KKK
  MMM MMMM MMM  III  KKK  KKK  RRRRRR     OOOOOO      TTT     I
II  KKK  KKK
  MMM  MM  MMM  III  KKKKK     RRR  RRR  OOO  OOO     TTT     I
II  KKKKK
  MMM      MMM  III  KKK KKK   RRRRRR    OOO  OOO     TTT     I
II  KKK KKK
  MMM      MMM  III  KKK  KKK  RRR  RRR   OOOOOO      TTT     I
II  KKK  KKK

  MikroTik RouterOS 7.23 (c) 1999-2026       https://www.mikrot
ik.com/


Press F1 for help


[admin@RB951Ui] > export 
# 2026-06-09 19:15:45 by RouterOS 7.23
# software id = ZZNA-J5L6
#
# model = RB951Ui-2HnD
# serial number = B8710C84F32B
/interface bridge
add admin-mac=48:8F:5A:46:82:5B auto-mac=no comment=defconf \
    name=bridge
/interface wireless
set [ find default-name=wlan1 ] band=2ghz-b/g/n \
    channel-width=20/40mhz-XX disabled=no distance=indoors \
    frequency=auto mode=ap-bridge ssid=MikroTik-46825F \
    wireless-protocol=802.11
/interface list
add comment=defconf name=WAN
add comment=defconf name=LAN
add name=WAN-PPP
add comment=TM_LAN name=TM_LAN
/interface wireless security-profiles
set [ find default=yes ] authentication-types=wpa2-psk \
    group-ciphers=tkip,aes-ccm mode=dynamic-keys \
    supplicant-identity="\D8\A9\D9\87" unicast-ciphers=\
    tkip,aes-ccm
/iot wiliot servers
set *1 address=mqtt.us-east-2.prod.wiliot.cloud name=\
    "Wiliot US East"
/ip hotspot profile
add dns-name=s.net hotspot-address=192.168.88.1 name=hsprof1 \
    use-radius=yes
/ip pool
add name=default-dhcp ranges=192.168.88.10-192.168.88.254
/ip dhcp-server
add address-pool=default-dhcp interface=bridge name=defconf
/routing table
add comment=Route-YouTube fib name=table-YouTube
/disk settings
set auto-media-interface=bridge auto-media-sharing=yes \
    auto-smb-sharing=yes
/interface bridge port
add bridge=bridge comment=defconf interface=ether2
add bridge=bridge comment=defconf interface=ether3
add bridge=bridge comment=defconf interface=ether4
add bridge=bridge comment=defconf interface=ether5
add bridge=bridge comment=defconf interface=wlan1
/ip neighbor discovery-settings
set discover-interface-list=TM_LAN
/ip settings
set ipv4-multipath-hash-policy=l4
/interface list member
add comment=defconf interface=bridge list=LAN
add comment=TM_WAN interface=ether1 list=WAN
add comment=TM_LAN interface=bridge list=TM_LAN
/ip address
add address=192.168.88.1/24 comment=defconf interface=bridge \
    network=192.168.88.0
/ip dhcp-client
add add-default-route=no comment=TM_WAN interface=ether1 \
    name=client1
/ip dhcp-server network
add address=192.168.88.0/24 comment=defconf dns-server=\
    192.168.88.1 gateway=192.168.88.1
/ip dns
set address-list-extra-time=1w3d allow-remote-requests=yes \
    servers=1.1.1.1,1.0.0.1 use-doh-server=\
    https://cloudflare-dns.com/dns-query
/ip dns static
add address=192.168.88.1 comment=defconf name=router.lan \
    type=A
add address=1.1.1.1 comment=TM_DoH name=cloudflare-dns.com \
    type=A
add address=1.0.0.1 comment=TM_DoH name=cloudflare-dns.com \
    type=A
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=ggpht.com \
    type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    googlevideo.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    music.youtube.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=youtu.be \
    type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    youtube-nocookie.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=youtube.com \
    type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    youtubegaming.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    youtubei.googleapis.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=\
    youtubekids.com type=FWD
add address-list=list-YouTube comment=Route-YouTube \
    forward-to=8.8.8.8 match-subdomain=yes name=ytimg.com \
    type=FWD
/ip firewall address-list
add address=192.168.88.0/24 comment=TM_Bypass list=\
    TM_Local_Subnets
add address=10.0.0.0/8 comment=TM_Bypass list=\
    TM_Local_Subnets
add address=172.16.0.0/12 comment=TM_Bypass list=\
    TM_Local_Subnets
add address=192.168.0.0/16 comment=TM_Bypass list=\
    TM_Local_Subnets
add address=cibeg.com comment=TM_Sticky list=TM_Banks
add address=ahly.net comment=TM_Sticky list=TM_Banks
add address=fawry.com comment=TM_Sticky list=TM_Banks
/ip firewall filter
add action=passthrough chain=unused-hs-chain comment=\
    "place hotspot rules here" disabled=yes
add action=accept chain=forward comment=Route-YouTube \
    connection-state=established,related routing-mark=\
    table-YouTube
add action=accept chain=input comment=\
    "defconf: accept established,related,untracked" \
    connection-state=established,related,untracked
add action=drop chain=input comment="defconf: drop invalid" \
    connection-state=invalid
add action=accept chain=input comment="defconf: accept ICMP" \
    protocol=icmp
add action=accept chain=input comment=\
    "defconf: accept to local loopback (for CAPsMAN)" \
    dst-address=127.0.0.1 in-interface=lo src-address=\
    127.0.0.1
add action=drop chain=input comment=\
    "defconf: drop all not coming from LAN" \
    in-interface-list=!LAN
add action=accept chain=forward comment=\
    "defconf: accept in ipsec policy" ipsec-policy=in,ipsec
add action=accept chain=forward comment=\
    "defconf: accept out ipsec policy" ipsec-policy=\
    out,ipsec
add action=fasttrack-connection chain=forward comment=\
    "defconf: fasttrack" connection-state=\
    established,related disabled=yes
add action=accept chain=forward comment=\
    "defconf: accept established,related, untracked" \
    connection-state=established,related,untracked
add action=drop chain=forward comment=\
    "defconf: drop invalid" connection-state=invalid
add action=drop chain=forward comment=\
    "defconf: drop all from WAN not DSTNATed" \
    connection-nat-state=!dstnat in-interface-list=WAN
add action=accept chain=input comment=\
    TM_Accept_Established_WAN connection-state=\
    established,related in-interface-list=WAN
add action=drop chain=input comment=TM_Drop_Invalid_WAN \
    connection-state=invalid in-interface-list=WAN
add action=accept chain=input comment=TM_Accept_DHCP_WAN \
    dst-port=68 in-interface-list=WAN protocol=udp src-port=\
    67
add action=drop chain=input comment=TM_Block_DNS_WAN \
    dst-port=53 in-interface-list=WAN protocol=udp
add action=drop chain=input comment=TM_Block_DNS_WAN \
    dst-port=53 in-interface-list=WAN protocol=tcp
add action=drop chain=input comment=TM_Block_ISP_Access \
    connection-state=new in-interface-list=WAN
/ip firewall mangle
add action=accept chain=prerouting comment=TM_Bypass_Local \
    dst-address-type=local
add action=accept chain=prerouting comment=\
    TM_Local_to_Local_Bypass dst-address-list=\
    TM_Local_Subnets src-address-list=TM_Local_Subnets
add action=change-mss chain=forward comment=TM_Fix_MSS \
    in-interface-list=TM_LAN new-mss=clamp-to-pmtu protocol=\
    tcp tcp-flags=syn
add action=change-ttl chain=postrouting comment=\
    TM_Anti_DPI_TTL new-ttl=set:64 out-interface=ether1 \
    src-address-list=TM_Local_Subnets
add action=mark-routing chain=prerouting comment=\
    Route-YouTube dst-address-list=list-YouTube \
    new-routing-mark=table-YouTube passthrough=no \
    src-address-list=TM_Local_Subnets
/ip firewall nat
add action=passthrough chain=unused-hs-chain comment=\
    "place hotspot rules here" disabled=yes
add action=redirect chain=dstnat comment=TM_Force_DNS \
    dst-port=53 protocol=udp src-address-list=\
    TM_Local_Subnets
add action=redirect chain=dstnat comment=TM_Force_DNS \
    dst-port=53 protocol=tcp src-address-list=\
    TM_Local_Subnets
add action=masquerade chain=srcnat comment=TM_Masq_WAN1 \
    out-interface=ether1
add action=redirect chain=dstnat comment=\
    "Force DNS - YouTube" dst-port=53 protocol=udp to-ports=\
    53
add action=redirect chain=dstnat comment=\
    "Force DNS - YouTube" dst-port=53 protocol=tcp to-ports=\
    53
add action=masquerade chain=srcnat comment=\
    "masquerade hotspot network" src-address=192.168.88.0/24
/ip firewall raw
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-ggpht.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=ggpht.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.ggpht.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.ggpht.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-googlevideo.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=googlevideo.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.googlevideo.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.googlevideo.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-music.youtube.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=music.youtube.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.music.youtube.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=*.music.youtube.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtu.be dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=youtu.be
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtu.be dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.youtu.be
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtube-nocookie.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=youtube-nocookie.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtube-nocookie.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=*.youtube-nocookie.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtube.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=youtube.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtube.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.youtube.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtubegaming.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=youtubegaming.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtubegaming.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=*.youtubegaming.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtubei.googleapis.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=youtubei.googleapis.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtubei.googleapis.com \
    dst-port=443 protocol=tcp src-address-list=\
    !TM_Local_Subnets tls-host=*.youtubei.googleapis.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-youtubekids.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=youtubekids.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.youtubekids.com dst-port=\
    443 protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.youtubekids.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-ytimg.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=ytimg.com
add action=add-dst-to-address-list address-list=list-YouTube \
    chain=prerouting comment=SNI-*.ytimg.com dst-port=443 \
    protocol=tcp src-address-list=!TM_Local_Subnets \
    tls-host=*.ytimg.com
/ip hotspot
add address-pool=default-dhcp disabled=no interface=bridge \
    name=hotspot1 profile=hsprof1
/ip hotspot user
add name=admin
/ip route
add comment=TM_Main_WAN1 distance=1 dst-address=0.0.0.0/0 \
    gateway=192.168.1.1%ether1
add check-gateway=ping comment=Route-YouTube dst-address=\
    0.0.0.0/0 gateway=192.168.1.1 routing-table=\
    table-YouTube
add check-gateway=ping comment=Route-YouTube dst-address=\
    0.0.0.0/0 gateway=ether2 routing-table=table-YouTube
/ipv6 firewall address-list
add address=::/128 comment="defconf: unspecified address" \
    list=bad_ipv6
add address=::1/128 comment="defconf: lo" list=bad_ipv6
add address=fec0::/10 comment="defconf: site-local" list=\
    bad_ipv6
add address=::ffff:0.0.0.0/96 comment="defconf: ipv4-mapped" \
    list=bad_ipv6
add address=::/96 comment="defconf: ipv4 compat" list=\
    bad_ipv6
add address=100::/64 comment="defconf: discard only " list=\
    bad_ipv6
add address=2001:db8::/32 comment="defconf: documentation" \
    list=bad_ipv6
add address=2001:10::/28 comment="defconf: ORCHID" list=\
    bad_ipv6
add address=3ffe::/16 comment="defconf: 6bone" list=bad_ipv6
/ipv6 firewall filter
add action=accept chain=input comment=\
    "defconf: accept established,related,untracked" \
    connection-state=established,related,untracked
add action=drop chain=input comment="defconf: drop invalid" \
    connection-state=invalid
add action=accept chain=input comment=\
    "defconf: accept ICMPv6" protocol=icmpv6
add action=accept chain=input comment=\
    "defconf: accept UDP traceroute" dst-port=33434-33534 \
    protocol=udp
add action=accept chain=input comment=\
    "defconf: accept DHCPv6-Client prefix delegation." \
    dst-port=546 protocol=udp src-address=fe80::/10
add action=accept chain=input comment="defconf: accept IKE" \
    dst-port=500,4500 protocol=udp
add action=accept chain=input comment=\
    "defconf: accept ipsec AH" protocol=ipsec-ah
add action=accept chain=input comment=\
    "defconf: accept ipsec ESP" protocol=ipsec-esp
add action=accept chain=input comment=\
    "defconf: accept all that matches ipsec policy" \
    ipsec-policy=in,ipsec
add action=drop chain=input comment=\
    "defconf: drop everything else not coming from LAN" \
    in-interface-list=!LAN
add action=fasttrack-connection chain=forward comment=\
    "defconf: fasttrack6" connection-state=\
    established,related
add action=accept chain=forward comment=\
    "defconf: accept established,related,untracked" \
    connection-state=established,related,untracked
add action=drop chain=forward comment=\
    "defconf: drop invalid" connection-state=invalid
add action=drop chain=forward comment=\
    "defconf: drop packets with bad src ipv6" \
    src-address-list=bad_ipv6
add action=drop chain=forward comment=\
    "defconf: drop packets with bad dst ipv6" \
    dst-address-list=bad_ipv6
add action=drop chain=forward comment=\
    "defconf: rfc4890 drop hop-limit=1" hop-limit=equal:1 \
    protocol=icmpv6
add action=accept chain=forward comment=\
    "defconf: accept ICMPv6" protocol=icmpv6
add action=accept chain=forward comment=\
    "defconf: accept HIP" protocol=139
add action=accept chain=forward comment=\
    "defconf: accept IKE" dst-port=500,4500 protocol=udp
add action=accept chain=forward comment=\
    "defconf: accept ipsec AH" protocol=ipsec-ah
add action=accept chain=forward comment=\
    "defconf: accept ipsec ESP" protocol=ipsec-esp
add action=accept chain=forward comment=\
    "defconf: accept all that matches ipsec policy" \
    ipsec-policy=in,ipsec
add action=drop chain=forward comment=\
    "defconf: drop everything else not coming from LAN" \
    in-interface-list=!LAN
/radius
add address=172.17.0.1 disabled=yes service=\
    ppp,hotspot,wireless timeout=3s
add address=192.168.88.254 service=\
    ppp,login,hotspot,wireless timeout=3s
/radius incoming
set accept=yes
/system clock
set time-zone-name=Asia/Baghdad
/tool bandwidth-server


### تفعيل وضع ديباغ لمشكلة مصادقة RADIUS

فيما يلي خطوات عملية لتفعيل وضع التصحيح محلياً وجمع بيانات تساعدنا في معرفة سبب "كلمة مرور غير صحيحة" رغم أنها صحيحة:

- 1) تشغيل خادم RADIUS المحلي (خادم Go الموجود في هذا المشروع) مع تفعيل وضع التصحيح (Debug)
    - انتقل لمجلد المشروع ثم شغّل الخادم في الواجهة الأمامية مع تعيين متغير البيئة `DEBUG_RADIUS=1` لتراكم السجلات على الـ stdout و `data/radius.log`: 
        - PowerShell:
            ```powershell
            cd D:\SASMAN\mikrotik_manager
            $env:DEBUG_RADIUS="1"
            go run .
            ```
        - CMD:
            ```cmd
            cd D:\SASMAN\mikrotik_manager
            set DEBUG_RADIUS=1
            go run .
            ```
        - Linux / macOS:
            ```bash
            cd /path/to/mikrotik_manager
            DEBUG_RADIUS=1 go run .
            ```
        - أو ببناء ملف تنفيذي ثم تشغيله في Windows:
            ```powershell
            go build -o sasman.exe .
            $env:DEBUG_RADIUS="1"
            .\sasman.exe
            ```
    - الملف اللوج الافتراضي للخادم داخل المشروع هو `data/radius.log`. في PowerShell لعرض السجلات الحية:
        ```powershell
        Get-Content data\radius.log -Wait -Tail 100
        ```
    - على Linux استخدم:
        ```bash
        tail -f data/radius.log
        ```

- 2) التحقق من وجود سر الـ NAS الصحيح في قاعدة البيانات
    - افتح قاعدة بيانات RADIUS (SQLite) وتأكد من جدول `nas` أن عنوان الـ NAS و`secret` مطابق لما في الميكروتيك:
        ```bash
        sqlite3 data/radius.db "SELECT nasname, secret FROM nas;"
        ```

- 3) مراقبة حزم RADIUS (لفحص ما يرسله الـ NAS وما يرد به الخادم)
    - على Linux: 
        ```bash
        sudo tcpdump -n -s 0 -A udp port 1812 or udp port 1813
        ```
    - على Windows: افتح Wireshark وابحث باستخدام الفلتر `udp.port == 1812 || udp.port == 1813`.

- 4) تجربة طلب مصادقة يدوي (اختبار PAP وMS-CHAP)
    - إذا مثبتة أدوات FreeRADIUS client: استخدم `radtest` لاختبار PAP بسرعة:
        ```bash
        radtest testuser password 127.0.0.1 0 mysecret
        ```
        حيث `mysecret` هو secret المسجل في جدول `nas` لخادم الاختبار.

- 5) FreeRADIUS (إذا تستخدمه بدلاً من خادم Go)
    - شغّل FreeRADIUS في وضع التصحيح المفصل:
        ```bash
        sudo systemctl stop freeradius
        sudo freeradius -X
        ```
    - تفحص الإخراج التفصيلي لطلبات Access-Request والردود وسبب الرفض (Message-Authenticator، NT-Response، إلخ).

- 6) نقاط تحقق سريعة عند ظهور "كلمة المرور غير صحيحة":
    - تأكد أن اسم المستخدم مستخدم بنفس الحروف (بعض البروتوكولات حساسة لحالة الأحرف).
    - تأكد من أن `NAS-IP-Address` في DB يطابق عنوان الـ NAS أو يوجد صف `0.0.0.0` يستخدم كـ wildcard.
    - تحقق ما إذا كان الـ NAS يرسل PAP أو MS-CHAPv2 (في سجل الطلب سيظهر ذلك). إذا MS-CHAPv2 تأكد أن الخادم يوافق على Message-Authenticator وتوليد MPPE keys.
    - راجع `data/radius.log` لأن خادم Go يسجل تفاصيل الطلبات والرفض مع سبب مترجم (انظر رسائل مثل "رفض الاتصال" و"كلمة المرور غير صحيحة").

- 7) إذا أرسلت لي السجلات
    - أرسل لي مقطع من `data/radius.log` (20-100 سطر) مع إخراج `tcpdump` أو لقطات Wireshark للحزم ذات الصلة، وسأحللها وأشير إلى سبب الرفض بالضبط.

ملاحظة سريعة: خادم Go في هذا المشروع يسجّل تلقائياً إلى `data/radius.log` ويطبع سبب الرفض مترجمًا بالعربية في كثير من الحالات، لذا أفضل نقطة بداية هي تشغيل الخادم في الواجهة الأمامية أو مشاهدة الملف `data/radius.log` أثناء محاولة المصادقة من الـ NAS.
set enabled=no
/tool mac-server
set allowed-interface-list=TM_LAN
/tool mac-server mac-winbox
set allowed-interface-list=TM_LAN
/tool mac-server ping
set enabled=no
/tool netwatch
add comment=TM_Monitor_WAN1 down-script="/ip/dns/cache/clear;\
    \_:log warning \"SASMAN: WAN1 is DOWN, DNS Cache cleared.\
    \"" host=1.1.1.1 interval=10s timeout=2s type=simple \
    up-script="/ip/dns/cache/clear; :log info \"SASMAN: WAN1 \
    is UP, DNS Cache cleared.\""
/user aaa
set use-radius=yes
[admin@RB951Ui] >  2026/06/08 17:32:22 [radius] Loaded 0 NAS secrets
2026/06/08 17:32:22 [radius] Starting Go RADIUS server on :1812 (Auth) and :1813 (Acct)...
2026/06/09 16:10:29 [radius] Loaded 1 NAS secrets
2026/06/09 16:14:18 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:14:18 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:14:18 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:55085
2026/06/09 16:14:21 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:14:21 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:14:21 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:55085
2026/06/09 16:14:24 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:14:24 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:14:24 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:55085
2026/06/09 16:14:59 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:14:59 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:14:59 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:35115
2026/06/09 16:15:02 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:15:02 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:15:02 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:35115
2026/06/09 16:15:05 [radius] Request from NAS IP: 172.17.0.1
2026/06/09 16:15:05 [radius] 🔑 طلب مصادقة جديد: يوزر [123] | من NAS: 172.17.0.1
2026/06/09 16:15:05 [radius] ❌ رفض الاتصال: يوزر [123] | السبب: كلمة المرور غير صحيحة | NAS: 172.17.0.1:35115