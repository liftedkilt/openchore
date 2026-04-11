#!/usr/bin/env python3
"""
LiteRT-LM HTTP server for Gemma 4 vision + text inference.
Provides an Ollama-compatible API for drop-in replacement.

5.2s vision review, 1.5s text generation, ~2.3GB RAM.
"""
import base64
import json
import os
import sys
import tempfile
import time
from http.server import HTTPServer, BaseHTTPRequestHandler

try:
    import litert_lm
except ImportError:
    print("ERROR: litert-lm-api not installed. Run: pip install litert-lm-api")
    sys.exit(1)

MODEL_PATH = os.environ.get("MODEL_PATH", "/app/model/gemma-4-E4B-it.litertlm")
PORT = int(os.environ.get("PORT", "8080"))

engine = None


def get_engine():
    global engine
    if engine is None:
        print(f"Loading model from {MODEL_PATH}...")
        start = time.time()
        engine = litert_lm.Engine(MODEL_PATH, vision_backend=litert_lm.Backend.CPU)
        print(f"Model loaded in {time.time() - start:.1f}s")
    return engine


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/api/chat":
            self._handle_chat()
        elif self.path == "/api/generate":
            self._handle_generate()
        elif self.path == "/api/show":
            self._handle_show()
        elif self.path == "/api/pull":
            self._respond_json(200, {"status": "success"})
        else:
            self.send_error(404)

    def do_GET(self):
        if self.path == "/api/tags":
            self._handle_tags()
        elif self.path == "/health":
            self._respond_json(200, {"status": "ok"})
        else:
            self.send_error(404)

    def _handle_chat(self):
        try:
            body = self._read_body()
            messages = body.get("messages", [])
            if not messages:
                self._respond_json(400, {"error": "messages required"})
                return

            options = body.get("options", {})
            max_tokens = options.get("num_predict", 1024)

            eng = get_engine()
            start = time.time()
            tmp_files = []

            # Note: newer litert_lm.Engine.create_conversation() no longer
            # accepts max_tokens as a kwarg — the supported parameters are
            # messages, tools, tool_event_handler, extra_context. We still
            # log the client's requested max_tokens for observability.
            with eng.create_conversation() as conv:
                response = None
                for msg in messages:
                    role = msg.get("role", "user")
                    content_text = msg.get("content", "")
                    images = msg.get("images", [])

                    content_parts = []

                    for img_b64 in images:
                        tmp = tempfile.NamedTemporaryFile(
                            suffix=".png", delete=False
                        )
                        tmp.write(base64.b64decode(img_b64))
                        tmp.close()
                        tmp_files.append(tmp.name)
                        content_parts.append({"type": "image", "path": tmp.name})

                    content_parts.append({"type": "text", "text": content_text})
                    response = conv.send_message(
                        {"role": role, "content": content_parts}
                    )

            # Clean up temp files
            for f in tmp_files:
                try:
                    os.unlink(f)
                except OSError:
                    pass

            resp_text = self._extract_text(response)
            duration = time.time() - start
            print(f"litert: chat completed in {duration:.1f}s (max_tokens={max_tokens})")

            self._respond_json(200, {
                "model": "gemma4:e4b",
                "message": {"role": "assistant", "content": resp_text},
                "done": True,
                "total_duration": int(duration * 1e9),
            })

        except Exception as e:
            print(f"ERROR in chat: {e}", file=sys.stderr)
            import traceback
            traceback.print_exc()
            self._respond_json(500, {"error": str(e)})

    def _handle_generate(self):
        try:
            body = self._read_body()
            prompt = body.get("prompt", "")
            if not prompt:
                self._respond_json(400, {"error": "prompt required"})
                return

            options = body.get("options", {})
            max_tokens = options.get("num_predict", 1024)

            eng = get_engine()
            start = time.time()

            # See note in _handle_chat: create_conversation() no longer
            # accepts max_tokens in the current litert_lm API.
            with eng.create_conversation() as conv:
                response = conv.send_message({
                    "role": "user",
                    "content": [{"type": "text", "text": prompt}],
                })

            resp_text = self._extract_text(response)
            duration = time.time() - start
            print(f"litert: generate completed in {duration:.1f}s (max_tokens={max_tokens})")

            self._respond_json(200, {
                "model": "gemma4:e4b",
                "response": resp_text,
                "done": True,
                "total_duration": int(duration * 1e9),
            })

        except Exception as e:
            print(f"ERROR in generate: {e}", file=sys.stderr)
            self._respond_json(500, {"error": str(e)})

    def _handle_tags(self):
        self._respond_json(200, {
            "models": [{
                "name": "gemma4:e4b",
                "model": "gemma4:e4b",
                "size": os.path.getsize(MODEL_PATH)
                if os.path.exists(MODEL_PATH) else 0,
            }]
        })

    def _handle_show(self):
        body = self._read_body()
        model = body.get("model", "")
        if "gemma4" in model or "gemma-4" in model:
            self._respond_json(200, {
                "modelfile": "litert-lm",
                "template": "gemma4",
            })
        else:
            self._respond_json(404, {"error": f"model {model} not found"})

    @staticmethod
    def _extract_text(response):
        if isinstance(response, dict) and "content" in response:
            parts = []
            for item in response["content"]:
                if isinstance(item, dict) and item.get("type") == "text":
                    parts.append(item.get("text", ""))
            return "".join(parts)
        if isinstance(response, str):
            return response
        return ""

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))

    def _respond_json(self, status, data):
        body = json.dumps(data).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print(f"litert: {args[0]}")


if __name__ == "__main__":
    print("Initializing LiteRT-LM engine...")
    eng = get_engine()

    # Warmup: run a quick text inference to fully initialize the engine
    print("Running warmup inference...")
    try:
        with eng.create_conversation() as conv:
            conv.send_message({"role": "user", "content": [{"type": "text", "text": "Hi"}]})
        print("Warmup complete — engine ready.")
    except Exception as e:
        print(f"Warmup failed (non-fatal): {e}")

    print(f"LiteRT server listening on :{PORT}")
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
