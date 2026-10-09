"""RESOLVEX_API_KEY=... python examples/flask_app.py   (listens on :3003)"""
import logging
import random
import time

import resolvex

resolvex.init(service_name="recommendations", version="2.0.1")

from flask import Flask  # noqa: E402  (import after init so Flask is instrumented)

app = Flask(__name__)
log = logging.getLogger("recs")


@resolvex.traced("rank-items")
def rank():
    time.sleep(random.uniform(0.005, 0.03))
    return [1, 2, 3]


@app.get("/recs")
def recs():
    log.info("computing recommendations")
    rank()
    if random.random() < 0.1:
        log.error("model server timed out")
        return "model timeout", 500
    return {"items": [1, 2, 3]}


if __name__ == "__main__":
    app.run(port=3003)
