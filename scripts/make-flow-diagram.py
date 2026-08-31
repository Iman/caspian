#!/usr/bin/env python3
"""Generate the level-0 flow diagram, one SVG per language.

WHY A GENERATOR AND NOT FOUR HAND-DRAWN FILES

The diagram exists in four languages. Drawn by hand, the Russian one gets a box
moved, nobody who reads Russian ever looks at the English one again, and the
four drift into four different diagrams that claim four different things. Here
the LAYOUT is written once and only the STRINGS differ, so a change to what the
picture says is a change to every language or it is a change to none.

WHY AN SVG AND NOT A HOSTED DIAGRAM

Nothing in this repository fetches anything at read time. A diagram embedded
from a drawing service would tell that service the address of every person who
opens the README, which is the same objection the panel already answers by
never loading a remote asset. The output here is a self-contained SVG: no
script, no remote font, no remote image. TestTheFlowDiagramsFetchNothing
enforces that.

Regenerate with:  python3 scripts/make-flow-diagram.py
"""

import os

# The protocols named on the box. Kept to what the parser actually accepts, in
# the same order as the README's own table, because a diagram is where an
# unsupported protocol would be believed the longest.
PROTOCOLS = "VLESS   VMess   Trojan   Shadowsocks   SOCKS   Hysteria2"
INPUTS = {
    "en": "share link  ·  Clash YAML  ·  xray JSON",
    "fa": "لینک اشتراک  ·  Clash YAML  ·  xray JSON",
    "ru": "ссылка  ·  Clash YAML  ·  xray JSON",
    "zh": "分享链接  ·  Clash YAML  ·  xray JSON",
}

LATIN = "Inter, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif"
FONTS = {
    "en": LATIN,
    "ru": LATIN,
    "fa": "Vazirmatn, Tahoma, 'Segoe UI', 'Noto Sans Arabic', 'Noto Naskh Arabic', sans-serif",
    "zh": "'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Noto Sans CJK SC', sans-serif",
}

STRINGS = {
    "en": {
        "title": "How Caspian works",
        "subtitle": "One box. Everything that joins its Wi-Fi comes out the other side.",
        "nodes": [
            ("Your devices", "phone, laptop, TV,", "the whole family"),
            ("The Caspian box", "a Raspberry Pi holding", "the config you pasted"),
            ("Your home router", "and your internet", "provider"),
            ("Your server", "abroad, at the address", "inside your config"),
            ("The open internet", "", ""),
        ],
        "arrows": ["Wi-Fi", "encrypted", "encrypted", "ordinary traffic"],
        "tunnel": "Encrypted tunnel. The router and the internet provider see one connection to one address, not what you open.",
        "foot_left": "Nothing is installed on any device. They join a Wi-Fi network, and that is all they do.",
        "foot_right": "Your server can see your traffic. Use one you trust, or one you run.",
    },
    "fa": {
        "title": "کاسپین چگونه کار می‌کند",
        "subtitle": "یک جعبه. هر چه به وای‌فای آن وصل شود، از آن سوی تونل بیرون می‌آید.",
        "nodes": [
            ("دستگاه‌های شما", "موبایل، لپ‌تاپ، تلویزیون،", "همهٔ خانواده"),
            ("جعبهٔ کاسپین", "یک Raspberry Pi که کانفیگِ", "پیست‌شدهٔ شما را دارد"),
            ("مودمِ خانهٔ شما", "و شرکتِ اینترنتی", "شما"),
            ("سرورِ شما", "در خارج، روی آدرسی که", "داخلِ کانفیگ است"),
            ("اینترنتِ آزاد", "", ""),
        ],
        "arrows": ["وای‌فای", "رمزگذاری‌شده", "رمزگذاری‌شده", "ترافیکِ عادی"],
        "tunnel": "تونلِ رمزگذاری‌شده. مودم و شرکتِ اینترنتی فقط یک اتصال به یک آدرس می‌بینند، نه اینکه شما چه باز می‌کنید.",
        "foot_left": "روی هیچ دستگاهی چیزی نصب نمی‌شود. فقط به یک شبکهٔ وای‌فای وصل می‌شوند، همین.",
        "foot_right": "سرورِ شما ترافیکتان را می‌بیند. سروری را به کار ببرید که به آن اعتماد دارید یا خودتان آن را می‌گردانید.",
    },
    "ru": {
        "title": "Как работает Caspian",
        "subtitle": "Одна коробка. Всё, что подключилось к её Wi-Fi, выходит с другой стороны.",
        "nodes": [
            ("Ваши устройства", "телефон, ноутбук,", "телевизор, вся семья"),
            ("Коробка Caspian", "Raspberry Pi с конфигом,", "который вы вставили"),
            ("Домашний роутер", "и ваш интернет-", "провайдер"),
            ("Ваш сервер", "за границей, по адресу", "из вашего конфига"),
            ("Открытый интернет", "", ""),
        ],
        "arrows": ["Wi-Fi", "шифрование", "шифрование", "обычный трафик"],
        "tunnel": "Зашифрованный туннель. Роутер и провайдер видят одно соединение с одним адресом, а не то, что вы открываете.",
        "foot_left": "На устройства ничего не устанавливается. Они просто подключаются к сети Wi-Fi.",
        "foot_right": "Ваш сервер видит ваш трафик. Используйте тот, которому доверяете, или свой собственный.",
    },
    "zh": {
        "title": "Caspian 的工作方式",
        "subtitle": "一个盒子。凡是连上它 Wi-Fi 的设备，流量都从另一端出去。",
        "nodes": [
            ("你的设备", "手机、笔记本、电视，", "全家人的设备"),
            ("Caspian 盒子", "一台树莓派，装着", "你粘贴的配置"),
            ("你家的路由器", "以及你的网络", "运营商"),
            ("你的服务器", "在境外，地址写在", "你的配置里"),
            ("开放的互联网", "", ""),
        ],
        "arrows": ["Wi-Fi", "已加密", "已加密", "普通流量"],
        "tunnel": "加密隧道。路由器和运营商只看到一个通往某个地址的连接，看不到你打开了什么。",
        "foot_left": "设备上不需要安装任何软件，只要连上一个 Wi-Fi 网络就可以。",
        "foot_right": "你的服务器能看到你的流量。请使用你信任的、或你自己运行的服务器。",
    },
}

