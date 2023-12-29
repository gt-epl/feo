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

filter_url = "http://localhost:3233/api/v1/namespaces/guest/actions/filter?blocking=true&result=true"
detect_url = "http://localhost:3233/api/v1/namespaces/guest/actions/detect?blocking=true&result=true"
annotate_url = "http://localhost:3233/api/v1/namespaces/guest/actions/annotate?blocking=true&result=true"
sink_url = "http://localhost:3233/api/v1/namespaces/guest/actions/sink?blocking=true&result=true"

root = './imgs'
files = list(sorted(os.listdir(root)))[:20]

prev_enc = get_enc(f'{root}/{files[0]}')
headers={
    "Content-Type": "application/json",
    "Authorization":"Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A="
    }
for file in files[1:]:
    
    # 1. Filter
    enc = get_enc(f'{root}/{file}')
    jobj = {"cur_frame":enc, "prev_frame":prev_enc}
    body = json.dumps(jobj)
    start = time.time()
    response = requests.post(filter_url, data=body, headers=headers)
    print('filter: ', time.time()-start)
    prev_enc = enc
    resp = json.loads(response.text)
    if not resp['success']:
        continue


    # 2. Detect
    jobj = {"frame":resp['frame']}
    body = json.dumps(jobj)
    start = time.time()
    response = requests.post(detect_url, data=body, headers=headers)
    print('detect: ', time.time()-start)


    # 3. Annotate
    start = time.time()
    response = requests.post(annotate_url, data=response.text, headers=headers)
    print('annotate: ', time.time()-start)
    resp = json.loads(response.text)

    print(resp)

    # 4. Sink (In FaaS, this happens in parallel to 3.)
    start = time.time()
    response = requests.post(sink_url, data=response.text, headers=headers)
    print('sink: ', time.time()-start)
    resp = json.loads(response.text)
