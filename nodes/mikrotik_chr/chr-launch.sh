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
# with the image's own busybox + qemu: no python, no ftp client):
#   1. waits for the CLAB_INTFS data interfaces containerlab attaches (eth1..N)
#   2. management: a qemu user-mode NIC (slirp). CHR's default config runs a
#      DHCP client on ether1, slirp hands it 198.18.255.15 and forwards the
#      service ports from the container's mgmt IP (22 ssh, 23 telnet, 21 ftp,
#      80/443 www, 8291 winbox, 8728/8729 api, 161/udp snmp) to the router
#   3. data: one tap per container interface, bridged pairwise (ethN <-> tapN,
#      LLDP forwarded). NIC order in the VM follows the eth index, so container
#      ethN is always RouterOS ether(N+1), even with gaps
#   4. after boot (REST answers on 127.0.0.1:80 through the forward): sets the
#      credentials and the identity, isolates the management return path in
#      its own routing table (`mgmt`, below), then fetches
#      /ftpboot/config.auto.rsc (the containerlab startup-config, when present)
#      into the router over the slirp host alias and `/import`s it
#   5. prints the ready marker on the container log and waits on qemu
#
# Restart (docker restart, containerlab restart --node): RouterOS keys an
# ethernet interface on its MAC. A NIC that comes back with a new MAC is a NEW
# device (ether1 becomes ether3) and the DHCP client bound to ether1 never
# binds. Docker re-rolls the container's eth0 MAC whenever the netns is
# recreated, so the node id that seeds every NIC MAC and the data NIC count are
# persisted in the container on the first boot and reused: the restarted qemu
# carries the same NICs in the same order and RouterOS keeps ether1..N and
# their config. The REST bootstrap is skipped on a restart, only the
# idempotent management-table step re-runs.
#
# Architectures: the image's own qemu (qemu-system-x86_64 on amd64,
#   qemu-system-aarch64 + edk2 on arm64), KVM when /dev/kvm exists, else TCG.
# Args (the kind's Cmd): --hostname H [--username U --password P]
#   (unknown flags are ignored); credentials default to the USERNAME/PASSWORD env
# Env (all optional, set under the node's `env:`); MEM, CPUS and DISK are the
#   official image's own variables:
#   MEM (qemu -m value, default 256 + 8 MiB per data NIC), CPUS (default 1),
#   DISK (default 128M), CHR_INTF_WAIT (s, 60), CHR_BOOT_WAIT (s, 300)
# Serial console: telnet to port 5000 of the container.
# Ready: /.chr-launch-ready exists once RouterOS is ready (the kind's default
#   healthcheck); it is removed at every start.
# Log lines (docker logs clab-<lab>-<node>), all prefixed "chr-launch:":
#   "RouterOS ready"                   login + startup config done
#   "startup config import FAILED: …"  the router rejected the .rsc (RouterOS's
#                                      own reason, with the line number)
#   "credential bootstrap FAILED: …"   the user/password step failed
#   "management route setup FAILED: …" the mgmt table step failed
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
SYSNET=/sys/class/net

log() { echo "chr-launch: $*"; }
rm -f "$READY_FILE"

# The slirp management subnet sits in the RFC 2544 benchmarking range, never in
# an address a lab is likely to use on a link: a connected 10.0.0.0/30 on
# ether2 (vrnetlab's 10.0.0.0/24 slirp) captured the slirp host 10.0.0.2, the
# source of EVERY forwarded REST/SSH/API connection, and cut management.
SLIRP_NET=198.18.255.0/24
ROS_IP=198.18.255.15
SLIRP_HOST=198.18.255.2
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
        TCG_CPU="-cpu cortex-a57" ;;
    *)
        QEMU="qemu-system-x86_64"
        TCG_CPU="" ;;
esac
if [ -e /dev/kvm ]; then
    ACCEL="-cpu host -accel kvm"
else
    ACCEL="$TCG_CPU -accel tcg,split-wx=on"
    log "no /dev/kvm: running under TCG emulation (slow boot)"
fi

MGMT="user,id=mgmt,net=$SLIRP_NET,host=$SLIRP_HOST,dhcpstart=$ROS_IP"
for p in 21 22 23 80 443 8291 8728 8729; do
    MGMT="$MGMT,hostfwd=tcp::$p-$ROS_IP:$p"
done
MGMT="$MGMT,hostfwd=udp::161-$ROS_IP:161"

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
NET="-netdev $MGMT -device virtio-net,netdev=mgmt,mac=52:54:00:$node_id:00"

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
    mac="$(printf '52:54:00:%s:%02x' "$node_id" "$((i & 255))")"
    NET="$NET -netdev tap,id=d$i,ifname=$tap,script=no,downscript=no -device virtio-net,netdev=d$i,mac=$mac"
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
            --header "$_auth" --post-data "$3" "http://127.0.0.1:80/rest/$2" 2>/dev/null
    else
        wget -q -O- -T "${REST_T:-20}" --header "$_auth" "http://127.0.0.1:80/rest/$2" 2>/dev/null
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
    timeout "$(( $4 + 5 ))" nc 127.0.0.1 80 <"$_f" >"$_f.out" 2>/dev/null &
    _pid=$!
    exec 7>"$_f"
    printf 'POST /rest/%s HTTP/1.1\r\nHost: 127.0.0.1\r\nAuthorization: Basic %s\r\nContent-Type: application/json\r\nContent-Length: %s\r\nConnection: close\r\n\r\n%s' \
        "$2" "$(printf '%s' "$1" | base64 | tr -d '\n')" \
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
# configuration keeps SSH enabled, the forwarded port must send its banner first.
# A config that disables SSH is not waited for; a banner that never comes is
# logged and the node is still declared ready (REST works).
ssh_banner() { (sleep 3) | nc 127.0.0.1 22 2>/dev/null | head -c 7; }
ready() {
    case "$(rest "$AUTH" "ip/service?name=ssh")" in
        *'"disabled":"false"'*)
            _w=0
            until [ "$(ssh_banner)" = "SSH-2.0" ]; do
                if [ "$_w" -ge "$BOOT_WAIT" ]; then
                    log "SSH did not answer within ${BOOT_WAIT}s (REST works)"
                    break
                fi
                sleep 2
                _w=$((_w + 5))
            done
            ;;
    esac
    touch "$READY_FILE" 2>/dev/null || true
    log "RouterOS ready"
}

