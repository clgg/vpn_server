#!/usr/bin/env python3
import json, re, subprocess, sys, urllib.parse

def sh(*args):
    return subprocess.check_output(list(args), text=True)

cookie = None
for line in sh("curl", "-s", "-c", "-", "-X", "POST", "http://127.0.0.1:8080/api/auth/login",
                 "-H", "Content-Type: application/json",
                 "-d", '{"email":"chenlg3273099@gmail.com","password":"chenLI_3273099"}').splitlines():
    if "vpn_session" in line:
        cookie = line.split("\t")[-1].strip()
if not cookie:
    sys.exit("login failed")

H = ["-H", "Host: 54.150.9.209", "-b", f"vpn_session={cookie}"]

def get(path):
    return sh("curl", "-s", *H, f"http://127.0.0.1:8080{path}")

users = {u["name"]: u for u in json.loads(get("/api/vpn/users"))["users"]}

def uuid_from(body, kind):
    if kind == "vless":
        return re.search(r"vless://([0-9a-f-]{36})@", body).group(1)
    if kind == "clash":
        return re.search(r"uuid: ([0-9a-f-]{36})", body).group(1)
    if kind in ("clash-import", "sing-box-import"):
        url = urllib.parse.unquote(re.search(r"url=([^#&]+)", body).group(1))
        return re.search(r"/([0-9a-f-]{36})/", url).group(1)
    if kind == "rocket-import":
        url = urllib.parse.unquote(re.search(r"url=([^&]+)", body).group(1))
        return re.search(r"/profiles/([0-9a-f-]{36})", url).group(1)
    if kind in ("sing-box", "rocket-json"):
        d = json.loads(body)
        if kind == "rocket-json":
            return d["profile_id"]
        return d["outbounds"][0]["uuid"]
    raise ValueError(kind)

checks = [
    ("vless", "/api/vpn/configs/{id}/vless"),
    ("clash", "/api/vpn/configs/{id}/clash"),
    ("clash-import", "/api/vpn/configs/{id}/clash-import"),
    ("sing-box", "/api/vpn/configs/{id}/sing-box"),
    ("sing-box-import", "/api/vpn/configs/{id}/sing-box-import"),
    ("rocket-import", "/api/vpn/configs/{id}/rocket-import"),
    ("rocket-json", "/api/vpn/configs/{id}/rocket"),
]

print("AUDIT owner vs clg_phone")
for name in ["owner", "clg_phone"]:
    u = users[name]
    print(f"\n[{name}] db_id={u['id']} uuid={u['uuid']}")
    for kind, tmpl in checks:
        body = get(tmpl.format(id=u["id"]))
        got = uuid_from(body, kind)
        status = "OK" if got == u["uuid"] else "MISMATCH"
        print(f"  {kind:18} {status} -> {got}")

# cross-check public endpoints
print("\nPUBLIC endpoints by UUID")
for name in ["owner", "clg_phone"]:
    u = users[name]
    for kind, suffix in [("clash.yaml", "clash"), ("sing-box.json", "sing-box")]:
        body = sh("curl", "-s", f"http://127.0.0.1:8080/api/vpn/public/{u['uuid']}/{kind}")
        if suffix == "clash":
            got = re.search(r"uuid: ([0-9a-f-]{36})", body).group(1)
            pname = re.search(r"- name: (\S+)", body).group(1)
            print(f"  {name} public/{kind}: uuid={got} proxy={pname} {'OK' if got==u['uuid'] else 'MISMATCH'}")
        else:
            got = json.loads(body)["outbounds"][0]["uuid"]
            tag = json.loads(body)["outbounds"][0]["tag"]
            print(f"  {name} public/{kind}: uuid={got} tag={tag} {'OK' if got==u['uuid'] else 'MISMATCH'}")
