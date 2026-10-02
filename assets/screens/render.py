"""Draws the README's terminal pictures as SVG windows in ask's look: black, white and greys, with
color only where it means something. Edit the transcripts at the bottom, then run
python3 assets/screens/render.py assets/screens from the repository root."""
import html, sys

FG, DIM, BOLD, GREEN, RED, BG, BAR = "#e6e6e6", "#7d7d7d", "#ffffff", "#3fb950", "#f85149", "#0e0e0e", "#1a1a1a"
BANDS = {"add": ("#12261e", "#aff5b4"), "del": ("#2d1215", "#ffc0c0"), "file": ("#262626", "#ffffff")}
CW, LH, PADX, TOP = 8.4, 21, 22, 52

def render(lines, width_chars, title, path):
    w = int(PADX * 2 + width_chars * CW)
    h = TOP + len(lines) * LH + 4
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">',
           f'<rect width="{w}" height="{h}" rx="10" fill="{BG}"/>',
           f'<path d="M0 10 a10 10 0 0 1 10 -10 h{w-20} a10 10 0 0 1 10 10 v24 h-{w} z" fill="{BAR}"/>']
    for i, x in enumerate((18, 36, 54)):
        out.append(f'<circle cx="{x}" cy="17" r="5.5" fill="#3a3a3a"/>')
    out.append(f'<text x="{w/2}" y="21.5" fill="{DIM}" font-size="12.5" text-anchor="middle" font-family="ui-sans-serif, -apple-system, Segoe UI, sans-serif">{html.escape(title)}</text>')
    out.append(f'<g font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="14">')
    for n, line in enumerate(lines):
        y = TOP + n * LH
        band = None
        if line and isinstance(line[0], str) and line[0] in BANDS:
            band, line = line[0], line[1:]
            bg, _ = BANDS[band]
            out.append(f'<rect x="{PADX-8}" y="{y-15}" width="{w-2*PADX+16}" height="{LH}" fill="{bg}"/>')
        x = PADX
        parts = []
        for seg in line:
            text, style = (seg, "") if isinstance(seg, str) else seg
            fill, weight = FG, "400"
            if band:
                fill = BANDS[band][1]
            if style == "dim": fill = DIM
            if style == "bold": fill, weight = BOLD, "600"
            if style == "green": fill = GREEN
            if style == "red": fill = RED
            if style == "prompt": fill = DIM
            lead = len(text) - len(text.lstrip(" "))
            body = text.lstrip(" ")
            if body:
                t = html.escape(body).replace(" ", "&#160;")
                parts.append(f'<tspan x="{x + lead * CW:.1f}" fill="{fill}" font-weight="{weight}">{t}</tspan>')
            x += len(text) * CW
        out.append(f'<text y="{y}">{"".join(parts)}</text>')
    out.append('</g></svg>')
    open(path, "w").write("\n".join(out) + "\n")

D = lambda s: (s, "dim")
B = lambda s: (s, "bold")
G = lambda s: (s, "green")
R = lambda s: (s, "red")
P = ("$ ", "prompt")

dest = sys.argv[1]
render([
    [P, B('ask -m claude:sonnet-5.5 "Why does the login test fail?"')],
    ["ask login-test-fail · ", G("ok"), D(" · Sonnet 5.5 · 48.0s · 31.0k in · 812 out · $0.09")],
    ["The expiry check compares seconds with milliseconds, auth/token.go:42."],
    [""],
    [P, B('ask -c login-test-fail -w "Fix it, then run the test."')],
    ["ask login-test-fail · ", G("ok"), D(" · Sonnet 5.5 · 1:12 · 1 file changed · 44.2k in · 1.3k out · $0.14")],
    ["Fixed: the expiry is now in milliseconds. go test ./auth passes."],
], 92, "ask", f"{dest}/run.svg")

render([
    [P, B("ask batch -w --worktree review.json")],
    [B("ask review-session-code · batch of 3 · 3 at a time")],
    ["✔ ", "claude    ", "Sonnet 5.5          ", D("2:31 · 2 files changed · 61.2k in · 2.1k out · $0.24")],
    ["✔ ", "codex     ", "GPT-6.1 Sol         ", D("3:05 · 3 files changed · 88.4k in · 3.0k out")],
    [D("⠹ "), "opencode  ", "Deepseek 4.1 Flash  ", D("write · worktree · 1:48 · 23.9k in · 640 out · $0.004")],
    [D("2/3 ok · 3:05 · 173.5k in · 5.7k out · $0.24")],
], 92, "ask batch", f"{dest}/batch.svg")

render([
    [B("ask settings"), D("  ~/.ask")],
    [""],
    [D("✔ "), "What do you want to change? ", B("Review and save")],
    [""],
    ["file", " ~/.ask/settings.json  ", G("+2"), " ", R("-1")],
    ["del", '    "timeout": 900'],
    ["add", '    "timeout": 1800,'],
    ["add", '    "max_cost": 2'],
    [""],
    ["file", " ~/.ask/models.json  ", G("+1")],
    ["add", '    "gpt-4o": false'],
    [""],
    ["2 files changed, ", G("3 insertions(+)"), ", ", R("1 deletion(-)")],
    [D("? "), B("Save these changes?"), D(" (Y/n)")],
], 92, "ask settings", f"{dest}/settings.svg")
