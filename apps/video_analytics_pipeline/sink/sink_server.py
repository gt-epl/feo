from sink import  main as sink

from http.server import BaseHTTPRequestHandler, HTTPServer
from io import BytesIO
import sys
import subprocess
import json

PORT = int(sys.argv[1])

class MyHandler(BaseHTTPRequestHandler):
    def do_POST(self):        
        content_buffer = BytesIO()

        while True:
            # Read the chunk size
            line = self.rfile.readline().strip()
            chunk_size = int(line, 16)

            # If the chunk size is 0, it's the end of the content
            if chunk_size == 0:
                break

            # Read the chunk
            chunk = self.rfile.read(chunk_size)
            content_buffer.write(chunk)

            # Consume the trailing newline after each chunk
            self.rfile.readline()

        """
        if 'Content-Length' not in self.headers:
            self.send_error(411, 'Length Required: Content-Length header is missing')
            return

        # Get the length of the request body
        content_length = int(self.headers['Content-Length'])

        # Read the request body
        body = self.rfile.read(content_length)
        """

        content = content_buffer.getvalue()

        args = {}
        try:
            # Parse the JSON data
            args = json.loads(content.decode('utf-8'))
        except json.JSONDecodeError as e:
            # Handle JSON decoding error
            self.send_error(400, f"Invalid JSON: {e}")
            return


        output_dict = sink(args)
        
        output_json = json.dumps(output_dict)

        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        self.wfile.write(output_json.encode('utf-8'))

        print(output_dict)

    def do_GET(self):
        self.do_POST()
 
with HTTPServer(("", PORT), MyHandler) as httpd:
    print("Serving at port", PORT)
    httpd.serve_forever()
