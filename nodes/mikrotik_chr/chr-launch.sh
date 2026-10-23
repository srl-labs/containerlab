#!/bin/sh
# chr-launch.sh: run the OFFICIAL mikrotik/chr image (MikroTik RouterOS Cloud
# Hosted Router) as a containerlab node, image untouched.
#
# The image is MikroTik's own Docker Hub distribution (Alpine + qemu +
# /diskimage/chr-<ver>.img + /start.sh). This script is bind-mounted into the
# node and set as its entrypoint by the containerlab `mikrotik_chr` kind. It replaces
# the image's /start.sh, which is not orchestration-ready (containerlab#3116):
# it scans interfaces once before containerlab attaches the links, bridges the
# management NIC so the container IP never reaches the router, and imports no
# startup config.
#
# What it does (vrnetlab's mikrotik/routeros launch.py is the model, re-done
# with the image's own busybox + qemu: no python, no ftp client, no tc):
#   1. waits for the CLAB_INTFS data interfaces containerlab attaches (eth1..N)
#   2. management, transparent: the container's eth0 is bridged with the VM's
#      ether1 tap, and eth0's addresses and default gateways move into RouterOS
#      ether1. The router IS the node's management address: every service and
#      LLDP reach it directly, nothing is port-forwarded. The image ships only
#      busybox (no tc), so this uses a bridge where vrnetlab uses tc-mirred
#   3. data: one tap per container interface, bridged pairwise (ethN <-> tapN,
#      LLDP forwarded). NIC order in the VM follows the eth index, so container
#      ethN is always RouterOS ether(N+1), even with gaps
#   4. after boot, over IPv6 link-local on the management bridge (no address is
#      added to the management network): sets the credentials and the identity,
#      configures ether1 with the management addresses and keeps their return
#      path in its own routing table (`mgmt`, below), then fetches
#      /ftpboot/config.auto.rsc (the containerlab startup-config, when present)
#      from the launcher and `/import`s it
#   5. prints the ready marker on the container log and waits on qemu
#
# Restart (docker restart, containerlab restart --node): RouterOS keys an
# ethernet interface on its MAC. A NIC that comes back with a new MAC is a NEW
# device (ether1 becomes ether3) and the DHCP client bound to ether1 never
# binds. Docker re-rolls the container's eth0 MAC whenever the netns is
# recreated, so the node id that seeds every NIC MAC and the data NIC count are
# persisted in the container on the first boot and reused: the restarted qemu
# carries the same NICs in the same order and RouterOS keeps ether1..N and
# their config. The REST bootstrap is skipped on a restart; the management
# step re-runs and re-applies the container's current addressing.
#
# Architectures: the image's own qemu (qemu-system-x86_64 on amd64,
#   qemu-system-aarch64 + edk2 on arm64), KVM when /dev/kvm exists, else TCG.
# Args (the kind's Cmd): --hostname H [--username U --password P]
#   (unknown flags are ignored); credentials default to the USERNAME/PASSWORD env
# Env (all optional, set under the node's `env:`); MEM, CPUS and DISK are the
#   official image's own variables:
#   MEM (qemu -m value, default 256 MiB + 8 MiB per data NIC and vCPU), CPUS (default 1),
#   DISK (default 128M), CHR_INTF_WAIT (s, 60), CHR_BOOT_WAIT (s, 300)
# Serial console: port 5000 inside the container
#   (docker exec -it <node> nc 127.0.0.1 5000); the container itself has no
#   address on the management network.
# Ready: /.chr-launch-ready exists once RouterOS is ready (the kind's default
#   healthcheck); it is removed at every start.
# Log lines (docker logs clab-<lab>-<node>), all prefixed "chr-launch:":
#   "RouterOS ready"                   login + startup config done
#   "startup config import FAILED: …"  the router rejected the .rsc (RouterOS's
#                                      own reason, with the line number)
#   "credential bootstrap FAILED: …"   the user/password step failed
#   "management setup FAILED: …"       the ether1 address / mgmt table step failed
#
# Copyright 2026 NetPilot Labs
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

set -u

# Credentials come from the USERNAME/PASSWORD env the kind sets (no quoting
# limits on the password); --username/--password still override them.
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD-admin}"
HOSTNAME_ARG=""
while [ $# -gt 0 ]; do
    case "$1" in
        --username) USERNAME="$2"; shift 2 ;;
        --password) PASSWORD="$2"; shift 2 ;;
        --hostname) HOSTNAME_ARG="$2"; shift 2 ;;
        --connection-mode) shift 2 ;;
        --trace) shift ;;
        *) shift ;;
    esac
done
[ -n "$HOSTNAME_ARG" ] || HOSTNAME_ARG="$(hostname)"

