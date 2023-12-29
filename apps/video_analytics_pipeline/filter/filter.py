import cv2
from skimage.metrics import structural_similarity
import base64
import io
import numpy as np
import time
import os

ssim_threshold = 0.8

'''
Description:
This function reviews structural similarity between 2 frames:
Current and Previous and returns a frame for further processing if set criterion is achieved - 
( sim_score > ssim_threshold)
'''

def get_frame(encoded_frame):
    frame_bytes = base64.b64decode(encoded_frame)
    frame_stream = io.BytesIO(frame_bytes)
    frame = cv2.imdecode(np.frombuffer(frame_stream.read(), np.uint8), 1)
    return frame
    
def main(args):
    global ssim_threshold
    response = {"success":False}
    

    start = time.time()
    frame = get_frame(args['cur_frame'])
    prev_frame = get_frame(args['prev_frame'])
    # print('decode_time: ', time.time() - start)

    # start = time.time()
    # gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
    # prev_gray = cv2.cvtColor(prev_frame, cv2.COLOR_BGR2GRAY)
    # print('preprocess_time: ', time.time() - start)

    # start = time.time()
    # can use this for a realistic setup
    #(score, diff) = structural_similarity(gray, prev_gray, full=True)

    # to speed up filter
    score  = np.random.uniform(0,1)
    ssim_threshold = 0.5

    # print('compute_time: ', time.time() - start)

    if float(ssim_threshold) < score:
        response["elapsed"] = time.time()-start
        return response

    response["success"] = True
    response["frame"] = args['cur_frame']
    response["elapsed"] = time.time()-start
    return response

def get_enc(fn):
    with open(fn,'rb') as fh:
        data = fh.read()
    
    encoded = base64.b64encode(data).decode()
    return encoded

if __name__ == '__main__':

    # Test code
    root = '../imgs'
    files = list(sorted(os.listdir(root)))

    prev_enc = get_enc(f'{root}/{files[0]}')

    for file in files[1:]:
        enc = get_enc(f'{root}/{file}')
        body = {"cur_frame":enc, "prev_frame":prev_enc}
        resp = main(body)
        print(resp['success'], resp['elapsed'])
        prev_enc = enc
