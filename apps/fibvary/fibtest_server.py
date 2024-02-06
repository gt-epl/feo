from http.server import BaseHTTPRequestHandler, HTTPServer
import sys
import subprocess
import random
 
PORT = int(sys.argv[1])

# cmd = './fibtest -s 100 -t 1'

# The time taken by the function can vary between 85ms to 115ms.

class MyHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        e = random.randint(0,30)
        popen = subprocess.Popen(("./fibtest", "-s", str(85+e), "-t", "1"), stdout=subprocess.PIPE)
        popen.wait()
        output = popen.stdout.read()
        print(output)
        self.send_response(200)
        self.send_header('Content-type', 'text/html')
        self.end_headers()
        self.wfile.write(output)
    
    def do_GET(self):
        self.do_POST()
 
with HTTPServer(("", PORT), MyHandler) as httpd:
    print("Serving at port", PORT)
    httpd.serve_forever()