CPUS="${CPUS:-1}"
DISK="${DISK:-128M}"
INTF_WAIT="${CHR_INTF_WAIT:-60}"
BOOT_WAIT="${CHR_BOOT_WAIT:-300}"
INTFS="${CLAB_INTFS:-0}"
STARTUP_CFG=/ftpboot/config.auto.rsc
# Per-container state in the rw layer: survives every restart, gone with the
# container (destroy, redeploy --cleanup).
DONE_MARKER=/.chr-launch-bootstrapped
READY_FILE=/.chr-launch-ready
ID_FILE=/.chr-launch-node-id
NICS_FILE=/.chr-launch-nics
MGMT_TABLE=mgmt
# every management object the launcher creates carries this comment, so each
# boot can find and re-apply them, and SaveConfig can leave them out
MGMT_COMMENT=clab-mgmt
MGMT_BR=br0
# eth0's addressing as last seen: a launcher re-run in the same network
# namespace finds eth0 already bridged and bare
MGMT_FILE=/.chr-launch-mgmt
SYSNET=/sys/class/net

log() { echo "chr-launch: $*"; }
rm -f "$READY_FILE"

FETCH_PORT=8080

# ---- disk -----------------------------------------------------------------
IMG=""
for f in /diskimage/*.img; do [ -f "$f" ] && IMG="$f" && break; done
if [ -z "$IMG" ]; then
    log "no /diskimage/*.img in this image: not a RouterOS CHR image"
    exit 1
fi
# Only GROW the disk to the floor: an image already larger keeps its tail.
disk_bytes() {
    case "$1" in
        *G|*g) echo $(( ${1%[Gg]} * 1024 * 1024 * 1024 )) ;;
        *M|*m) echo $(( ${1%[Mm]} * 1024 * 1024 )) ;;
        *K|*k) echo $(( ${1%[Kk]} * 1024 )) ;;
        *) echo "$1" ;;
    esac
}
want_bytes="$(disk_bytes "$DISK")"
cur_bytes="$(stat -c %s "$IMG" 2>/dev/null || wc -c < "$IMG")"
if [ "$cur_bytes" -lt "$want_bytes" ] 2>/dev/null; then
    truncate -s "$DISK" "$IMG"
fi

# ---- wait for containerlab's data interfaces ------------------------------
# shellcheck disable=SC2012  # interface names are ethN, ls is safe here
count_intfs() { ls -d "$SYSNET"/eth[1-9]* 2>/dev/null | wc -l; }
waited=0
while [ "$(count_intfs)" -lt "$INTFS" ] && [ "$waited" -lt "$INTF_WAIT" ]; do
    sleep 1
    waited=$((waited + 1))
done
log "interfaces: $(count_intfs) of $INTFS expected after ${waited}s"

# ---- qemu command line ----------------------------------------------------
# The image's own qemu and firmware, as its /start.sh selects them.
case "$(uname -m)" in
    aarch64|arm64)
        QEMU="qemu-system-aarch64 -M virt -pflash /usr/share/qemu/edk2-aarch64-code.fd"
        PCI_BRIDGES=0
        TCG_CPU="-cpu cortex-a57" ;;
    *)
        QEMU="qemu-system-x86_64"
        PCI_BRIDGES=1
        TCG_CPU="" ;;
esac
if [ -e /dev/kvm ]; then
    ACCEL="-cpu host -accel kvm"
else
    ACCEL="$TCG_CPU -accel tcg,split-wx=on"
    log "no /dev/kvm: running under TCG emulation (slow boot)"
fi

# MACs: unique per node AND per NIC. The container's eth0 MAC is unique per
# node, so its LAST TWO octets identify the node; the NIC's last byte is the
# interface index (00 = mgmt, 01 = ether2, …). Two nodes sharing a NIC MAC on
# a link drop each other's frames as their own. The id is read on the FIRST
# boot only and persisted (see Restart in the header).
node_id="$(cat "$ID_FILE" 2>/dev/null)"
case "$node_id" in
    [0-9a-f][0-9a-f]:[0-9a-f][0-9a-f]) ;;
    *)
        e="$(cat "$SYSNET/eth0/address" 2>/dev/null)"
        if [ -n "$e" ]; then
            o2="${e##*:}"; rest_mac="${e%:*}"; o1="${rest_mac##*:}"
            node_id="$o1:$o2"
        else
            node_id="$(hexdump -n2 -e '2/1 "%02x:"' /dev/urandom | sed 's/:$//')"
        fi
        printf '%s\n' "$node_id" > "$ID_FILE" 2>/dev/null \
            || log "node id $node_id not persisted: a restart will renumber the NICs"
        ;;
esac
NET="-netdev tap,id=mgmt,ifname=tap0,script=no,downscript=no -device virtio-net,netdev=mgmt,mac=52:54:00:$node_id:00"

check_nic_index() { # check_nic_index <max-index>: reject MAC-address wraparound
    # The last MAC octet holds the NIC index; zero belongs to management.
    if [ "$1" -gt 255 ]; then
        log "at most 255 data NICs are supported (eth255 / ether256); index $1 would reuse a NIC MAC"
        return 1
    fi
}

qemu_data_nic() { # qemu_data_nic <index> <mac>: append-only QEMU arguments
    case "$PCI_BRIDGES" in
        1)
            # The default PC has only 29 free PCI slots, including management.
            # Keep management on the root bus and put data NICs behind bridges.
            # Follow vrnetlab's conservative allocation of 25 NICs per bridge.
            _bridge=$(( ($1 - 1) / 25 + 1 ))
            _slot=$(( ($1 - 1) % 25 + 1 ))
            if [ "$_slot" -eq 1 ]; then
                printf '%s ' "-device pci-bridge,id=chrpci$_bridge,chassis_nr=$_bridge,bus=pci.0"
            fi
            # Omit PXE ROMs to avoid exhausting SeaBIOS option-ROM space.
            # Keep MSI-X, but share one vector between RX/TX queues (plus one
            # for config) to reduce IRQ pressure.
            # Disabling MSI-X entirely can leave NICs missing in RouterOS.
            printf '%s' "-netdev tap,id=d$1,ifname=tap$1,script=no,downscript=no -device virtio-net,netdev=d$1,mac=$2,bus=chrpci$_bridge,addr=$(printf '0x%x' "$_slot"),romfile=,vectors=2"
            ;;
        *)
            # ARM64 uses the virt machine and keeps its existing NIC layout.
            printf '%s' "-netdev tap,id=d$1,ifname=tap$1,script=no,downscript=no -device virtio-net,netdev=d$1,mac=$2"
            ;;
    esac
}

# ---- management: eth0 + tap0 on one bridge, eth0's addressing to the VM ----
ip_global() { # ip_global <4|6> <dev>: the first global address/prefix
    ip -"$1" addr show dev "$2" 2>/dev/null \
        | sed -n '/scope global/s/^ *inet6\{0,1\} \([^ ]*\) .*/\1/p' | head -n1
}
ip_gw() { # ip_gw <4|6>: the default gateway
    ip -"$1" route show default 2>/dev/null | sed -n 's/^default via \([^ ]*\) .*/\1/p' | head -n1
}
M4="$(ip_global 4 eth0)"; M6="$(ip_global 6 eth0)"
G4="$(ip_gw 4)"; G6="$(ip_gw 6)"
if [ -n "$M4$M6" ]; then
    printf 'M4=%s\nM6=%s\nG4=%s\nG6=%s\n' "$M4" "$M6" "$G4" "$G6" > "$MGMT_FILE" 2>/dev/null || true
