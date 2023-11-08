#!/bin/sh
# start the ssh daemon
twistd -n ftp -p 21 -r /root/ &
/usr/sbin/sshd &
sleep infinity