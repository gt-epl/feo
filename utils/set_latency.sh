lat=$1
sudo tc qdisc add dev eth1 root netem delay ${lat}ms
sudo tc -s qdisc