elif [ -f "$MGMT_FILE" ]; then
    # shellcheck disable=SC1090  # written by this script, four plain assignments
    . "$MGMT_FILE"
fi
log "management: ${M4:-no IPv4}${M6:+, $M6} via ${G4:-no gateway}${G6:+, $G6}"

tunctl -t tap0 >/dev/null 2>&1 || ip tuntap add dev tap0 mode tap
if [ ! -e "$SYSNET/$MGMT_BR" ]; then
    _mtu="$(cat "$SYSNET/eth0/mtu" 2>/dev/null || echo 1500)"
    ip addr flush dev eth0
    # The launcher's side of the bootstrap channel: IPv6 link-local on the
    # bridge only. eth0 and tap0 are plain bridge ports.
    sysctl -qw net.ipv6.conf.default.disable_ipv6=0 net.ipv6.conf.default.accept_ra=0 2>/dev/null
    brctl addbr "$MGMT_BR"
    brctl setfd "$MGMT_BR" 0 2>/dev/null
    echo 16384 > "$SYSNET/$MGMT_BR/bridge/group_fwd_mask" 2>/dev/null
    ip link set "$MGMT_BR" address "52:54:01:$node_id:00"
    ip link set tap0 mtu "$_mtu"
    ip link set "$MGMT_BR" mtu "$_mtu"
    # eth0 joins the bridge only once ether1 is configured (mgmt_attach): until
    # then nothing the router sends (its boot-time router advertisements, CHR's
    # DHCP discovers) reaches the management network.
    brctl addif "$MGMT_BR" tap0
    sysctl -qw net.ipv6.conf.eth0.disable_ipv6=1 net.ipv6.conf.tap0.disable_ipv6=1 \
        "net.ipv6.conf.$MGMT_BR.disable_ipv6=0" "net.ipv6.conf.$MGMT_BR.accept_ra=0" \
        "net.ipv6.conf.$MGMT_BR.router_solicitations=0" 2>/dev/null
fi
ip link set tap0 up
ip link set "$MGMT_BR" up
# RouterOS's ether1 link-local address, from the MAC this script gives it
VM_LL="fe80::5054:ff:fe${node_id%%:*}:${node_id##*:}00"
VM_EP="$VM_LL%$MGMT_BR"
# The bridge's own link-local address (the startup-config fetch source). Read
# once qemu runs: until then tap0, the bridge's only port, has no carrier and
# the address stays tentative.
LL=""
bridge_ll() {
    _w=0
    while [ -z "$LL" ] && [ "$_w" -lt 20 ]; do
        LL="$(ip -6 addr show dev "$MGMT_BR" 2>/dev/null | grep -v tentative \
            | sed -n 's/^ *inet6 \(fe80::[^/]*\)\/64 .*/\1/p' | head -n1)"
        [ -n "$LL" ] || { sleep 1; _w=$((_w + 1)); }
    done
    [ -n "$LL" ] || log "no IPv6 link-local on $MGMT_BR"
}

