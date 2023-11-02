wrk -t 8 -c 16 -R20 -d60s -s copy.lua http://192.168.10.11:3233/api/v1/namespaces/guest/actions/copy?blocking=true&result=true
