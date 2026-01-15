# Containerlab Labs (Extended) — Cisco ASE Track

A curated set of **demo-first, hands-on networking labs** built on **containerlab**.
Forked from `hellt/clabs` and extended with **Cisco Systems Engineering–aligned** enterprise scenarios.

## What you’ll find here
- **Enterprise routing + segmentation** (TCP/IP, subnets, default gateways)
- **Security policy simulation** (ACL-style controls / least privilege)
- **Repeatable demos** (setup + validation scripts, test cases)
- **Troubleshooting-ready** workflows (ping/trace, reachability, policy checks)

## Custom Labs
- **Cisco Smart Enterprise Network + Security Lab**  
  Path: [labs/cisco-smart-enterprise/](labs/cisco-smart-enterprise/)  
  Focus: multi-department segmentation + routed L3 gateway + ACL-style security validation

## How to run (later / when ready)
> Requires Docker + containerlab (WSL/Codespaces both supported).
```bash
# Example (inside a lab folder)
containerlab deploy -t topology.clab.yml
bash scripts/setup.sh
bash scripts/test.sh
containerlab destroy -t topology.clab.yml