# NIC order = PCI order = the order of the -device args below. Walk
# eth1..ethMAX so a gap (links on ether2 and ether5 only) keeps RouterOS
# numbering aligned: a missing ethN still gets a tap, just no bridge. The count
# never shrinks across restarts: a `docker restart` recreates the netns and
# every lab veth of the node is gone, and a NIC absent for one boot loses the
# config bound to it. The tap keeps ether(N+1) a real interface, its link comes
# back when containerlab restores the veth or the lab is redeployed.
max=0
for d in "$SYSNET"/eth[1-9]*; do
    [ -e "$d" ] || continue
    n="${d##*/eth}"
    [ "$n" -gt "$max" ] && max="$n"
done
prev="$(cat "$NICS_FILE" 2>/dev/null)"
case "$prev" in *[!0-9]*|"") prev=0 ;; esac
[ "$prev" -gt "$max" ] && max="$prev"
# The wait above can end short of CLAB_INTFS on a large lab (links attach after
# every container started): the NIC set still spans the expected count, and
# the watcher bridges the late veths.
case "$INTFS" in *[!0-9]*|"") INTFS=0 ;; esac
[ "$INTFS" -gt "$max" ] && max="$INTFS"
check_nic_index "$max" || exit 1
[ "$max" -gt "$prev" ] && { printf '%s\n' "$max" > "$NICS_FILE" 2>/dev/null || true; }

# RouterOS v7 memory ≈ 256 + 8·CPUS·(ifaces) MiB (MikroTik CHR system
# requirements). Scale by the HIGHEST generated NIC, not CLAB_INTFS: a sparse
# link on ether64 means CLAB_INTFS=1 yet 63 data NICs are created below.
if [ -z "${MEM:-}" ]; then
    MEM=$((256 + 8 * CPUS * max))
fi

bridge_eth() { # bridge_eth <i>: container ethI joins brI beside tapI (brI made once)
    _mtu="$(cat "$SYSNET/eth$1/mtu" 2>/dev/null || echo 1500)"
    if [ ! -e "$SYSNET/br$1" ]; then
        brctl addbr "br$1"
        ip link set "br$1" mtu "$_mtu"
        ip link set "tap$1" mtu "$_mtu"
        # forward LLDP (01:80:c2:00:00:0e, bit 14); STP/LACP cannot cross a bridge
        echo 16384 > "$SYSNET/br$1/bridge/group_fwd_mask" 2>/dev/null
        brctl addif "br$1" "tap$1"
    fi
    brctl addif "br$1" "eth$1"
    ip link set "eth$1" up
    ip link set "br$1" up
}
# A veth that lands or returns while qemu runs (containerlab restores a link
# right after a restart --node, a large lab attaches late, a peer node's
# redeploy recreates the pair) joins its bridge as it appears, so the RouterOS
# interface behind tapN gets its link back without a restart here.
watch_links() {
    _warned=""
    while kill -0 "$QEMU_PID" 2>/dev/null; do
        i=1
        while [ "$i" -le "$max" ]; do
            if [ -e "$SYSNET/eth$i" ] && [ ! -e "$SYSNET/br$i/brif/eth$i" ]; then
                bridge_eth "$i" && log "link eth$i attached: bridged to tap$i"
            fi
            i=$((i + 1))
        done
        for d in "$SYSNET"/eth[1-9]*; do
            [ -e "$d" ] || continue
            n="${d##*/eth}"
            [ "$n" -gt "$max" ] || continue
            case " $_warned " in *" $n "*) continue ;; esac
            _warned="$_warned $n"
            log "link eth$n attached after qemu started with $max data NICs: no RouterOS interface for it until the node is restarted"
        done
        sleep 2
    done
}

i=1
while [ "$i" -le "$max" ]; do
    tap="tap$i"
    tunctl -t "$tap" >/dev/null 2>&1 || ip tuntap add dev "$tap" mode tap
    [ -e "$SYSNET/eth$i" ] && bridge_eth "$i"
    ip link set "$tap" up
    mac="$(printf '52:54:00:%s:%02x' "$node_id" "$i")"
    NET="$NET $(qemu_data_nic "$i" "$mac")"
    i=$((i + 1))
done

# shellcheck disable=SC2086
$QEMU $ACCEL -display none -monitor none \
    -m "$MEM" -smp "cpus=$CPUS" \
    -drive "file=$IMG,format=raw" \
    -serial telnet:0.0.0.0:5000,server,nowait \
    $NET &
QEMU_PID=$!
trap 'kill "$QEMU_PID" 2>/dev/null; wait "$QEMU_PID" 2>/dev/null; exit 0' TERM INT
log "qemu started (pid $QEMU_PID, mem ${MEM}M, cpus $CPUS, data nics $max)"
watch_links &