W, H = 1300, 512
NODE_W, NODE_H = 178, 132
NODE_Y = 168
GAP = 78
LEFT = 49

INK = "#0f172a"
MUTED = "#5b6b7f"
CARD_BG = "#f6f8fb"
CARD_LINE = "#dbe3ec"
NODE_BG = "#ffffff"
NODE_LINE = "#c9d6e5"
BOX_BG = "#eaf3ff"
BOX_LINE = "#2f6fd0"
SRV_BG = "#eefaf1"
SRV_LINE = "#1f9254"
NET_BG = "#fff8e8"
NET_LINE = "#b9791a"
TUNNEL_BG = "#e3edfb"
TUNNEL_LINE = "#2f6fd0"


def esc(text):
    return (text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


def node_x(index, rtl):
    """Left edge of node `index`, counted along the reading direction."""
    slot = 4 - index if rtl else index
    return LEFT + slot * (NODE_W + GAP)


def build(lang):
    s = STRINGS[lang]
    rtl = lang == "fa"
    font = FONTS[lang]
    out = []
    add = out.append

    add(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="100%" '
        f'role="img" aria-labelledby="t p">')
    add(f'<title id="t">{esc(s["title"])}</title>')
    add(f'<desc id="p">{esc(s["tunnel"])}</desc>')
    add(f'<style>'
        f'.h{{font:600 27px {font};fill:{INK}}}'
        f'.sub{{font:400 16px {font};fill:{MUTED}}}'
        f'.n{{font:600 17px {font};fill:{INK}}}'
        f'.d{{font:400 13.5px {font};fill:{MUTED}}}'
        f'.a{{font:500 12.5px {font};fill:{MUTED}}}'
        f'.tun{{font:500 14.5px {font};fill:#1d4e91}}'
        f'.f{{font:400 13px {font};fill:{MUTED}}}'
        f'.p{{font:600 11.5px {font};fill:#1d4e91}}'
        + (
            # Persian is laid out right to left. Without this the bidi
            # algorithm puts a sentence-final period at the LEFT end of the
            # line, which is what the first render of this diagram did.
            '.h,.sub,.n,.d,.a,.tun,.f{direction:rtl;unicode-bidi:isolate}'
            '.p{direction:ltr;unicode-bidi:isolate}' if rtl else ''
        )
        + '</style>')

    add(f'<defs><marker id="ar" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" '
        f'markerHeight="7" orient="auto-start-reverse">'
        f'<path d="M0 0 L10 5 L0 10 z" fill="{MUTED}"/></marker></defs>')

    add(f'<rect x="0" y="0" width="{W}" height="{H}" rx="18" fill="{CARD_BG}" stroke="{CARD_LINE}"/>')

    # Always "start". Under direction:rtl the start edge IS the right edge, so
    # anchoring "end" here pushed the Persian title off the canvas.
    anchor = "start"
    tx = W - 44 if rtl else 44
    add(f'<text class="h" x="{tx}" y="60" text-anchor="{anchor}">{esc(s["title"])}</text>')
    add(f'<text class="sub" x="{tx}" y="90" text-anchor="{anchor}">{esc(s["subtitle"])}</text>')

    fills = [(NODE_BG, NODE_LINE), (BOX_BG, BOX_LINE), (NODE_BG, NODE_LINE),
             (SRV_BG, SRV_LINE), (NET_BG, NET_LINE)]

    for i, (name, l1, l2) in enumerate(s["nodes"]):
        x = node_x(i, rtl)
        bg, line = fills[i]
        width = 2 if i in (1, 3) else 1.2
        add(f'<rect x="{x}" y="{NODE_Y}" width="{NODE_W}" height="{NODE_H}" rx="13" '
            f'fill="{bg}" stroke="{line}" stroke-width="{width}"/>')
        cx = x + NODE_W / 2
        add(glyph(i, cx, NODE_Y + 30, line))
        add(f'<text class="n" x="{cx}" y="{NODE_Y + 66}" text-anchor="middle">{esc(name)}</text>')
        if l1:
            add(f'<text class="d" x="{cx}" y="{NODE_Y + 89}" text-anchor="middle">{esc(l1)}</text>')
        if l2:
            add(f'<text class="d" x="{cx}" y="{NODE_Y + 108}" text-anchor="middle">{esc(l2)}</text>')

    # Two strips under the box that actually holds the config: what it speaks,
    # and what you are allowed to paste into it.
    bcx = node_x(1, rtl) + NODE_W / 2
    for row, (text, w) in enumerate(((PROTOCOLS, 360), (INPUTS[lang], 268))):
        py = NODE_Y + NODE_H + 14 + row * 30
        add(f'<rect x="{bcx - w / 2}" y="{py}" width="{w}" height="25" rx="12.5" '
            f'fill="#ffffff" stroke="{BOX_LINE}" stroke-dasharray="3 3"/>')
        add(f'<text class="p" x="{bcx}" y="{py + 17}" text-anchor="middle">{esc(text)}</text>')

    # Arrows between consecutive nodes, drawn along the reading direction.
    for i, label in enumerate(s["arrows"]):
        a, b = node_x(i, rtl), node_x(i + 1, rtl)
        if rtl:
            x1, x2 = a, b + NODE_W
        else:
            x1, x2 = a + NODE_W, b
        y = NODE_Y + NODE_H / 2
        add(f'<line x1="{x1}" y1="{y}" x2="{x2}" y2="{y}" stroke="{MUTED}" '
            f'stroke-width="1.6" marker-end="url(#ar)"/>')
        add(f'<text class="a" x="{(x1 + x2) / 2}" y="{NODE_Y - 13}" text-anchor="middle">{esc(label)}</text>')

    # The tunnel band, spanning the box, the router and the server.
    left = min(node_x(1, rtl), node_x(3, rtl))
    right = max(node_x(1, rtl), node_x(3, rtl)) + NODE_W
    ty = NODE_Y + NODE_H + 84
    add(f'<rect x="{left}" y="{ty}" width="{right - left}" height="40" rx="10" '
        f'fill="{TUNNEL_BG}" stroke="{TUNNEL_LINE}" stroke-dasharray="6 4"/>')
    add(f'<text class="tun" x="{(left + right) / 2}" y="{ty + 25}" '
        f'text-anchor="middle">{esc(s["tunnel"])}</text>')

    fy = ty + 72
    add(f'<text class="f" x="{tx}" y="{fy}" text-anchor="{anchor}">{esc(s["foot_left"])}</text>')
    add(f'<text class="f" x="{tx}" y="{fy + 21}" text-anchor="{anchor}">{esc(s["foot_right"])}</text>')

    add('</svg>')
    return "\n".join(out) + "\n"


def glyph(index, cx, cy, color):
    """A small mark per node. Shapes rather than emoji, and no icon font."""
    c = f'fill="none" stroke="{color}" stroke-width="1.7" stroke-linecap="round"'
    if index == 0:  # devices: a phone beside a laptop
        return (f'<g {c}><rect x="{cx-25}" y="{cy-11}" width="15" height="23" rx="3"/>'
                f'<rect x="{cx-4}" y="{cy-10}" width="27" height="17" rx="2"/>'
                f'<path d="M{cx-8} {cy+10} h35"/></g>')
    if index == 1:  # the box: a board with a radiating signal
        return (f'<g {c}><rect x="{cx-20}" y="{cy-9}" width="40" height="21" rx="3"/>'
                f'<path d="M{cx-11} {cy-2} h22 M{cx-11} {cy+5} h13"/>'
                f'<path d="M{cx+16} {cy-16} a11 11 0 0 1 0 15" /></g>')
    if index == 2:  # router: a box with two antennas
        return (f'<g {c}><rect x="{cx-21}" y="{cy-3}" width="42" height="16" rx="3"/>'
                f'<path d="M{cx-11} {cy-3} v-13 M{cx+11} {cy-3} v-13"/></g>')
    if index == 3:  # server: stacked units
        return (f'<g {c}><rect x="{cx-19}" y="{cy-13}" width="38" height="12" rx="2"/>'
                f'<rect x="{cx-19}" y="{cy+2}" width="38" height="12" rx="2"/>'
                f'<path d="M{cx-13} {cy-7} h3 M{cx-13} {cy+8} h3"/></g>')
    return (f'<g {c}><circle cx="{cx}" cy="{cy}" r="14"/>'
            f'<path d="M{cx-14} {cy} h28 M{cx} {cy-14} a20 20 0 0 0 0 28 '
            f'a20 20 0 0 0 0 -28"/></g>')


def main():
    here = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    outdir = os.path.join(here, "docs", "images")
    os.makedirs(outdir, exist_ok=True)
    for lang in ("en", "fa", "ru", "zh"):
        path = os.path.join(outdir, f"flow-{lang}.svg")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(build(lang))
        print(f"wrote {os.path.relpath(path, here)}")


if __name__ == "__main__":
    main()
