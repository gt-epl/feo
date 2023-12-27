# Absolue directory of this script
SCRIPT_DIR=$(dirname "$(realpath $0)")
HOST_IP=$2 #192.168.10.10, etc. 

wsk action create incrementBy1 $SCRIPT_DIR/increment_by_1.py \
	--apihost $1 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create incrementBy2 $SCRIPT_DIR/increment_by_2.py \
	--apihost $1 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create incrementBy3 $SCRIPT_DIR/increment_by_3.py \
	--apihost $1 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create incrementBy4 $SCRIPT_DIR/increment_by_4.py \
	--apihost $1 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

curl -X PUT -H "Content-Type: application/x-yaml" --data-binary "@$SCRIPT_DIR/dag_manifest.yml" http://$HOST_IP:9696/api/v1/namespaces/guest/dag/testDagApp