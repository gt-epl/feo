import requests
import base64
import os
import json
import time

feo_dag_url = "http://192.168.10.10:9696/api/v1/namespaces/guest/dag/testDagApp?blocking=true&result=true"

headers={
    "Content-Type": "application/json",
    "Authorization":"Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A="
    }

# Create initial payload
jobj = {"number": 20}
body = json.dumps(jobj)

# Invoke request
start = time.time()
response = requests.post(feo_dag_url, data=body, headers=headers)
print('filter: ', time.time()-start)

# Unmarshal response payload
resp = json.loads(response.text)
print(resp)
