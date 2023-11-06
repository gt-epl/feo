wrk -t 4 -c 4 -R1 -d1s -s copy.lua http://192.168.10.11:9696/api/v1/namespaces/guest/actions/copy?blocking=true&result=true
