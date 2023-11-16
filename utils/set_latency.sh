sudo tc qdisc add dev eth1 root netem delay 10ms
sudo tc -s qdisc
