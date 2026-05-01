"""Local session viewer — reads from session state cache + local session files."""

import http.server
import json
import socketserver
import os
from pathlib import Path
from urllib.parse import urlparse

STATE_FILE = ".conflux/session_state.json"
try:
    from conflux.config import load_config
    config = load_config("./config.yaml")
    AGENT_PATHS = {}
    for name, aconf in config.agents.items():
        if aconf.enabled:
            AGENT_PATHS[name] = aconf.effective_path
except Exception:
    config = None
    AGENT_PATHS = {}

VIEWER_DIR = os.path.dirname(os.path.abspath(__file__))


def load_state():
    """Load session state cache."""
    if not os.path.exists(STATE_FILE):
        return {}
    try:
        with open(STATE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError):
        return {}


def get_session_files_by_state(sessions):
    """Build computer → agent → session tree from state cache."""
    tree = {}
    for sid, state in sessions.items():
        computer = state.get("computer", "未知电脑")
        agent = state.get("agent", "未知")
        title = state.get("title", sid[:12] + "...")
        msg_count = state.get("last_message_index", 0) + 1
        doc_token = state.get("doc_token", "")

        if computer not in tree:
            tree[computer] = {}
        if agent not in tree[computer]:
            tree[computer][agent] = []

        tree[computer][agent].append({
            "session_id": sid,
            "title": title,
            "message_count": msg_count,
            "doc_token": doc_token,
        })

    # Sort sessions by title (which starts with date)
    for computer in tree:
        for agent in tree[computer]:
            tree[computer][agent].sort(
                key=lambda s: s["title"], reverse=True)

    return tree


def parse_session_messages(session_id, agent_name):
    """Parse a session file locally and return messages as dicts."""
    parser_name = agent_name.lower()
    if parser_name not in AGENT_PATHS:
        return None

    base_path = AGENT_PATHS[parser_name]
    if not base_path or not os.path.exists(base_path):
        return None

    try:
        from conflux.parsers import get_parser
        parser = get_parser(parser_name)

        for fpath in Path(base_path).rglob("*.jsonl"):
            if fpath.stem == session_id:
                messages = parser.parse_session(fpath, "local")
                return [
                    {
                        "role": m.role,
                        "content": m.content,
                        "timestamp": m.timestamp.isoformat(),
                        "index": m.message_index,
                    }
                    for m in messages
                ]

        return None
    except Exception as e:
        return {"error": str(e)}


class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=VIEWER_DIR, **kwargs)

    def do_GET(self):
        parsed = urlparse(self.path)

        if parsed.path == "/api/tree":
            state = load_state()
            tree = get_session_files_by_state(state)
            self._json_response(tree)

        elif parsed.path.startswith("/api/session/"):
            # /api/session/{agent}/{session_id}
            parts = parsed.path.split("/")
            if len(parts) >= 5:
                agent = parts[3]
                session_id = parts[4]
                data = parse_session_messages(session_id, agent)
                if data is None:
                    self._json_response(
                        {"error": "Session not found"}, status=404)
                else:
                    self._json_response(
                        {"session_id": session_id, "agent": agent,
                         "messages": data})
            else:
                self._json_response({"error": "Invalid path"}, status=400)

        else:
            super().do_GET()

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, OPTIONS")
        self.send_header(
            "Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()

    def _json_response(self, data, status=200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(
            json.dumps(data, ensure_ascii=False).encode("utf-8"))


if __name__ == "__main__":
    PORT = 8765
    with socketserver.TCPServer(("", PORT), Handler) as httpd:
        print(f"Local viewer running on http://localhost:{PORT}")
        print(f"Tracking {len(load_state())} sessions from session state")
        httpd.serve_forever()
