"""Serve local design studies without stale HTML, CSS, or JavaScript.

Run: python3 docs/design/serve.py
This server is for local review; it is not the deployed frontend.
"""

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


class PreviewHandler(SimpleHTTPRequestHandler):
    def send_head(self):
        # Ignore validators as well as disabling storage: existing browser
        # caches must receive the current files rather than a 304 response.
        for header in ("If-Modified-Since", "If-None-Match"):
            if header in self.headers:
                del self.headers[header]
        return super().send_head()

    def end_headers(self):
        self.send_header("Cache-Control", "no-store, max-age=0")
        self.send_header("Pragma", "no-cache")
        self.send_header("Expires", "0")
        super().end_headers()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=8765)
    args = parser.parse_args()
    handler = partial(PreviewHandler, directory=str(Path(__file__).resolve().parent))
    print(f"Design preview: http://127.0.0.1:{args.port}/", flush=True)
    ThreadingHTTPServer(("127.0.0.1", args.port), handler).serve_forever()
