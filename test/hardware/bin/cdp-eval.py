#!/usr/bin/env python3
"""Read one fact out of a Chrome tab on the attached phone.

SPDX-License-Identifier: AGPL-3.0-or-later
Copyright (C) 2026 Iman Samizadeh

    cdp-eval.py <base> <url-fragment> <body|url|title>

<base> is an http://127.0.0.1:PORT that an `adb forward` points at the phone's
localabstract:chrome_devtools_remote socket.

Why this exists rather than only reading the screen: measured on the attached
handset on 2026-08-30, the first `uiautomator dump` after launching Chrome
contained a promotional modal dialog and NOT the page. A readback that can be
covered by a dialog is a readback that returns the wrong answer quietly. The
DevTools protocol reads the DOM and does not care what is drawn on top of it.

"url" and "title" need only the plain HTTP /json/list endpoint and no
dependency at all. "body" needs a WebSocket, because Runtime.evaluate has no
HTTP form. Both websockets and websocket-client are supported because neither
is in the standard library and which one a machine has is luck.

Traffic here rides the USB cable, not the phone's wifi. So this reads the
result of a fetch that went over wifi; it never becomes the fetch itself.
"""

import json
import sys
import urllib.request

EXIT_OK = 0
EXIT_NO_TAB = 3
EXIT_NO_WEBSOCKET = 4


def tabs(base):
    with urllib.request.urlopen(base + "/json/list", timeout=10) as fh:
        return json.load(fh)


def pick(base, fragment):
    matches = [
        t for t in tabs(base)
        if t.get("type") == "page" and fragment in (t.get("url") or "")
    ]
    if not matches:
        print("no Chrome tab has a URL containing %r" % fragment, file=sys.stderr)
        sys.exit(EXIT_NO_TAB)
    # Newest first is what /json/list gives; the most recent navigation wins.
    return matches[0]


def body_text(ws_url):
    request = json.dumps({
        "id": 1,
        "method": "Runtime.evaluate",
        "params": {
            "expression": "document.body ? document.body.innerText : ''",
            "returnByValue": True,
        },
    })

    try:
        from websockets.sync.client import connect
    except ImportError:
        pass
    else:
        with connect(ws_url, max_size=8 * 1024 * 1024, open_timeout=10) as conn:
            conn.send(request)
            while True:
                message = json.loads(conn.recv(timeout=15))
                if message.get("id") == 1:
                    return message["result"]["result"].get("value", "")

    try:
        import websocket
    except ImportError:
        print(
            "neither the 'websockets' nor the 'websocket-client' package is "
            "installed, so the DOM cannot be read. Install one, or accept the "
            "uiautomator readback and its modal-dialog limit.",
            file=sys.stderr,
        )
        sys.exit(EXIT_NO_WEBSOCKET)

    conn = websocket.create_connection(ws_url, timeout=15)
    try:
        conn.send(request)
        while True:
            message = json.loads(conn.recv())
            if message.get("id") == 1:
                return message["result"]["result"].get("value", "")
    finally:
        conn.close()


def main():
    if len(sys.argv) != 4:
        print(__doc__.strip().splitlines()[2], file=sys.stderr)
        return EXIT_NO_WEBSOCKET
    base, fragment, what = sys.argv[1], sys.argv[2], sys.argv[3]
    tab = pick(base, fragment)
    if what == "url":
        print(tab.get("url") or "")
    elif what == "title":
        print(tab.get("title") or "")
    elif what == "body":
        print(body_text(tab["webSocketDebuggerUrl"]))
    else:
        print("unknown field %r: use body, url or title" % what, file=sys.stderr)
        return EXIT_NO_WEBSOCKET
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
