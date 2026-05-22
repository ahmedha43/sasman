/log info ">>> TechnoMeeM PCC V4.1 Update (V7) <<<"

# --- 1. Cleanup Old Rules ---
/system backup save name="Before_TechnoMeeM_PCC"
/export file="Before_TechnoMeeM_PCC"
/ip firewall mangle remove [find comment~"TM_"]
/ip route remove [find comment~"TM_"]
/routing rule remove [find comment~"TM_"]
/routing table remove [find name~"to_WAN"]
/ip firewall nat remove [find comment~"TM_"]
/ip address remove [find comment~"TM_"]
/ip firewall address-list remove [find comment~"TM_"]
/tool netwatch remove [find comment~"TM_"]
/routing table add name=to_WAN1 fib
/routing table add name=to_WAN2 fib
/routing table add name=to_WAN3 fib

# --- 2. Address Lists (Local Subnets) ---
/ip firewall address-list
add address=10.0.0.0/8 list=TM_Local_Subnets comment="TM_Private_10"
add address=172.16.0.0/12 list=TM_Local_Subnets comment="TM_Private_172"
add address=192.168.0.0/16 list=TM_Local_Subnets comment="TM_Private_192"

# --- 3. Interfaces & IPs ---
/ip address add address=10.0.0.1/24 interface=bridge1 comment="TM_LAN"
/ip dhcp-client add interface=ether1 disabled=no add-default-route=no use-peer-dns=no comment="TM_WAN1"
/ip dhcp-client add interface=ether2 disabled=no add-default-route=no use-peer-dns=no comment="TM_WAN2"
/interface pppoe-client add name=pppoe-out3 user=user4 password=123 interface=pppoe-out3 disabled=no add-default-route=no use-peer-dns=no comment="TM_WAN3"

# --- 4. Sticky Banking & Exceptions ---
/ip firewall address-list
add list=TM_Banks address=cibeg.com comment="TM_Sticky"
add list=TM_Banks address=ahly.net comment="TM_Sticky"
add list=TM_Banks address=fawry.com comment="TM_Sticky"

# --- 5. Mangle Rules (Input / Output / Bypass) ---
/ip firewall mangle
add chain=prerouting dst-address-type=local action=accept comment="TM_Bypass_Local"
add chain=prerouting src-address-list=TM_Local_Subnets hotspot=!auth action=accept comment="TM_Bypass_Hotspot_Unauth"
add chain=prerouting src-address-list=TM_Local_Subnets dst-address-list=TM_Local_Subnets action=accept comment="TM_Local_to_Local_Bypass"
add chain=input in-interface=ether1 action=mark-connection new-connection-mark=conn_WAN1 passthrough=yes comment="TM_Input_WAN1"
add chain=input in-interface=ether2 action=mark-connection new-connection-mark=conn_WAN2 passthrough=yes comment="TM_Input_WAN2"
add chain=input in-interface=pppoe-out3 action=mark-connection new-connection-mark=conn_WAN3 passthrough=yes comment="TM_Input_WAN3"
add chain=output connection-mark=conn_WAN1 action=mark-routing new-routing-mark=to_WAN1 passthrough=no comment="TM_Output_Router_WAN1"
add chain=output connection-mark=conn_WAN2 action=mark-routing new-routing-mark=to_WAN2 passthrough=no comment="TM_Output_Router_WAN2"
add chain=output connection-mark=conn_WAN3 action=mark-routing new-routing-mark=to_WAN3 passthrough=no comment="TM_Output_Router_WAN3"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-list=TM_Banks action=mark-connection new-connection-mark=conn_WAN1 passthrough=yes comment="TM_Sticky_Bank"

# --- 6. PCC Core Logic (Total Streams: 5) ---
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-type=!local dst-address-list=!TM_Local_Subnets per-connection-classifier=both-addresses:5/0 action=mark-connection new-connection-mark=conn_WAN1 passthrough=yes comment="TM_PCC_0_WAN1"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-type=!local dst-address-list=!TM_Local_Subnets per-connection-classifier=both-addresses:5/1 action=mark-connection new-connection-mark=conn_WAN1 passthrough=yes comment="TM_PCC_1_WAN1"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-type=!local dst-address-list=!TM_Local_Subnets per-connection-classifier=both-addresses:5/2 action=mark-connection new-connection-mark=conn_WAN2 passthrough=yes comment="TM_PCC_2_WAN2"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-type=!local dst-address-list=!TM_Local_Subnets per-connection-classifier=both-addresses:5/3 action=mark-connection new-connection-mark=conn_WAN2 passthrough=yes comment="TM_PCC_3_WAN2"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=no-mark dst-address-type=!local dst-address-list=!TM_Local_Subnets per-connection-classifier=both-addresses:5/4 action=mark-connection new-connection-mark=conn_WAN3 passthrough=yes comment="TM_PCC_4_WAN3"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=conn_WAN1 action=mark-routing new-routing-mark=to_WAN1 passthrough=no comment="TM_Route_LAN_WAN1"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=conn_WAN2 action=mark-routing new-routing-mark=to_WAN2 passthrough=no comment="TM_Route_LAN_WAN2"
add chain=prerouting src-address-list=TM_Local_Subnets connection-mark=conn_WAN3 action=mark-routing new-routing-mark=to_WAN3 passthrough=no comment="TM_Route_LAN_WAN3"

