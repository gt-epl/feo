appName="$1"
initPort="$2"
numReplicas="$3"
feoIp="$4"

curl -X PUT "http://$feoIp/api/v1/namespaces/guest/actions/$appName?initPort=$initPort&numReplicas=$numReplicas"
