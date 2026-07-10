#!/usr/bin/env python3
"""Generate the documentation diagrams as PNG files into docs/static/img/.

Usage:
    python3 generate.py

Requires Pillow. Colors follow the gruvbox dark palette so the diagrams look
native on the site's dark theme while staying readable on light backgrounds.
"""

from __future__ import annotations

import os

from PIL import Image, ImageDraw, ImageFont

OUT_DIR = os.path.join(os.path.dirname(__file__), "..", "static", "img")

# gruvbox dark palette
BG = "#282828"
BG_SOFT = "#3c3836"
BG_HARD = "#504945"
FG = "#ebdbb2"
FG_DIM = "#bdae93"
GRAY = "#928374"
BLUE = "#83a598"
YELLOW = "#fabd2f"
GREEN = "#b8bb26"
RED = "#fb4934"
AQUA = "#8ec07c"
ORANGE = "#fe8019"

FONT_PATH = "/System/Library/Fonts/Helvetica.ttc"
MONO_PATH = "/System/Library/Fonts/Menlo.ttc"


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(FONT_PATH, size, index=1 if bold else 0)


def mono(size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(MONO_PATH, size)


def new_canvas(w: int, h: int) -> tuple[Image.Image, ImageDraw.ImageDraw]:
    img = Image.new("RGB", (w, h), BG)
    return img, ImageDraw.Draw(img)


def box(d, x, y, w, h, lines, *, fill=BG_SOFT, outline=BLUE, fnt=None,
        color=FG, radius=16, width=4):
    d.rounded_rectangle([x, y, x + w, y + h], radius=radius, fill=fill,
                        outline=outline, width=width)
    fnt = fnt or font(30)
    fonts = [fnt] * len(lines) if not isinstance(fnt, list) else fnt
    total = sum(f.size + 10 for f in fonts) - 10
    ty = y + (h - total) / 2
    for line, f in zip(lines, fonts):
        tw = d.textlength(line, font=f)
        d.text((x + (w - tw) / 2, ty), line, font=f, fill=color)
        ty += f.size + 10


def arrow(d, x1, y1, x2, y2, *, color=FG_DIM, width=5, label=None,
          label_font=None, label_color=None, head=18):
    d.line([x1, y1, x2, y2], fill=color, width=width)
    # arrowhead
    import math
    ang = math.atan2(y2 - y1, x2 - x1)
    for da in (math.radians(155), math.radians(-155)):
        hx = x2 + head * math.cos(ang + da)
        hy = y2 + head * math.sin(ang + da)
        d.line([x2, y2, hx, hy], fill=color, width=width)
    if label:
        f = label_font or font(24)
        tw = d.textlength(label, font=f)
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        d.rectangle([mx - tw / 2 - 8, my - f.size / 2 - 6,
                     mx + tw / 2 + 8, my + f.size / 2 + 10], fill=BG)
        d.text((mx - tw / 2, my - f.size / 2), label, font=f,
               fill=label_color or FG_DIM)


def title(d, w, text, sub=None):
    f = font(40, bold=True)
    tw = d.textlength(text, font=f)
    d.text(((w - tw) / 2, 30), text, font=f, fill=FG)
    if sub:
        fs = font(26)
        tw = d.textlength(sub, font=fs)
        d.text(((w - tw) / 2, 84), sub, font=fs, fill=FG_DIM)


def save(img: Image.Image, name: str):
    os.makedirs(OUT_DIR, exist_ok=True)
    path = os.path.join(OUT_DIR, name)
    img.save(path, "PNG", optimize=True)
    print(f"wrote {path}")


def architecture():
    W, H = 1700, 1250
    img, d = new_canvas(W, H)
    title(d, W, "kumokagi architecture",
          "one interface, five backends, ambient credentials only")

    # Row 1: application
    box(d, 550, 140, 600, 90, ["Application code"], fnt=font(34, bold=True),
        outline=FG_DIM)

    # Row 2: config layer
    box(d, 250, 300, 560, 110,
        ["viper (Go)", "pydantic-settings (Python)"], outline=AQUA)
    box(d, 950, 300, 560, 110,
        ["KumokagiSource", "Load() / Verify()"], outline=AQUA)
    arrow(d, 850, 200, 530, 295, label="reads config values")
    arrow(d, 850, 200, 1230, 295, label="wires source once")
    arrow(d, 945, 355, 815, 355, label="v.Set(key, value)")

    # Row 3: factory
    box(d, 550, 490, 600, 100,
        ["factory.New(ctx, cfg)", "backend registry — database/sql driver pattern"],
        fnt=[font(32, bold=True), font(24)], outline=YELLOW)
    arrow(d, 1230, 415, 1230, 460)
    d.line([1230, 460, 1230, 540, 1155, 540], fill=FG_DIM, width=5)

    # Row 4: providers
    providers = ["Vault", "AWS", "Azure", "GCP", "1Password"]
    backends = ["HashiCorp Vault", "AWS Secrets", "Azure Key Vault",
                "GCP Secret", "1Password CLI"]
    backends2 = ["KV v2", "Manager", "", "Manager", "(op)"]
    bw, gap = 290, 40
    total = 5 * bw + 4 * gap
    x0 = (W - total) / 2
    for i, p in enumerate(providers):
        x = x0 + i * (bw + gap)
        box(d, x, 720, bw, 90, [f"{p} Provider"], outline=BLUE, fnt=font(28))
        arrow(d, 850, 595, x + bw / 2, 715)
        lines = [backends[i]] + ([backends2[i]] if backends2[i] else [])
        box(d, x, 950, bw, 100, lines, outline=ORANGE, fill=BG_HARD,
            fnt=font(26))
        arrow(d, x + bw / 2, 815, x + bw / 2, 945)

    f = font(24)
    note = "auth: ambient credentials — VAULT_TOKEN · IRSA · Workload Identity · ADC · op signin"
    tw = d.textlength(note, font=f)
    d.text(((W - tw) / 2, 1120), note, font=f, fill=GRAY)
    f2 = mono(22)
    note2 = "import _ \"github.com/stlimtat/kumokagi/pkg/providers/aws\"   // links only the SDKs you use"
    tw = d.textlength(note2, font=f2)
    d.text(((W - tw) / 2, 1165), note2, font=f2, fill=GRAY)
    save(img, "architecture.png")


def rotation():
    W, H = 1700, 1150
    img, d = new_canvas(W, H)
    title(d, W, "Surviving credential rotation",
          "no cache — every fetch returns the current value")

    # lifelines
    ax, bx = 430, 1270  # lane centers
    box(d, ax - 260, 140, 520, 80, ["Application (long-lived pod)"],
        fnt=font(30, bold=True), outline=AQUA)
    box(d, bx - 260, 140, 520, 80, ["Secrets backend"],
        fnt=font(30, bold=True), outline=ORANGE)
    for x in (ax, bx):
        d.line([x, 220, x, 1050], fill=GRAY, width=3)

    def step(y, txt, color=FG_DIM):
        f = font(28, bold=True)
        d.ellipse([60, y - 22, 104, y + 22], outline=color, width=4)
        n = txt
        d.text((75, y - 16), n, font=f, fill=color)

    fm = mono(24)

    # 1. initial fetch
    step(290, "1", GREEN)
    arrow(d, ax, 290, bx - 5, 290, label="Get(prod/myapp/db_password)",
          label_font=fm, color=GREEN, label_color=GREEN)
    arrow(d, bx, 350, ax + 5, 350, label='returns "v1"', label_font=fm,
          color=GREEN, label_color=GREEN)
    d.text((ax + 30, 385), "app connects to database with v1",
           font=font(24), fill=FG_DIM)

    # 2. rotation happens
    step(500, "2", YELLOW)
    box(d, bx - 290, 460, 580, 80,
        ["secret rotated: v1 -> v2", "(kumokagi rotate / cloud console / operator)"],
        fnt=[font(28, bold=True), font(22)], outline=YELLOW, color=YELLOW)

    # 3. auth failure
    step(640, "3", RED)
    box(d, ax - 290, 600, 580, 80,
        ["database rejects v1", "authentication failure caught by app"],
        fnt=[font(28, bold=True), font(22)], outline=RED, color=RED)

    # 4. re-fetch
    step(790, "4", GREEN)
    arrow(d, ax, 790, bx - 5, 790, label="Get(prod/myapp/db_password)",
          label_font=fm, color=GREEN, label_color=GREEN)
    arrow(d, bx, 850, ax + 5, 850, label='returns "v2"  (always fresh)',
          label_font=fm, color=GREEN, label_color=GREEN)

    # 5. reconnect
    step(950, "5", AQUA)
    box(d, ax - 290, 910, 580, 80,
        ["reconnect with v2", "no restart · no redeploy · no manifest change"],
        fnt=[font(28, bold=True), font(22)], outline=AQUA, color=AQUA)

    save(img, "rotation.png")


def auth_chain(name, subtitle, steps, note):
    """Horizontal 4-box auth flow: steps interleaved
    [box, arrow-label-lines, box, arrow-label-lines, box, ...].
    Arrow labels are drawn above the arrow so they never cover the boxes."""
    boxes = steps[::2]
    labels = steps[1::2]
    n = len(boxes)
    bw, bh, gap = 330, 130, 210
    W = n * bw + (n - 1) * gap + 160
    H = 500
    img, d = new_canvas(W, H)
    title(d, W, subtitle[0], subtitle[1])
    y = 210
    x = 80
    fm = mono(20)
    for i, (lines, colr) in enumerate(boxes):
        box(d, x, y, bw, bh, lines, outline=colr,
            fnt=[font(26, bold=True)] + [font(22)] * (len(lines) - 1))
        if i < n - 1:
            mid_y = y + bh / 2
            arrow(d, x + bw + 8, mid_y, x + bw + gap - 8, mid_y)
            label_lines = labels[i]
            if isinstance(label_lines, str):
                label_lines = [label_lines]
            ly = mid_y - 16 - len(label_lines) * 26
            for line in label_lines:
                tw = d.textlength(line, font=fm)
                d.text((x + bw + gap / 2 - tw / 2, ly), line, font=fm,
                       fill=FG_DIM)
                ly += 26
        x += bw + gap
    f = font(24)
    tw = d.textlength(note, font=f)
    d.text(((W - tw) / 2, 410), note, font=f, fill=GRAY)
    save(img, name)


def main():
    architecture()
    rotation()

    auth_chain(
        "auth-aws.png",
        ("AWS — IRSA authentication flow",
         "IAM Roles for Service Accounts: the pod's identity is exchanged for temporary IAM credentials"),
        [
            (["Pod", "projected SA token", "(OIDC JWT)"], AQUA),
            ["AssumeRoleWith", "WebIdentity"],
            (["AWS STS", "validates token against", "EKS OIDC provider"], BLUE),
            ["temporary", "credentials"],
            (["AWS SDK", "default credential", "chain"], YELLOW),
            ["GetSecretValue"],
            (["Secrets Manager", "prod/myapp/", "db_password"], ORANGE),
        ],
        "no AWS_ACCESS_KEY_ID · no AWS_SECRET_ACCESS_KEY · credentials expire automatically",
    )

    auth_chain(
        "auth-azure.png",
        ("Azure — Workload Identity flow",
         "a federated Kubernetes token is exchanged for an Entra ID access token"),
        [
            (["Pod", "projected SA token", "(federated JWT)"], AQUA),
            ["federated", "credential"],
            (["Microsoft", "Entra ID", "token exchange"], BLUE),
            "access token",
            (["DefaultAzure-", "Credential", "azure-sdk chain"], YELLOW),
            "GetSecret",
            (["Azure Key Vault", "prod--myapp--", "db_password"], ORANGE),
        ],
        "no client secret · no connection string · token scoped to the vault",
    )

    auth_chain(
        "auth-gcp.png",
        ("GCP — Workload Identity Federation flow",
         "the GKE metadata server issues tokens for the bound Google service account"),
        [
            (["Pod", "Kubernetes SA", "(KSA)"], AQUA),
            ["metadata", "server"],
            (["GKE Workload", "Identity", "KSA-to-GSA binding"], BLUE),
            ["OAuth2", "access token"],
            (["Application", "Default", "Credentials (ADC)"], YELLOW),
            ["AccessSecret-", "Version"],
            (["Secret Manager", "prod--myapp--", "db_password"], ORANGE),
        ],
        "no JSON key file · token minted on demand by the metadata server",
    )

    auth_chain(
        "auth-vault.png",
        ("Vault — Kubernetes auth flow",
         "the pod's service account JWT is exchanged for a short-lived Vault token"),
        [
            (["Pod", "service account", "JWT"], AQUA),
            ["login", "(k8s auth)"],
            (["Vault", "kubernetes auth", "method"], BLUE),
            ["VAULT_TOKEN", "(TTL)"],
            (["Vault SDK", "token from env or", "~/.vault-token"], YELLOW),
            "KV v2 read",
            (["Vault KV v2", "secret/data/prod/", "myapp/db_password"], ORANGE),
        ],
        "policy: path \"secret/data/prod/myapp/*\" { capabilities = [\"read\"] }",
    )

    auth_chain(
        "auth-onepassword.png",
        ("1Password — CLI session flow",
         "a human signs in once; kumokagi shells out to the op CLI"),
        [
            (["Engineer", "op signin", "(biometric / master pw)"], AQUA),
            ["session", "token"],
            (["op CLI", "session in", "keychain"], BLUE),
            "op item get",
            (["kumokagi", "onepassword", "provider"], YELLOW),
            ["field:", "password"],
            (["1Password vault", "item: prod--", "myapp--db_password"], ORANGE),
        ],
        "no infrastructure to run · best for solo devs and small teams",
    )


if __name__ == "__main__":
    main()
