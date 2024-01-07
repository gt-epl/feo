#!/bin/bash
# send an ack if required
if test -n "$__OW_WAIT_FOR_ACK"
  then echo '{"ok":true}' >&3
fi
# read input forever line by line
while read line
do
   # parse the in input with `jq`
   ms="$(echo $line | jq -r .value.ms)"
   ./fibtest -s $ms -t 1 > out 2>&1
   resp=$(cat out)
   # log in stdout
   #echo msg="hello $name"
   # produce the result - note the fd3
   echo '{"resp": "'$resp'"}' >&3
done
