#!/bin/bash

policy=$1
DoUtils=$2
DST=/tmp/
IPINFO=/tmp/ipinfo.csv

GO() {
  GOOS=linux GOARCH=amd64 go $@
}

write_config() {
  ip=$1
  sed "s/HOSTIP/$ip/" config.template.yml | sed "s/POLICY/$policy/" > config.yml
}

copy_config() {
  svr=$1
  intf=$2
  echo "[.] copy config to $svr@$intf"

  ip=$(ssh $svr "ip -f inet addr show $intf | sed -En -e 's/.*inet ([0-9.]+).*/\1/p'")
  echo $svr, $ip > $IPINFO


  write_config $ip
  rsync config.yml $svr:$DST
}

copy_to_svr() {
  svr=$1
  intf=$2
  echo "[+] $svr : copy binaries"
  rsync feo $svr:$DST

  rsync central_server/central_server $svr:~/

  copy_config $svr $intf

  ## onetime only
  if [ ! -z "$DoUtils" ]; then
    echo "[.] copy onetime utils and apps"
    rsync utils/custom.conf $svr:~/
    rsync -avz utils $svr:~/
    rsync -avz apps $svr:~/
  fi

  echo "---"
}

# Absolue directory of this script
SCRIPT_DIR=$(dirname "$(realpath $0)")
FEO_DIR="$SCRIPT_DIR/../"

#export PATH=$PATH:/usr/local/go/bin
echo "[+] build feo"
cd $FEO_DIR
GO build

echo "[+] build central_server"
cd central_server
GO build
cd $FEO_DIR

# assume ssh config with the aliases present in the hostsfile below 
# is present on the node which executes the script
hostsfile=$SCRIPT_DIR/clabhosts.txt
hostsfile=$SCRIPT_DIR/azhosts.txt

rm $IPINFO
echo "alias,ip" > $IPINFO

# Funny story: Use an unused descriptor 9. 
# Because any commands which reads from stdin (e.g. ssh in copy_config)
# will consume all of the input from the hostfile and while loop will terminate
# Instead force file to be read from 9
while IFS= read -r -u 9 host; do
  if [ ! -z "$host" ]; then
    copy_to_svr $host eth1 #for clab
    # copy_to_svr $host eth0 #for azure
    echo $host
  fi
done 9< $hostsfile
