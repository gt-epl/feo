intf=$1
sudo tc qdisc del dev $intf root netem
sudo tc -s qdisc
