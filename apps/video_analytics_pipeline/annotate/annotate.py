import cv2
import base64
import io
import numpy as np
import os


# twisted way to ensure container & local execution correctness. (container expects cfg in /)
if os.path.exists('./cfg'):
    class_file='cfg/yolov3.txt'
else:
    class_file='/cfg/yolov3.txt'

COLORS = None

with open(class_file, 'r') as f:
    classes = [line.strip() for line in f.readlines()]
    COLORS = np.random.uniform(0, 255, size=(len(classes), 3))

def draw_prediction(img, class_id, x, y, x_plus_w, y_plus_h):
    label = str(classes[class_id])
    color = COLORS[class_id]
    cv2.rectangle(img, (x,y), (x_plus_w,y_plus_h), color, 3)
    cv2.putText(img, label, (x-10,y-10), cv2.FONT_HERSHEY_SIMPLEX, 0.5, color, 3)
    return img

def get_frame(encoded_frame):
    frame_bytes = base64.b64decode(encoded_frame)
    frame_stream = io.BytesIO(frame_bytes)
    frame = cv2.imdecode(np.frombuffer(frame_stream.read(), np.uint8), 1)
    return frame

def main(args):

    image = get_frame(args['frame']) 
    indices = args["indices"]
    boxes = args["boxes"]
    class_ids = args["classids"]

    #res = []
    for i in indices:
        box = boxes[i]
        x = box[0]
        y = box[1]
        w = box[2]
        h = box[3]
        img = draw_prediction(image, class_ids[i], round(x), round(y), round(x+w), round(y+h))
        #res.append({'label':label, 'ts':ts, 'image':img})

    # Note: Do something with image. Perhaps store it to disk. 
    # This is a good candidate for a function reliant on state (disk)
     # Currently does nothing.
    return {"success":True}
