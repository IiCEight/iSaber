"""Serve local design studies without stale HTML, CSS, or JavaScript.

Run: python3 docs/design/serve.py
This server is for local review; it is not the deployed frontend.
"""

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit


class PreviewHandler(SimpleHTTPRequestHandler):
    # Fixed public assets only: prototypes reuse production styles without
    # exposing the repository, configuration files, or application data.
    project = Path(__file__).resolve().parents[2]
    shared_assets = {
        "/shared/globals.css": project / "frontend/app/globals.css",
        "/shared/theme.css": project / "frontend/app/theme.css",
        "/shared/archive.css": project / "frontend/app/archive/archive.css",
        "/shared/pagination.css": project / "frontend/components/pagination.css",
        "/shared/icon.png": project / "frontend/app/icon.png",
    }

    def translate_path(self, path):
        asset = self.shared_assets.get(urlsplit(path).path)
        return str(asset) if asset else super().translate_path(path)

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
