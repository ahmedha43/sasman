#!/bin/bash

NETWORK_FILE="/opt/sas4/etc/network.json"

#find number of addresses
N_ADDRESSES=`jq '.addresses' $NETWORK_FILE | grep 'interface' | wc -l`
if [ "$N_ADDRESSES" -eq "0" ]; then
 echo "Not IP addresses configured !"
 exit
fi
for (( N=0; N<$N_ADDRESSES; N++ ))
do
 INTERFACE=`jq ".addresses[$N].interface" $NETWORK_FILE | tr -d '"'`
 IP=`jq ".addresses[$N].ip" $NETWORK_FILE | tr -d '"'`
 NETMASK=`jq ".addresses[$N].netmask" $NETWORK_FILE | tr -d '"'`
 ifconfig $INTERFACE up
 echo "Setting up interface $INTERFACE"
 COMMAND="ifconfig $INTERFACE:$N $IP netmask $NETMASK"
 if [ "$N" -eq "0" ]; then
  COMMAND="ifconfig $INTERFACE $IP netmask $NETMASK"
 fi
 echo $COMMAND
 `$COMMAND`
done

GATEWAY=`jq ".gateway" $NETWORK_FILE | tr -d '"'`
route add default gw $GATEWAY

rm /etc/resolv.conf
touch /etc/resolv.conf
N_NAMESERVERS=`jq '.nameservers' $NETWORK_FILE |  grep '"' | wc -l`

for (( N=0; N<$N_NAMESERVERS; N++ ))
do
 DNS=`jq ".nameservers[$N]" $NETWORK_FILE | tr -d '"'`
 echo "nameserver $DNS" >> /etc/resolv.conf
done

#set hostname
HOSTNAME=`jq ".hostname" $NETWORK_FILE | tr -d '"'`
hostname $HOSTNAME

echo "Finished setting up network"
exit 0
