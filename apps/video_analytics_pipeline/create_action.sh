# Absolue directory of this script
SCRIPT_DIR=$(dirname "$(realpath $0)")

wsk action create filter filter/filter.py \
  --memory 1024 \
  --docker asarma31/openwhisk-video-analytics-pipeline-base \
	--apihost http://localhost:3233 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create detect detect/detect.py \
  --memory 1024 \
  --docker asarma31/openwhisk-video-analytics-pipeline-base \
	--apihost http://localhost:3233 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create annotate annotate/annotate.py \
  --memory 1024 \
  --docker asarma31/openwhisk-video-analytics-pipeline-base \
	--apihost http://localhost:3233 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP

wsk action create sink sink/sink.py \
  --memory 1024 \
  --docker asarma31/openwhisk-video-analytics-pipeline-base \
	--apihost http://localhost:3233 \
	--auth 23bc46b1-71f6-4ed5-8c54-816aa4f8c502:123zO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP
