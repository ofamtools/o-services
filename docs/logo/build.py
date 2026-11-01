#!/usr/bin/env python3
"""Build and reproduce the o-services logos in assets/.

Outputs:
  assets/icon.svg
  assets/icon-animated.svg
  assets/logo-light.svg
  assets/logo-dark.svg
  assets/logo-animated-light.svg
  assets/logo-animated-dark.svg
"""
import pathlib

TOOL = "o-services"
ASSETS = pathlib.Path(__file__).resolve().parents[2] / "assets"
ASSETS.mkdir(parents=True, exist_ok=True)

# Wordmark glyphs matching the o-family standard (11-wide strokes, baseline 84)
B, X, A = 84, 44, 18
R = 20
CY = 64
k = R * 0.7071

def f(v): return ('%.1f' % v).rstrip('0').rstrip('.')

def glyph(ch, x):
    c = x + R
    if ch == 'o':
        return f"M{f(c)} {f(X)}a{f(R)} {f(R)} 0 1 0 0.01 0", 2 * R
    if ch == '-':
        return f"M{f(x)} {f(CY)}h18", 18
    if ch == 's':
        w = 30
        return (f"M{f(x+w-1)} {f(X+6)}C{f(x+w-4)} {f(X+1)} {f(x+21)} {X} {f(x+15)} {X}"
                f"C{f(x+7)} {X} {f(x+1)} {f(X+4)} {f(x+1)} {f(X+10)}"
                f"C{f(x+1)} {f(CY-3)} {f(x+8)} {f(CY-1)} {f(x+15)} {f(CY)}"
                f"C{f(x+22)} {f(CY+1)} {f(x+w)} {f(CY+3)} {f(x+w)} {f(B-10)}"
                f"C{f(x+w)} {f(B-4)} {f(x+23)} {B} {f(x+15)} {B}"
                f"C{f(x+8)} {B} {f(x+2)} {f(B-2)} {f(x)} {f(B-7)}"), w
    if ch == 'e':
        return f"M{f(x)} {f(CY)}H{f(x+2*R)}A{f(R)} {f(R)} 0 1 0 {f(c+k)} {f(CY+k)}", 2 * R
    if ch == 'r':
        return f"M{f(x)} {X}V{B}M{f(x)} {f(X+20)}a20 20 0 0 1 20 -20h4", 24
    if ch == 'v':
        w = 34
        return f"M{f(x)} {X}L{f(x+w/2)} {B}L{f(x+w)} {X}", w
    if ch == 'i':
        return f"M{f(x)} {X}V{B}M{f(x)} {A+5}h0", 0
    if ch == 'c':
        return f"M{f(c+k)} {f(CY-k)}A{f(R)} {f(R)} 0 1 0 {f(c+k)} {f(CY+k)}", R + k
    raise ValueError(ch)

def wordmark(x0, text="o-services", gap=15):
    d = []; x = x0
    for ch in text:
        p, w = glyph(ch, x); d.append(p); x += w + gap
    return "".join(d), x - gap

wm_path, end_wm = wordmark(122, "o-services")
W = 558

# Palette
c_dark_box = "#0c1524"
c_dark_pri = "#0284c7"
c_dark_sec = "#38bdf8"
c_dark_ink = "#f8fafc"

c_light_box = "#f0f9ff"
c_light_pri = "#0284c7"
c_light_sec = "#0369a1"
c_light_cut = "#ffffff"
c_light_ink = "#0f172a"

def icon_mark(c_pri, c_sec, c_cut, animated=False):
    anim_style = ""
    if animated:
        anim_style = """<style>
.w1,.w2,.w3,.led{transform-box:fill-box;transform-origin:50% 100%}
@media (prefers-reduced-motion: no-preference){
  .w1{animation:pulse-w1 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .w2{animation:pulse-w2 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .w3{animation:pulse-w3 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .led{animation:pulse-led 3.2s cubic-bezier(.45,0,.25,1) infinite}
}
@keyframes pulse-w1{0%,100%{opacity:.35;transform:scale(0.96)}20%{opacity:1;transform:scale(1.04)}45%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-w2{0%,100%{opacity:.35;transform:scale(0.96)}10%{opacity:.35;transform:scale(0.96)}30%{opacity:1;transform:scale(1.04)}55%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-w3{0%,100%{opacity:.35;transform:scale(0.96)}20%{opacity:.35;transform:scale(0.96)}40%{opacity:1;transform:scale(1.04)}65%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-led{0%,100%{opacity:.6}20%,40%{opacity:1}60%{opacity:.6}}
</style>"""
    return f"""{anim_style}<g>
<path class="w3" d="M 24 24 A 52 52 0 0 1 104 24" fill="none" stroke="{c_sec}" stroke-width="8" stroke-linecap="round"/>
<path class="w2" d="M 36 38 A 36 36 0 0 1 92 38" fill="none" stroke="{c_sec}" stroke-width="8" stroke-linecap="round"/>
<path class="w1" d="M 48 52 A 20 20 0 0 1 80 52" fill="none" stroke="{c_sec}" stroke-width="8" stroke-linecap="round"/>
<rect x="36" y="66" width="56" height="38" rx="10" fill="{c_pri}"/>
<path d="M46 82h36v14a4 4 0 0 1-4 4H50a4 4 0 0 1-4-4V82z" fill="{c_cut}"/>
<circle class="led" cx="54" cy="74" r="3.5" fill="{c_sec}"/>
<circle class="led" cx="74" cy="74" r="3.5" fill="{c_sec}"/>
</g>"""

def logo_mark(c_pri, c_sec, c_cut, animated=False):
    anim_style = ""
    if animated:
        anim_style = """<style>
.w1,.w2,.w3,.led{transform-box:fill-box;transform-origin:50% 100%}
@media (prefers-reduced-motion: no-preference){
  .w1{animation:pulse-w1 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .w2{animation:pulse-w2 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .w3{animation:pulse-w3 3.2s cubic-bezier(.45,0,.25,1) infinite}
  .led{animation:pulse-led 3.2s cubic-bezier(.45,0,.25,1) infinite}
}
@keyframes pulse-w1{0%,100%{opacity:.35;transform:scale(0.96)}20%{opacity:1;transform:scale(1.04)}45%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-w2{0%,100%{opacity:.35;transform:scale(0.96)}10%{opacity:.35;transform:scale(0.96)}30%{opacity:1;transform:scale(1.04)}55%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-w3{0%,100%{opacity:.35;transform:scale(0.96)}20%{opacity:.35;transform:scale(0.96)}40%{opacity:1;transform:scale(1.04)}65%{opacity:.35;transform:scale(0.96)}}
@keyframes pulse-led{0%,100%{opacity:.6}20%,40%{opacity:1}60%{opacity:.6}}
</style>"""
    return f"""{anim_style}<g>
<path class="w3" d="M 22 18 A 38 38 0 0 1 82 18" fill="none" stroke="{c_sec}" stroke-width="7" stroke-linecap="round"/>
<path class="w2" d="M 32 30 A 26 26 0 0 1 72 30" fill="none" stroke="{c_sec}" stroke-width="7" stroke-linecap="round"/>
<path class="w1" d="M 42 42 A 14 14 0 0 1 62 42" fill="none" stroke="{c_sec}" stroke-width="7" stroke-linecap="round"/>
<rect x="28" y="54" width="48" height="32" rx="8" fill="{c_pri}"/>
<path d="M36 68h32v12a3 3 0 0 1-3 3H39a3 3 0 0 1-3-3V68z" fill="{c_cut}"/>
<circle class="led" cx="44" cy="61" r="2.8" fill="{c_sec}"/>
<circle class="led" cx="60" cy="61" r="2.8" fill="{c_sec}"/>
</g>"""

def main():
    (ASSETS / "icon.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128" role="img" aria-label="o-services">
<title>o-services</title>
<rect width="128" height="128" rx="28" fill="{c_dark_box}"/>
{icon_mark(c_dark_pri, c_dark_sec, c_dark_box, animated=False)}
</svg>""")

    (ASSETS / "icon-animated.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128" role="img" aria-label="o-services">
<title>o-services</title>
<rect width="128" height="128" rx="28" fill="{c_dark_box}"/>
{icon_mark(c_dark_pri, c_dark_sec, c_dark_box, animated=True)}
</svg>""")

    (ASSETS / "logo-dark.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} 96" width="{W}" height="96" role="img" aria-label="o-services">
<title>o-services</title>
{logo_mark(c_dark_pri, c_dark_sec, c_dark_box, animated=False)}
<path d="{wm_path}" fill="none" stroke="{c_dark_ink}" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/>
</svg>""")

    (ASSETS / "logo-animated-dark.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} 96" width="{W}" height="96" role="img" aria-label="o-services">
<title>o-services</title>
{logo_mark(c_dark_pri, c_dark_sec, c_dark_box, animated=True)}
<path d="{wm_path}" fill="none" stroke="{c_dark_ink}" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/>
</svg>""")

    (ASSETS / "logo-light.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} 96" width="{W}" height="96" role="img" aria-label="o-services">
<title>o-services</title>
{logo_mark(c_light_pri, c_light_sec, c_light_cut, animated=False)}
<path d="{wm_path}" fill="none" stroke="{c_light_ink}" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/>
</svg>""")

    (ASSETS / "logo-animated-light.svg").write_text(f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} 96" width="{W}" height="96" role="img" aria-label="o-services">
<title>o-services</title>
{logo_mark(c_light_pri, c_light_sec, c_light_cut, animated=True)}
<path d="{wm_path}" fill="none" stroke="{c_light_ink}" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/>
</svg>""")

    print(f"Generated SVGs in {ASSETS}")

if __name__ == "__main__":
    main()