# ---- REST helpers ---------------------------------------------------------
json_str() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }
rest() { # rest <user:pass> <path> [json]
    # Basic auth in a header, never in the URL: a password containing a URI
    # delimiter (#, ?, /) would change where the authority ends. REST_T is the
    # read timeout (default 20s).
    _auth="Authorization: Basic $(printf '%s' "$1" | base64 | tr -d '\n')"
    if [ $# -ge 3 ]; then
        wget -q -O- -T "${REST_T:-20}" --header "Content-Type: application/json" \
            --header "$_auth" --post-data "$3" "http://[$VM_EP]/rest/$2" 2>/dev/null
    else
        wget -q -O- -T "${REST_T:-20}" --header "$_auth" "http://[$VM_EP]/rest/$2" 2>/dev/null
    fi
}
# busybox wget discards the body of an HTTP error response, and RouterOS puts
# its diagnosis ONLY there ({"detail":"Script Error: … (line N column M)"}; its
# /log stays silent). So the calls whose failure must be readable go over a raw
# HTTP POST through busybox nc. busybox nc exits the moment stdin hits EOF,
# reply or not, and RouterOS keeps the connection open despite
# `Connection: close`, so the request side is held open on a fifo and closed
# once Content-Length bytes of body have arrived.
resp_complete() { # resp_complete <file>: headers + Content-Length bytes of body arrived
    _cl="$(grep -i -m1 '^Content-Length:' "$1" 2>/dev/null | tr -dc '0-9')"
    [ -n "$_cl" ] || return 1
    grep -q -m1 '^\r*$' "$1" 2>/dev/null || return 1
    [ "$(sed '1,/^\r*$/d' "$1" | wc -c | tr -d ' ')" -ge "$_cl" ]
}
rest_raw() { # rest_raw <user:pass> <path> <json> <timeout-s>  -> "<status> <body>"
    _f="/tmp/chr-rest.$$"; rm -f "$_f" "$_f.out"; mkfifo "$_f" || return 1
    timeout "$(( $4 + 5 ))" nc "$VM_EP" 80 <"$_f" >"$_f.out" 2>/dev/null &
    _pid=$!
    exec 7>"$_f"
    printf 'POST /rest/%s HTTP/1.1\r\nHost: [%s]\r\nAuthorization: Basic %s\r\nContent-Type: application/json\r\nContent-Length: %s\r\nConnection: close\r\n\r\n%s' \
        "$2" "$VM_LL" "$(printf '%s' "$1" | base64 | tr -d '\n')" \
        "$(printf '%s' "$3" | wc -c | tr -d ' ')" "$3" >&7
    _left=$4
    while kill -0 "$_pid" 2>/dev/null && ! resp_complete "$_f.out" && [ "$_left" -gt 0 ]; do
        sleep 1; _left=$(( _left - 1 ))
    done
    exec 7>&-
    wait "$_pid" 2>/dev/null
    _st="$(head -n1 "$_f.out" 2>/dev/null | tr -d '\r' | cut -d' ' -f2)"
    _body="$(sed '1,/^\r*$/d' "$_f.out" 2>/dev/null | tr -d '\r')"
    rm -f "$_f" "$_f.out"
    printf '%s %s' "${_st:-000}" "$_body"
}
# RouterOS REST returns the object/array on success and an empty body for a
# bare `set`: only an explicit error object is a failure here. A silently
# failed call is caught by the read-back checks that follow each step.
is_err() { case "$1" in *'"error"'*) return 0 ;; *) return 1 ;; esac; }

# ---- wait for RouterOS ----------------------------------------------------
waited=0
AUTH=""
# A fresh official CHR disk has ONLY the factory `admin` account (blank
# password): the first boot authenticates as that account whatever --username
# asks for, and the requested user is created below. On a restart the pair that
# answers is the requested user.
while [ -z "$AUTH" ]; do
    for _cred in "admin:" "admin:$PASSWORD" "$USERNAME:$PASSWORD"; do
        rest "$_cred" system/resource >/dev/null 2>&1 && { AUTH="$_cred"; break; }
    done
    [ -n "$AUTH" ] && break
    if ! kill -0 "$QEMU_PID" 2>/dev/null; then
        log "qemu exited during boot"
        exit 1
    fi
    sleep 2
    waited=$((waited + 2))
    if [ "$waited" -ge "$BOOT_WAIT" ]; then
        log "RouterOS did not answer within ${BOOT_WAIT}s: leaving qemu running"
        wait "$QEMU_PID"
        exit 1
    fi
done
log "RouterOS answered after ${waited}s"

