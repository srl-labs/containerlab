env GOOS=linux GOARCH=amd64 go build -o bin/container-lab src/*.go

go run src/*.go -h

nfpm pkg --packager rpm --target ./rpm/

export SRL_SW_IMAGE=srlinux:20.6.1-286
export SRL_LICENSE=~/containerlab/srl_config//license.key
export SRL_TOPOLOGY=~/containerlab/srl_config/types/topology-7220IXRD1.yml

docker run -t -d --rm --privileged --sysctl net.ipv6.conf.all.disable_ipv6=0 --sysctl net.ipv4.ip_forward=0 --sysctl net.ipv6.conf.all.accept_dad=0 --sysctl net.ipv6.conf.default.accept_dad=0 --sysctl net.ipv6.conf.all.autoconf=0 --sysctl net.ipv6.conf.default.autoconf=0 -u $(id -u):$(id -g) -v $SRL_LICENSE:/opt/srlinux/etc/license.key:ro -v $SRL_TOPOLOGY:/tmp/topology.yml --name test $SRL_SW_IMAGE sudo bash -c /opt/srlinux/bin/sr_linux


#go run main.go > mygraph.dot

dot -Tps graph/wan-topo.dot -o graph/wan-topo.ps

dot -Tpng -Gdpi=300 graph/wan-topo.dot > graph/wan-topo.png