# --- 7. NAT (Internet Access) ---
/ip firewall nat add chain=srcnat out-interface=ether1 action=masquerade comment="TM_Masq_WAN1"
/ip firewall nat add chain=srcnat out-interface=ether2 action=masquerade comment="TM_Masq_WAN2"
/ip firewall nat add chain=srcnat out-interface=pppoe-out3 action=masquerade comment="TM_Masq_WAN3"

# --- 8. Routing & Flawless Failover ---
/ip route add dst-address=1.1.1.1 gateway=192.168.101.1 scope=10 check-gateway=ping comment="TM_Rec_Host_WAN1"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.1 routing-table=to_WAN1 distance=1 target-scope=12 comment="TM_Main_WAN1"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.1 routing-table=main distance=1 target-scope=12 comment="TM_Main_Route_WAN1"
/ip route add dst-address=0.0.0.0/0 gateway=192.168.103.1 routing-table=to_WAN1 check-gateway=ping distance=2 comment="TM_Backup_WAN1_via_WAN2_RealGW"
/ip route add dst-address=0.0.0.0/0 gateway=pppoe-out3 routing-table=to_WAN1 check-gateway=ping distance=3 comment="TM_Backup_WAN1_via_WAN3_RealGW"
/ip route add dst-address=1.1.1.3 gateway=192.168.103.1 scope=10 check-gateway=ping comment="TM_Rec_Host_WAN2"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.3 routing-table=to_WAN2 distance=1 target-scope=12 comment="TM_Main_WAN2"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.3 routing-table=main distance=2 target-scope=12 comment="TM_Main_Route_WAN2"
/ip route add dst-address=0.0.0.0/0 gateway=192.168.101.1 routing-table=to_WAN2 check-gateway=ping distance=2 comment="TM_Backup_WAN2_via_WAN1_RealGW"
/ip route add dst-address=0.0.0.0/0 gateway=pppoe-out3 routing-table=to_WAN2 check-gateway=ping distance=3 comment="TM_Backup_WAN2_via_WAN3_RealGW"
/ip route add dst-address=1.1.1.3 gateway=pppoe-out3 scope=10 check-gateway=ping comment="TM_Rec_Host_WAN3"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.3 routing-table=to_WAN3 distance=1 target-scope=12 comment="TM_Main_WAN3"
/ip route add dst-address=0.0.0.0/0 gateway=1.1.1.3 routing-table=main distance=3 target-scope=12 comment="TM_Main_Route_WAN3"
/ip route add dst-address=0.0.0.0/0 gateway=192.168.101.1 routing-table=to_WAN3 check-gateway=ping distance=2 comment="TM_Backup_WAN3_via_WAN1_RealGW"
/ip route add dst-address=0.0.0.0/0 gateway=192.168.103.1 routing-table=to_WAN3 check-gateway=ping distance=3 comment="TM_Backup_WAN3_via_WAN2_RealGW"

# --- 9. Route Rules (Local DNS & Routing Fix) ---
/routing rule
add action=lookup-only-in-table dst-address=10.0.0.0/8 table=main comment="TM_Rule_LAN_10"
add action=lookup-only-in-table dst-address=172.16.0.0/12 table=main comment="TM_Rule_LAN_172"
add action=lookup-only-in-table dst-address=192.168.0.0/16 table=main comment="TM_Rule_LAN_192"

# --- 10. DNS & DHCP Services ---
/ip dns set allow-remote-requests=yes servers=8.8.8.8,1.0.0.1
/ip pool add name=tm_pool ranges=10.0.0.10-10.0.0.254
/ip dhcp-server add name=dhcp1 interface=bridge1 address-pool=tm_pool disabled=no lease-time=1h
/ip dhcp-server network add address=10.0.0.0/24 gateway=10.0.0.1 dns-server=8.8.8.8,1.0.0.1

# --- 11. Netwatch & Per-WAN Connection Flush ---
/tool netwatch add host=1.1.1.1 interval=10s timeout=2s down-script=":log warning \"TM: WAN1 Down - Flushing connections\"; /ip firewall connection remove [find connection-mark=\"conn_WAN1\"]" up-script=":log info \"TM: WAN1 Restored\"" comment="TM_Monitor_WAN1"
/tool netwatch add host=1.1.1.3 interval=10s timeout=2s down-script=":log warning \"TM: WAN2 Down - Flushing connections\"; /ip firewall connection remove [find connection-mark=\"conn_WAN2\"]" up-script=":log info \"TM: WAN2 Restored\"" comment="TM_Monitor_WAN2"
/tool netwatch add host=1.1.1.3 interval=10s timeout=2s down-script=":log warning \"TM: WAN3 Down - Flushing connections\"; /ip firewall connection remove [find connection-mark=\"conn_WAN3\"]" up-script=":log info \"TM: WAN3 Restored\"" comment="TM_Monitor_WAN3"

/log info ">>> Generation Complete (V4.1) <<<"
