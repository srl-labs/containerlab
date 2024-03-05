#!/bin/bash

# apply nvue config in background
setsid ./root/apply-nvue.sh > /root/nvue.log &

# run systemd services
exec /sbin/init
