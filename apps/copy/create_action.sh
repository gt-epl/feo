# Absolue directory of this script
SCRIPT_DIR=$(dirname "$(realpath $0)")

wsk action create copy $SCRIPT_DIR/copy.js \
	--apihost $1 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP