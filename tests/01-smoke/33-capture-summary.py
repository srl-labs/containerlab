#!/usr/bin/env python3
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause
"""Summarize a little-endian pcapng file as containerlab capture writes it, as JSON.

Prints the number of sections, the interface names in order, the packet count per interface,
whether packets are in timestamp order, the number of 802.1Q tagged frames and the number of
ICMP echo requests and replies (untagged or tagged).
"""

import json
import struct
import sys

SHB, IDB, EPB = 0x0A0D0D0A, 0x00000001, 0x00000006


def main(path):
    with open(path, "rb") as f:
        data = f.read()

    sections, names, counts, stamps = 0, [], {}, []
    vlan, echo = 0, 0
    offset = 0
    while offset < len(data):
        block_type, length = struct.unpack_from("<II", data, offset)
        body = data[offset + 8 : offset + length - 4]
        if block_type == SHB:
            sections += 1
        elif block_type == IDB:
            options, name = body[8:], ""
            while len(options) >= 4:
                code, size = struct.unpack_from("<HH", options)
                if code == 0:
                    break
                if code == 2:
                    name = options[4 : 4 + size].decode()
                options = options[4 + size + (4 - size % 4) % 4 :]
            names.append(name)
        elif block_type == EPB:
            iface, high, low, captured = struct.unpack_from("<IIII", body)
            frame = body[20 : 20 + captured]
            counts[names[iface]] = counts.get(names[iface], 0) + 1
            stamps.append(high << 32 | low)
            ethertype, l3 = struct.unpack_from(">H", frame, 12)[0], 14
            if ethertype == 0x8100:
                vlan += 1
                ethertype, l3 = struct.unpack_from(">H", frame, 16)[0], 18
            if ethertype == 0x0800 and frame[l3 + 9] == 1 and frame[l3 + 20] in (0, 8):
                echo += 1
        offset += length

    print(
        json.dumps(
            {
                "sections": sections,
                "interfaces": names,
                "packets": counts,
                "ordered": stamps == sorted(stamps),
                "vlan_frames": vlan,
                "icmp_echo": echo,
            }
        )
    )


if __name__ == "__main__":
    main(sys.argv[1])
