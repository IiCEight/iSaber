#!/usr/bin/env python3
"""Serve one locally saved public article for read-only visual review.

The input is an anonymous public API response. No network requests, account
cookies, publishing, or other writes are forwarded by this preview server.
"""
import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, unquote, urlsplit

ARTICLE = {}


class PublicPreview(BaseHTTPRequestHandler):
    def reply(self, status, data):
        body = json.dumps(data, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        url = urlsplit(self.path)
        path = unquote(url.path)
        query = parse_qs(url.query)
        if path == "/api/v1/auth/session":
            return self.reply(401, {"code": "unauthorized"})
        if path == "/api/v1/articles/" + ARTICLE["slug"]:
            return self.reply(200, ARTICLE)
        if path in ("/api/v1/categories", "/api/v1/tags"):
            items = ([ARTICLE["category"]] if ARTICLE.get("category") else []) if path.endswith("categories") else ARTICLE.get("tags", [])
            return self.reply(200, {"items": items, "next_cursor": None})
        if path in ("/api/v1/articles", "/api/v1/articles/search"):
            summary = {key: value for key, value in ARTICLE.items() if key not in ("markdown", "comments_enabled")}
            matches = True
            for key in ("category", "tag", "language"):
                value = query.get(key, [""])[0]
                if value:
                    matches &= ARTICLE["language"] == value if key == "language" else any(label["slug"] == value for label in ARTICLE.get("tags", [])) if key == "tag" else (ARTICLE.get("category") or {}).get("slug") == value
            text = query.get("q", [""])[0].casefold()
            matches &= all(word in (ARTICLE["title"] + " " + ARTICLE["markdown"]).casefold() for word in text.split())
            items = [summary] if matches else []
            return self.reply(200, {"items": items, "next_cursor": None, "page": 1, "page_size": 10, "total": len(items), "total_pages": 1})
        if path.endswith("/comments"):
            # Do not invent an empty comment history for this article snapshot.
            return self.reply(503, {"code": "comments_not_in_snapshot"})
        self.reply(404, {"code": "not_found"})

    def do_POST(self):
        self.reply(405, {"code": "read_only_preview"})

    do_PUT = do_POST
    do_PATCH = do_POST
    do_DELETE = do_POST


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=18080)
    parser.add_argument("--article-json", required=True, help="Path to a saved anonymous public article JSON response")
    args = parser.parse_args()
    with open(args.article_json, encoding="utf-8") as source:
        ARTICLE.update(json.load(source))
    for field in ("id", "slug", "title", "markdown", "language"):
        if not isinstance(ARTICLE.get(field), str):
            parser.error(f"Article JSON is missing {field}")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), PublicPreview)
    print(f"Read-only article snapshot: http://127.0.0.1:{args.port} — {ARTICLE['title']}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