# Ready means a user can log in: RouterOS starts its SSH server well after REST
# answers on a slow boot (about 40 s later under TCG on arm64), so when the
# configuration keeps SSH enabled, its port must send the banner first.
# A config that disables SSH is not waited for; a banner that never comes is
# logged and the node is still declared ready (REST works).
ssh_banner() { (sleep 3) | nc "$VM_EP" 22 2>/dev/null | head -c 7; }
ready() {
    case "$(rest "$AUTH" "ip/service?name=ssh")" in
        *'"disabled":"false"'*)
            _t0=$(date +%s)
            until [ "$(ssh_banner)" = "SSH-2.0" ]; do
                if [ $(( $(date +%s) - _t0 )) -ge "$BOOT_WAIT" ]; then
                    log "SSH did not answer within $(( $(date +%s) - _t0 ))s (REST works)"
                    break
                fi
                sleep 2
            done
            ;;
    esac
    touch "$READY_FILE" 2>/dev/null || true
    log "RouterOS ready"
}

# A failed step WITHHOLDS the ready marker (qemu keeps running, so the node can
# still be inspected over the serial console, port 5000 in the container).
bootstrap_fail() {
    log "credential bootstrap FAILED: $1"
    wait "$QEMU_PID"
    exit 0
}

# ---- management: ether1 addressing and its own routing table ----------------
# ether1 gets the container's management addresses, and their return path lives
# in a `mgmt` routing table selected by a rule on those source addresses: a
# default route the lab adds to `main` (a static 0.0.0.0/0, a PPPoE client's
# default route, a dual-WAN pair) would otherwise take SSH/REST replies away
# from the management network while the data plane works. CHR's default ether1
# DHCP client is disabled: the address is static. Everything is tagged
# clab-mgmt and re-applied on every boot from the container's CURRENT addressing
# (a restart can bring a new IP), while the bootstrap channel itself
# (link-local) never depends on it.
mgmt_fail() {
    log "management setup FAILED: $1 (redeploy the node: containerlab deploy --reconfigure)"
    wait "$QEMU_PID"
    exit 0
}
rest_has() { # rest_has <path?query>: the GET answered a non-empty JSON array
    case "$(rest "$AUTH" "$1")" in '['*'{'*) return 0 ;; *) return 1 ;; esac
}
rest_add() { # rest_add <what> <path> <json>: a 2xx with no error object, else mgmt_fail with the router's detail
    _r="$(rest_raw "$AUTH" "$2" "$3" 20)"
    case "$_r" in
        2??\ *) is_err "$_r" && mgmt_fail "$1: ${_r#* }" ;;
        *) mgmt_fail "$1: ${_r:-no response}" ;;
    esac
}
rest_count() { # rest_count <path?query>: number of objects the GET answered
    rest "$AUTH" "$1" | tr '{' '\n' | grep -c '"\.id"'
}
rest_clear() { # rest_clear <menu>: remove its clab-mgmt objects
    _ids="$(rest "$AUTH" "$1?comment=$MGMT_COMMENT" | tr '{' '\n' \
        | sed -n 's/.*"\.id":"\([^"]*\)".*/\1/p' | paste -sd, -)"
    [ -z "$_ids" ] || rest_add "remove old $1" "$1/remove" "{\".id\":\"$_ids\"}"
}
# The tagged objects already match the container's addressing (a restart with
# the same IP): leave them alone, re-applying would only drop live sessions.
mgmt_in_place() {
    _want=0
    if [ -n "$M4" ]; then
        rest_has "ip/address?interface=ether1&address=$M4&comment=$MGMT_COMMENT" || return 1
        rest_has "routing/rule?src-address=${M4%/*}/32&table=$MGMT_TABLE" || return 1
        [ -z "$G4" ] || rest_has "ip/route?routing-table=$MGMT_TABLE&dst-address=0.0.0.0/0&gateway=$G4" || return 1
        _want=$((_want + 1))
    fi
    if [ -n "$M6" ]; then
        rest_has "ipv6/address?interface=ether1&address=$M6&comment=$MGMT_COMMENT" || return 1
        _want=$((_want + 1))
    fi
    [ "$(rest_count "ip/address?comment=$MGMT_COMMENT")$(rest_count "ipv6/address?comment=$MGMT_COMMENT")" \
        = "$([ -n "$M4" ] && echo 1 || echo 0)$([ -n "$M6" ] && echo 1 || echo 0)" ] || return 1
    rest_has "ipv6/nd?interface=ether1&ra-lifetime=none" || return 1
    [ "$_want" -gt 0 ]
}
mgmt_setup() {
    if mgmt_in_place; then
        log "management on ether1 unchanged: ${M4:-no IPv4}${M6:+, $M6}"
        return 0
    fi
    for _m in routing/rule ip/route ipv6/route ip/address ipv6/address; do rest_clear "$_m"; done
    rest_has "routing/table?name=$MGMT_TABLE" \
        || rest_add "routing table $MGMT_TABLE" routing/table/add \
            "{\"name\":\"$MGMT_TABLE\",\"fib\":\"yes\",\"comment\":\"$MGMT_COMMENT\"}"
    _c="\"comment\":\"$MGMT_COMMENT\""
    if [ -n "$M4" ]; then
        _net4="$(ipcalc -n "$M4" | sed -n 's/^NETWORK=//p')/${M4#*/}"
        rest_add "ether1 address $M4" ip/address/add "{\"address\":\"$M4\",\"interface\":\"ether1\",$_c}"
        rest_add "route $_net4 in table $MGMT_TABLE" ip/route/add \
            "{\"dst-address\":\"$_net4\",\"gateway\":\"ether1\",\"routing-table\":\"$MGMT_TABLE\",$_c}"
        [ -z "$G4" ] || rest_add "default via $G4 in table $MGMT_TABLE" ip/route/add \
            "{\"dst-address\":\"0.0.0.0/0\",\"gateway\":\"$G4\",\"routing-table\":\"$MGMT_TABLE\",$_c}"
        rest_add "routing rule for ${M4%/*}" routing/rule/add \
            "{\"src-address\":\"${M4%/*}/32\",\"action\":\"lookup-only-in-table\",\"table\":\"$MGMT_TABLE\",$_c}"
    fi
    if [ -n "$M6" ]; then
        rest_add "ether1 address $M6" ipv6/address/add \
            "{\"address\":\"$M6\",\"interface\":\"ether1\",\"advertise\":\"no\",$_c}"
        # RouterOS normalizes the host bits of dst-address (::3/64 -> ::/64)
        rest_add "IPv6 route $M6 in table $MGMT_TABLE" ipv6/route/add \
            "{\"dst-address\":\"$M6\",\"gateway\":\"ether1\",\"routing-table\":\"$MGMT_TABLE\",$_c}"
        [ -z "$G6" ] || rest_add "IPv6 default via $G6 in table $MGMT_TABLE" ipv6/route/add \
            "{\"dst-address\":\"::/0\",\"gateway\":\"$G6%ether1\",\"routing-table\":\"$MGMT_TABLE\",$_c}"
        rest_add "routing rule for ${M6%/*}" routing/rule/add \
            "{\"src-address\":\"${M6%/*}/128\",\"action\":\"lookup-only-in-table\",\"table\":\"$MGMT_TABLE\",$_c}"
    fi
    _id="$(rest "$AUTH" "ip/dhcp-client?interface=ether1" | sed -n 's/.*"\.id":"\([^"]*\)".*/\1/p' | head -n1)"
    [ -z "$_id" ] || rest_add "disable the ether1 DHCP client" ip/dhcp-client/set \
        "{\"numbers\":\"$_id\",\"disabled\":\"yes\"}"
    # RouterOS's default ND entry (interface=all, ra-lifetime=30m) sends router
    # advertisements on ether1 too, and every host on the management network
    # with accept_ra would install the router as its IPv6 default route. An
    # ether1 entry with ra-lifetime=none keeps the RAs harmless and withdraws a
    # route a host already learned (a disabled entry does not override "all").
    # /ipv6/nd keeps no comment, so its entry is found by interface.
    _nd="$(rest "$AUTH" "ipv6/nd?interface=ether1" | sed -n 's/.*"\.id":"\([^"]*\)".*/\1/p' | head -n1)"
    if [ -n "$_nd" ]; then
        rest_add "ether1 router advertisements off" ipv6/nd/set \
            "{\"numbers\":\"$_nd\",\"ra-lifetime\":\"none\",\"disabled\":\"no\"}"
    else
        rest_add "ether1 router advertisements off" ipv6/nd/add \
            "{\"interface\":\"ether1\",\"ra-lifetime\":\"none\"}"
    fi
    # Prove they took (a transport flake answers an empty body, which is not
    # an error by the set-contract).
    if [ -n "$M4" ]; then
        rest_has "ip/address?interface=ether1&address=$M4" || mgmt_fail "$M4 not on ether1"
        rest_has "routing/rule?src-address=${M4%/*}/32&table=$MGMT_TABLE" \
            || mgmt_fail "routing rule for ${M4%/*} not applied"
    fi
    log "management on ether1: ${M4:-no IPv4}${M6:+, $M6}, return path in routing table $MGMT_TABLE"
}
# Join the management network, then have the router reach its gateway from the
# management address: proves the return path both ways, and the ARP/ND request
# refreshes the gateway's neighbor entry when the IP was used by another MAC
# before (a redeployed lab hands out the same addresses).
mgmt_attach() {
    [ -e "$SYSNET/$MGMT_BR/brif/eth0" ] || brctl addif "$MGMT_BR" eth0
    ip link set eth0 up
    for _pair in "$G4 ${M4%/*}" "$G6 ${M6%/*}"; do
        # shellcheck disable=SC2086  # split "gateway source"; an empty half skips the pair
        set -- $_pair
        [ $# -eq 2 ] || continue
        _t0=$(date +%s)
        until case "$(rest "$AUTH" ping "{\"address\":\"$1\",\"src-address\":\"$2\",\"count\":\"2\"}")" in
                *'"received":"'[1-9]*) true ;; *) false ;; esac; do
            if [ $(( $(date +%s) - _t0 )) -ge 30 ]; then
                log "gateway $1 does not answer from $2 after $(( $(date +%s) - _t0 ))s (continuing)"
                break
            fi
            sleep 1
        done
    done
}

# Bootstrap + startup-config import happen ONCE per container lifetime. On a
# restart the CHR disk keeps its config and the marker (in the container rw
# layer) survives, so the bootstrap (which could wait forever if the config
# disabled `www` or changed the account, or reject duplicate `add` lines) is
# skipped. A fresh deploy recreates the container and the disk, so the marker
# and the config are both gone and the bootstrap runs.
if [ -f "$DONE_MARKER" ]; then
    log "already bootstrapped (restart): skipping config"
    mgmt_setup
    mgmt_attach
    ready
    wait "$QEMU_PID"
    exit 0
fi

# ---- credentials + identity -----------------------------------------------
# Every credential-establishing call is CHECKED: the ready marker never
# advertises credentials that do not work.
FACTORY="admin:"
rest "$FACTORY" system/resource >/dev/null 2>&1 || FACTORY="admin:$PASSWORD"
if [ "$USERNAME" = "admin" ]; then
    if [ -n "$PASSWORD" ]; then
        out="$(rest "$FACTORY" user/set \
            "{\"numbers\":\"admin\",\"password\":\"$(json_str "$PASSWORD")\"}")"
        is_err "$out" && bootstrap_fail "set admin password: ${out:-no response}"
    fi
    AUTH="admin:$PASSWORD"
else
    out="$(rest "$FACTORY" user/add \
        "{\"name\":\"$(json_str "$USERNAME")\",\"password\":\"$(json_str "$PASSWORD")\",\"group\":\"full\"}")"
    is_err "$out" && bootstrap_fail "create user $USERNAME: ${out:-no response}"
    AUTH="$USERNAME:$PASSWORD"
    # The factory `admin` still has a BLANK password and is reachable over the
    # forwarded services: disable it so the node keeps no unauthenticated
    # full-access account.
    out="$(rest "$AUTH" user/set "{\"numbers\":\"admin\",\"disabled\":\"yes\"}")"
    is_err "$out" && bootstrap_fail "disable factory admin: ${out:-no response}"
    if rest "admin:" system/resource >/dev/null 2>&1; then
        bootstrap_fail "factory admin still authenticates after disable"
    fi
fi
rest "$AUTH" system/resource >/dev/null 2>&1 \
    || bootstrap_fail "the requested credentials do not authenticate"
out="$(rest "$AUTH" system/identity/set "{\"name\":\"$(json_str "$HOSTNAME_ARG")\"}")"
is_err "$out" && bootstrap_fail "set identity: ${out:-no response}"
got="$(rest "$AUTH" system/identity)"
case "$got" in
    *"\"name\":\"$HOSTNAME_ARG\""*) : ;;
    *) bootstrap_fail "identity not applied: ${got:-no response}" ;;
esac

mgmt_setup
mgmt_attach

# ---- startup config -------------------------------------------------------
if [ -s "$STARTUP_CFG" ]; then
    bridge_ll
    # one-shot HTTP server on the bridge's link-local address; the router
    # fetches from it through ether1 (fetch takes a zoned link-local address
    # only as `address=`, not inside a URL)
    {
        printf 'HTTP/1.0 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %s\r\n\r\n' \
            "$(wc -c < "$STARTUP_CFG")"
        cat "$STARTUP_CFG"
    } | timeout 90 nc -l -s "$LL%$MGMT_BR" -p "$FETCH_PORT" >/dev/null 2>&1 &
    sleep 1
    fetch="$(rest "$AUTH" tool/fetch \
        "{\"address\":\"$LL%ether1\",\"port\":\"$FETCH_PORT\",\"src-path\":\"/config.auto.rsc\",\"mode\":\"http\",\"dst-path\":\"config.auto.rsc\"}")"
    case "$fetch" in
        *'"status":"finished"'*)
            # /import answers only when it finishes: a large config gets
            # CHR_BOOT_WAIT seconds.
            result="$(rest_raw "$AUTH" import '{"file-name":"config.auto.rsc"}' "$BOOT_WAIT")"
            case "$result" in
                *'"ret":"true"'*)
                    log "startup config imported"
                    ;;
                *)
                    # RouterOS imports line by line and stops at the first
                    # error: the lines before it ARE applied.
                    detail="$(printf '%s' "$result" | sed -n 's/.*"detail":"\([^"]*\)".*/\1/p')"
                    log "startup config import FAILED: ${detail:-${result:-no response}}; lines before the failing one were applied, the rest were not"
                    wait "$QEMU_PID"
                    exit 0
                    ;;
            esac
            ;;
        *)
            log "startup config import FAILED: fetch did not finish: ${fetch:-no response}"
            wait "$QEMU_PID"
            exit 0
            ;;
    esac
fi

touch "$DONE_MARKER" 2>/dev/null || true
ready
wait "$QEMU_PID"
