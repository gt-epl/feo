intf=$1
lat=$2
sudo tc qdisc add dev $intf root netem delay ${lat}ms
sudo tc -s qdisc
