from detect.detect import main as detect
from filter.filter import get_enc, main as filter
from annotate.annotate import main as annotate
from sink.sink import main as sink 
import os


root = './imgs'
files = list(sorted(os.listdir(root)))[:20]

prev_enc = get_enc(f'{root}/{files[0]}')

for file in files[1:]:
    
    # 1. Filter
    print('filter')
    enc = get_enc(f'{root}/{file}')
    body = {"cur_frame":enc, "prev_frame":prev_enc}
    resp = filter(body)
    prev_enc = enc

    if not resp['success']:
        continue

    # 2. Detect
    print('detect')
    body = {"frame":resp['frame']}
    resp = detect(body)

    # 3. Annotate
    print('annotate')
    body = { "indices":resp["indices"], 
            "frame":resp["frame"],
            "boxes":resp["boxes"],
            "classids":resp["classids"]}
    resp = annotate(body)


    # 4. Sink (In FaaS, this happens in parallel to 3.)
    print('sink')
    resp = sink(body)



