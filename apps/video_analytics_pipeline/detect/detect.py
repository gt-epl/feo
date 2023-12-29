
import cv2
import numpy as np
import time
import base64
import io
import os

# twisted way to ensure container & local execution correctness. (container expects cfg in /)
if os.path.exists('./cfg'):
    weights_file = './cfg/yolov3-tiny.weights'
    cfg_file     = './cfg/yolov3-tiny.cfg'
else:
    weights_file = '/cfg/yolov3-tiny.weights'
    cfg_file     = '/cfg/yolov3-tiny.cfg'

neuralnet = cv2.dnn.readNet(weights_file, cfg_file)

def get_frame(encoded_frame):
    frame_bytes = base64.b64decode(encoded_frame)
    frame_stream = io.BytesIO(frame_bytes)
    frame = cv2.imdecode(np.frombuffer(frame_stream.read(), np.uint8), 1)
    return frame

def main(args):
    global neuralnet
    try:

        image = get_frame(args['frame']) 
        Width = image.shape[1]
        Height = image.shape[0]
        scale = 0.00392

        blob = cv2.dnn.blobFromImage(image, scale, (416,416), (0,0,0), True, crop=False)
        neuralnet.setInput(blob)
        layer_names = neuralnet.getLayerNames()
        output_layers = [layer_names[i - 1] for i in neuralnet.getUnconnectedOutLayers()]
        outs = neuralnet.forward(output_layers)


        class_ids = []
        confidences = []
        boxes = []
        conf_threshold = 0.5
        nms_threshold = 0.4

        for out in outs:
            for detection in out:
                scores = detection[5:]
                class_id = int(np.argmax(scores))
                confidence = scores[class_id]
                if confidence > 0.5:
                    center_x = int(detection[0] * Width)
                    center_y = int(detection[1] * Height)
                    w = int(detection[2] * Width)
                    h = int(detection[3] * Height)
                    x = center_x - w / 2
                    y = center_y - h / 2
                    class_ids.append(class_id)
                    confidences.append(float(confidence))
                    boxes.append([x, y, w, h])


        # post processing
        indices = cv2.dnn.NMSBoxes(boxes, confidences, conf_threshold, nms_threshold)

        result =  {"boxes": boxes,
                "indices": indices.tolist(),
                "classids":class_ids,
                "confidence":confidences,
                  "frame": args["frame"]}
        return result

    except Exception as e:
        return {
            "statusCode": 500,
            "body": f"Internal Server Error: {str(e)}"
        }

if __name__ == '__main__':
    root = '../imgs'
    for file in os.listdir(root):
        fn = f'{root}/{file}'
        with open(fn,'rb') as fh:
            data = fh.read()
        
        encoded = base64.b64encode(data).decode()
        body = {"frame":encoded}
        start = time.time()
        resp = main(body)
        print(f'{file} main time:', time.time()-start)
