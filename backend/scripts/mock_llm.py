"""A fake OpenAI-compatible server used to test the assistant end to end without a real API key.

First request  -> asks to call list_services.
Second request -> answers using whatever the tool returned.
Run: python3 scripts/mock_llm.py 8765   (then use base URL http://host.docker.internal:8765/v1)
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        tool_msgs = [m for m in body["messages"] if m["role"] == "tool"]
        if body["messages"][-1]["role"] == "user" and "Reply with the single word OK" in body["messages"][0].get("content", ""):
            reply = {"choices": [{"message": {"content": "OK"}}]}
        elif not tool_msgs:
            reply = {"choices": [{"message": {"content": "", "tool_calls": [
                {"id": "call_1", "type": "function", "function": {"name": "list_services", "arguments": json.dumps({"window": "24h"})}}]}}]}
        else:
            services = json.loads(tool_msgs[-1]["content"])
            worst = max(services, key=lambda s: s["errorRate"]) if isinstance(services, list) and services else None
            text = f"MOCK ANSWER: the least healthy service is **{worst['name']}** with an error rate of {worst['errorRate']:.0%}." if worst else "MOCK ANSWER: no services."
            reply = {"choices": [{"message": {"content": text}}]}
        out = json.dumps(reply).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(out)))
        self.end_headers()
        self.wfile.write(out)


HTTPServer(("0.0.0.0", int(sys.argv[1])), H).serve_forever()