# A failed step WITHHOLDS the ready marker (qemu keeps running, so the node can
# still be inspected over the serial console, telnet port 5000).
bootstrap_fail() {
    log "credential bootstrap FAILED: $1"
    wait "$QEMU_PID"
    exit 0
}

# ---- management return path: its own routing table ------------------------
# CHR's DHCP client on ether1 installs the management default route
# (0.0.0.0/0 via the slirp host) in `main`. Every forwarded SSH/REST/API
# connection reaches the router from a source OUTSIDE the slirp subnet (the
# docker bridge address), so each reply needs a default route, and a lab
# default route the startup config adds to `main` (a static 0.0.0.0/0, a PPPoE
# client's add-default-route, a dual-WAN pair) competes with it and cuts
# management while the container runs and the data plane works. So, before the
# startup config can add any route: a `mgmt` table with its own default via the
# slirp host, a rule sending everything sourced from ether1's address to that
# table only, and the DHCP default withdrawn. `main` carries nothing but what
# the lab configures. Each object is looked up first, so a re-run adds nothing
# twice, which is why the restart path re-runs it.
mgmt_fail() {
    log "management route setup FAILED: $1 (redeploy the node: containerlab deploy --reconfigure)"
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
mgmt_route_setup() {
    rest_has "routing/table?name=$MGMT_TABLE" \
        || rest_add "routing table $MGMT_TABLE" routing/table/add "{\"name\":\"$MGMT_TABLE\",\"fib\":\"yes\"}"
    rest_has "ip/route?routing-table=$MGMT_TABLE&dst-address=0.0.0.0/0" \
        || rest_add "default route in table $MGMT_TABLE" ip/route/add \
            "{\"dst-address\":\"0.0.0.0/0\",\"gateway\":\"$SLIRP_HOST\",\"routing-table\":\"$MGMT_TABLE\"}"
    rest_has "routing/rule?src-address=$ROS_IP/32&table=$MGMT_TABLE" \
        || rest_add "routing rule for $ROS_IP" routing/rule/add \
            "{\"src-address\":\"$ROS_IP/32\",\"action\":\"lookup-only-in-table\",\"table\":\"$MGMT_TABLE\"}"
    _id="$(rest "$AUTH" "ip/dhcp-client?interface=ether1" | sed -n 's/.*"\.id":"\([^"]*\)".*/\1/p' | head -n1)"
    [ -n "$_id" ] || mgmt_fail "no DHCP client on ether1"
    rest_add "dhcp-client add-default-route=no" ip/dhcp-client/set \
        "{\"numbers\":\"$_id\",\"add-default-route\":\"no\"}"
    # Prove the three took (a transport flake answers an empty body, which is
    # not an error by the set-contract).
    rest_has "ip/route?routing-table=$MGMT_TABLE&dst-address=0.0.0.0/0" \
        || mgmt_fail "default route not in table $MGMT_TABLE"
    rest_has "routing/rule?src-address=$ROS_IP/32&table=$MGMT_TABLE" \
        || mgmt_fail "routing rule for $ROS_IP not applied"
    rest_has "ip/dhcp-client?interface=ether1&add-default-route=no" \
        || mgmt_fail "dhcp-client still adds the default route"
    log "management return path isolated in routing table $MGMT_TABLE"
}

# Bootstrap + startup-config import happen ONCE per container lifetime. On a
# restart the CHR disk keeps its config and the marker (in the container rw
# layer) survives, so the bootstrap (which could wait forever if the config
# disabled `www` or changed the account, or reject duplicate `add` lines) is
# skipped. A fresh deploy recreates the container and the disk, so the marker
# and the config are both gone and the bootstrap runs.
if [ -f "$DONE_MARKER" ]; then
    log "already bootstrapped (restart): skipping config"
    mgmt_route_setup
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

mgmt_route_setup

# ---- startup config -------------------------------------------------------
if [ -s "$STARTUP_CFG" ]; then
    # one-shot HTTP server the router fetches from through the slirp host alias
    {
        printf 'HTTP/1.0 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %s\r\n\r\n' \
            "$(wc -c < "$STARTUP_CFG")"
        cat "$STARTUP_CFG"
    } | timeout 90 nc -l -s 127.0.0.1 -p "$FETCH_PORT" >/dev/null 2>&1 &
    sleep 1
    fetch="$(rest "$AUTH" tool/fetch \
        "{\"url\":\"http://$SLIRP_HOST:$FETCH_PORT/config.auto.rsc\",\"dst-path\":\"config.auto.rsc\",\"mode\":\"http\"}")"
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
