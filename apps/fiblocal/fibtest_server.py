from http.server import BaseHTTPRequestHandler, HTTPServer
import sys
import subprocess
 
PORT = int(sys.argv[1])

# cmd = './fibtest -s 100 -t 1'
 
class MyHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        popen = subprocess.Popen(("./fibtest", "-s", "100", "-t", "1"), stdout=subprocess.PIPE)
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
