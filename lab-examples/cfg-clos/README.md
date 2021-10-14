# Tier-2 CLOS with Config Engine (cfg-clos)
For detailed information on this example, please refer to : https://containerlab.srlinux.dev/lab-examples/cfg-clos/

### Execution
```
# Deploy the topology
$ containerlab deploy --topo cfg-clos.topo.yml

# Generate and apply the configuration from the templates
$ containerlab config --topo cfg-clos.topo.yml  -p . -l cfg-clos 
```