# vpn_server

Go + SQLite backend deployed on AWS Lightsail.

## Runtime

- Public host: `http://<server-ip-or-domain>`
- API process: `127.0.0.1:8080`
- Public entry: Nginx on port `80`
- SQLite database: `/var/lib/go-sqlite-api/app.db`
- systemd service: `go-sqlite-api`

## Endpoints

- `GET /` game official site
- `GET /health`
- `GET /api/version`
- `GET /api/items`
- `POST /api/items` with `{"title":"first item"}`
- `PATCH /api/items/{id}` with `{"done":true}` or `{"title":"new title"}`
- `DELETE /api/items/{id}`

## Local Development

```bash
go mod tidy
ADDR=127.0.0.1:8080 DB_PATH=./app.db go run .
```

Open:

```text
http://127.0.0.1:8080/
```

## Deploy

The deploy script copies the current checkout to the Lightsail server, builds it there, updates systemd and Nginx, then verifies `/health`.

Create the VPN runtime environment file on the server before applying VPN configuration:

```bash
sudo install -d -m 755 /etc/go-sqlite-api
sudo cp /opt/apps/go-sqlite-api/deploy/vpn.env.example /etc/go-sqlite-api/vpn.env
sudo chmod 600 /etc/go-sqlite-api/vpn.env
sudo nano /etc/go-sqlite-api/vpn.env
```

Keep real values out of git. The tracked `deploy/vpn.env.example` file is only a template.

```bash
./deploy/deploy.sh
```

Optional environment overrides:

```bash
REMOTE_HOST=<server-ip-or-domain> \
REMOTE_USER=ubuntu \
SSH_KEY=~/.ssh/LightsailDefaultKey-ap-northeast-1.pem \
./deploy/deploy.sh
```

## Server Commands

```bash
ssh -i ~/.ssh/LightsailDefaultKey-ap-northeast-1.pem ubuntu@<server-ip-or-domain>

sudo systemctl status go-sqlite-api
sudo journalctl -u go-sqlite-api -f
sudo systemctl restart go-sqlite-api
```

## Smoke Test

```bash
curl http://<server-ip-or-domain>/health
curl -X POST http://<server-ip-or-domain>/api/items \
  -H 'Content-Type: application/json' \
  -d '{"title":"first sqlite item"}'
curl http://<server-ip-or-domain>/api/items
```
