#!/bin/bash
if (($1==1))
then
echo "断开 $2"
iptables -A OUTPUT -d $2 -j REJECT
else
iptables -D OUTPUT -d $2 -j REJECT
fi
