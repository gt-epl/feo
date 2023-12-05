import requests
import json
import base64

# TODO: Change this to openwhisk
# Define the URL of your deployed OpenFaaS function
function_url = "http://127.0.0.1:31112/function/detection"
function_url = "http://localhost:3233/api/v1/namespaces/guest/actions/detect?blocking=true&result=true"

# Load an image as bytes (replace with your image path)
with open('coldstart.jpeg', 'rb') as image_file:
    image_data = image_file.read()

encoded = "test"
encoded = base64.b64encode(image_data).decode()

body = json.dumps({"img":encoded})
print(len(body))
# Send a POST request to your function
response = requests.post(function_url, data=body, headers={"Content-Type": "application/json","Authorization":"Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A="})

# Check the response status code
if response.status_code == 200:
    print(response.text)
    exit()
    # Parse the JSON response
    result = json.loads(response.text)
    bounding_boxes = result["bounding_boxes"]
    preprocess_time = result["preprocessing_time"]
    prediction_time = result["prediction_time"]
    
    # Process the bounding boxes or other results as needed
    print("Bounding Boxes:", bounding_boxes)
    print("Preprocessing Time:", preprocess_time)
    print("Prediction Time:", prediction_time)
else:
    print("Error:", response.status_code, response.text)
