"""Records the last webhook request it receives, for tests. python3 scripts/webhook_sink.py 8766 /tmp/sink.json"""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        body = self.rfile.read(int(self.headers["content-length"])).decode()
        json.dump({"headers": dict(self.headers), "body": body}, open(sys.argv[2], "w"))
        self.send_response(200)
        self.end_headers()


HTTPServer(("0.0.0.0", int(sys.argv[1])), H).serve_forever()
