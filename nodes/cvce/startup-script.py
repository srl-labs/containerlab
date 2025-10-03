import time
import subprocess
from dataclasses import dataclass
from typing import List, Optional, Tuple


@dataclass
class WanConfig:
    intf_name: str
    mode: str  # DHCP or STATIC
    ip_address: Optional[str]
    netmask: Optional[str]
    next_hop: Optional[str]


@dataclass
class Config:
    wan_configs: List[WanConfig]
    vco_fqdn: str
    activation_key: str


VALID_WANS = {
    "GE3",
    "GE4",
    "GE5",
    "GE6",
    "GE7",
    "GE8",
}
VALID_MODES = {
    "STATIC",
    "DHCP"
}


def load_wan_configs(wan_configs: str) -> List[WanConfig]:
    configs = []

    for line in wan_configs.splitlines():
        if_name = ""
        if_mode = ""
        if_address = ""
        if_netmask = ""
        if_next_hop = ""

        items = line.split()

        if_name, if_mode, *tail = items

        if if_name not in VALID_WANS or if_mode not in VALID_MODES:
            raise ValueError("invalid interface name or mode")

        if if_mode == "STATIC":
            if_address, if_netmask, if_next_hop, *tail = tail
            configs.append(
                WanConfig(if_name, if_mode, if_address, if_netmask, if_next_hop)
            )
        elif if_mode == "DHCP":
            configs.append(WanConfig(if_name, if_mode, None, None, None))

    return configs


def load_activation_info(content: str) -> Tuple[str, str]:
    vco_fqdn = ""
    activation_key = ""

    for line in content.splitlines():
        items = line.split("=")
        items = [item.strip().lower() for item in items]
        if len(items) < 2:
            continue

        if items[0] == "key":
            activation_key = items[1].upper()
        elif items[0] == "vco_fqdn":
            vco_fqdn = items[1]

    return vco_fqdn, activation_key


def load_config() -> Config:
    wan_configs = []
    vco_fqdn = None
    activation_key = None

    with open("/clab-data/edge-config", "r") as f:
        content = f.read()
        wan_configs = load_wan_configs(content)

    with open("/clab-data/activation-info", "r") as f:
        content = f.read()
        vco_fqdn, activation_key = load_activation_info(content)

    if vco_fqdn and activation_key:
        return Config(wan_configs, vco_fqdn, activation_key)
    else:
        raise ValueError("missing vco fqdn or activation key")


def set_wan_config(config: WanConfig):
    args: List[str] = ["/opt/vc/bin/set_wan_config.sh", config.intf_name, config.mode]

    if config.mode == "STATIC":
        if config.ip_address and config.netmask and config.next_hop:
            args += [config.ip_address, config.netmask, config.next_hop]
        else:
            raise ValueError("must provide IP information for static configurations")

    subprocess.run(args)


def do_activate(vco_fqdn: str, activation_key: str):
    subprocess.run(
        ["/opt/vc/bin/activate.py", "-f", "-s", vco_fqdn, "-i", activation_key]
    )


def main():
    # give time for things to startup
    time.sleep(120)

    config = load_config()

    for wan_config in config.wan_configs:
        set_wan_config(wan_config)
        time.sleep(5)

    do_activate(config.vco_fqdn, config.activation_key)
    time.sleep(5)


if __name__ == "__main__":
    main()
