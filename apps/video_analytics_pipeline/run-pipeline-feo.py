import requests
import base64
import os
import json
import time

def get_enc(fn):
    with open(fn,'rb') as fh:
        data = fh.read()
    
    encoded = base64.b64encode(data).decode()
    return encoded

feo_dag_url = "http://192.168.10.10:9696/api/v1/namespaces/guest/dag/video_analytics_pipeline?blocking=true&result=true"

root = './imgs'
files = list(sorted(os.listdir(root)))[:20]

prev_enc = get_enc(f'{root}/{files[0]}')
headers={
    "Content-Type": "application/json",
    "Authorization":"Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A="
    }
for file in files[1:]:
    
    enc = get_enc(f'{root}/{file}')
    jobj = {"cur_frame":enc, "prev_frame":prev_enc}
    body = json.dumps(jobj)
    start = time.time()
    response = requests.post(feo_dag_url, data=body, headers=headers)
    print('pipeline: ', time.time()-start)
    prev_enc = enc
    resp = json.loads(response.text)
    print(resp)
    for key, value in response.headers.items():
        if "Invoc-Time" in key:
            print(key, ' : ', value)
    if not resp['success']:
        continue
