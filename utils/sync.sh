#!/bin/bash

policy=$1
DoUtils=$2
DST=/tmp/

write_config() {
  ip=$1
  sed "s/HOSTIP/$ip/" config.template.yml | sed "s/POLICY/$policy/" > config.yml
}

copy_config() {
  svr=$1
  ip=$(ssh $svr "ip -f inet addr show eth1 | sed -En -e 's/.*inet ([0-9.]+).*/\1/p'")

  write_config $ip
  echo "[.] copy config to $svr"
  rsync config.yml $svr:$DST
}

# Absolue directory of this script
SCRIPT_DIR=$(dirname "$(realpath $0)")
FEO_DIR="$SCRIPT_DIR/../"

export PATH=$PATH:/usr/local/go/bin
cd "$FEO_DIR"
go build
cd central_server
go build
cd ..

num_nodes=4
for ((i=0;i<$num_nodes;i++)); do
  svr=clabcl$i
  echo "[+] $svr : copy binaries"
  rsync feo $svr:$DST

  rsync central_server/central_server $svr:~/

  copy_config $svr

  ## onetime only
  if [ ! -z "$DoUtils" ]; then
    echo "[.] copy onetime utils and apps"
    rsync utils/custom.conf $svr:~/
    rsync -avz utils $svr:~/
    rsync -avz apps $svr:~/
  fi

  echo "---"

done

echo "[+] save config locally in tmp"
ip=$(ip -f inet addr show eth1 | sed -En -e 's/.*inet ([0-9.]+).*/\1/p')
write_config $ip
cp config.yml $DST
cp feo $DST

