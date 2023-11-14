sudo tc qdisc add dev eth1 root netem delay 2.5ms
sudo tc -s qdisc
