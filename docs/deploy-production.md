# Deploy production — Dashboard Sales

Dokumen serah-terima untuk tim DevOps (dan pengembang). Standar tim: `esb-ops-dev-stack`
(GitHub Actions → GHCR → SSH → docker compose). Hanya ada **satu environment: production**.

## 1. Gambaran

```
Browser ──HTTPS──▶ reverse proxy DevOps ──▶ 127.0.0.1:${PUBLISH_PORT}
                                             │
                        ┌────────────────────┘   (docker compose, jaringan internal)
                        ▼
                  web-app (nginx :80)  ── /api/*, /auth/*, /healthz ──▶ api (Go :8080) ──▶ db (postgres:16)
                        │
                        └── menyajikan Angular (SPA)
```

- Hanya **web-app** yang membuka port ke host (`PUBLISH_PORT`). `api` dan `db` tidak dipublish.
- `api` menjalankan sendiri **scheduler sinkron ESB** (tiap hari 06:00 WIB) di dalam prosesnya —
  tidak perlu cron terpisah. Karena itu **jalankan hanya satu instance `api`**.
- Login memakai cookie sesi (`HttpOnly`); sesi tersimpan di database.

## 2. Yang perlu disiapkan DevOps

| # | Kebutuhan | Catatan |
|---|---|---|
| 1 | Server Linux amd64 dengan Docker + plugin Compose | |
| 2 | Folder `/opt/apps/<slug>/` | `<slug>` = nama repo (huruf kecil, mis. `dashboard-sales`) |
| 3 | User deploy + SSH key khusus | Private key → GitHub Secret `SSH_PRIVATE_KEY` |
| 4 | `docker login ghcr.io` di server, sekali | Token dengan scope `read:packages` |
| 5 | Domain/subdomain + reverse proxy → `127.0.0.1:${PUBLISH_PORT}` | Lihat §3 |
| 6 | `PUBLISH_PORT` | Port host yang bebas |
| 7 | GitHub Secrets di repo | `SSH_HOST`, `SSH_USER`, `SSH_PRIVATE_KEY`, opsional `SSH_PORT` (default 22) |
| 8 | `.env` production di server | Template: `.env.production.example` (§4) |
| 9 | Backup rutin volume database (`pgdata`) | Lihat §7 — backup bawaan `deploy.sh` **tidak cukup** |

Permintaan ke **Eka** (bukan DevOps): repo di org **Operations-ESB** dengan nama huruf kecil,
akses contributor, dan (nanti) client operations-sso production.

Isi folder di server:

```
/opt/apps/<slug>/
├── docker-compose.prod.yml   (salin dari repo)
├── deploy.sh                 (salin dari repo; chmod +x)
├── .env                      (HANYA di server, jangan di-commit)
└── backups/                  (dibuat otomatis oleh deploy.sh)
```

## 3. Reverse proxy

- **TLS di-terminate di proxy DevOps.** Aplikasi mengeluarkan cookie sesi ber-flag `Secure`
  (default saat `APP_ENV=production`), jadi wajib HTTPS. Kalau sementara dilayani lewat HTTP
  polos, set `COOKIE_SECURE=false` di `.env` — kalau tidak, browser membuang cookie dan
  halaman login akan **berulang terus** (tidak ada pesan error).
- Teruskan header `Host` dan `X-Forwarded-For`. **Sebaiknya proxy MENGGANTI (bukan menambah)
  `X-Forwarded-For` dengan IP klien asli** (`proxy_set_header X-Forwarded-For $remote_addr;`
  di nginx). API memakai IP itu untuk membatasi percobaan login; nilai yang bisa dipalsukan
  klien melemahkan batas per-IP (batas per-email tetap berlaku).
- Tidak ada WebSocket/streaming. Request terpanjang (sinkron manual) berjalan di background,
  jadi timeout proxy standar cukup.
- Pemantauan: `GET /healthz` lewat proxy → `200 ok` dari API (bukan sekadar halaman web).

## 4. `.env` production

Salin `.env.production.example` ke `/opt/apps/<slug>/.env` lalu isi. Variabel wajib:

| Variabel | Isi |
|---|---|
| `APP_SLUG` | sama dengan nama repo/folder |
| `APP_ENV` | `production` (mengaktifkan scheduler dan cookie `Secure`) |
| `PUBLISH_PORT` | port host untuk web-app |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | dipakai container `db` **dan** `deploy.sh` |
| `DATABASE_URL` | `postgres://USER:PASSWORD@db:5432/DB?sslmode=disable` |
| `ESB_API_BASE_URL` / `ESB_API_KEY` | kredensial ESB **production** (bukan staging) |

Opsional: `SYNC_ALERT_WEBHOOK_URL` (Slack/Discord, dipanggil bila ada hari yang gagal sinkron),
`HEALTHCHECKS_PING_URL`, `SESSION_TTL_HOURS` (default 12), `SYNC_HOUR_WIB` (default 6),
`SYNC_CATCHUP_DAYS` (default 7), `COOKIE_SECURE`.

Dua jebakan:
1. **`deploy.sh` men-`source` file ini.** Nilai yang mengandung spasi, `<`, `>`, `&`, `#`, `$`
   atau `;` **harus diberi tanda kutip** (URL webhook hampir selalu perlu). Baris `OIDC_*` dari
   `.env.example` jangan disalin apa adanya — belum dipakai.
2. Password database di `DATABASE_URL` harus aman untuk URL. Pakai karakter alfanumerik
   (mis. `openssl rand -hex 24`) supaya tidak perlu di-encode.

## 5. Deploy pertama

1. Repo sudah di org, secrets sudah diisi, server sudah siap (§2).
2. Push ke `main` (atau *Actions → Deploy production → Run workflow*). Alurnya:
   `make check-ci` → build image `api` + `web-app` (amd64, tag = SHA commit) → push ke GHCR →
   SSH → `deploy.sh`.
3. `deploy.sh`: pull image → cek `migrate has-pending` → (ada migration) backup + `migrate up` →
   `up -d` → cek health (maks. ~60 detik).
4. Di deploy pertama database masih kosong, jadi seluruh migration berjalan dan backup-nya
   kosong. Itu normal.
5. **Buat admin pertama** (tidak ada UI untuk ini, sengaja):
   ```
   cd /opt/apps/<slug>
   docker compose -f docker-compose.prod.yml exec api /app/server user set-password --email=EMAIL --admin
   ```
   Password diminta dua kali (tersembunyi). Untuk non-interaktif: `... exec -T api ...` dan
   kirim password lewat stdin (`printf '%s\n' "$PW" | ...`) — **jangan** lewat argumen.
6. Login di `https://<domain>`, tambah user lain di **Kelola User**.
7. **Isi data historis.** Scheduler mengejar 7 hari terakhir saat start. Untuk lebih lama:
   menu **Sinkron Data** (maks. 62 hari per sinkron), atau lewat server:
   `docker compose -f docker-compose.prod.yml exec api /app/server sync run --from=2026-09-01 --to=2026-09-29`.

### Verifikasi setelah deploy

```
docker compose -f docker-compose.prod.yml ps                    # 3 service Up
docker compose -f docker-compose.prod.yml logs api | tail -20   # cari "listening on :8080"
curl -fsS https://<domain>/healthz                               # ok
```

Baris log yang diharapkan dari scheduler: `sync-esb scheduler: next run at …`. Bila tertulis
`sync scheduler disabled` atau `NOT started: ESB_API_BASE_URL / ESB_API_KEY not set`, cek `.env`.

## 6. Operasional

| Tugas | Perintah (di `/opt/apps/<slug>`) |
|---|---|
| Deploy versi tertentu | `IMAGE_TAG=<sha> ./deploy.sh` |
| **Rollback aplikasi** | `IMAGE_TAG=<sha-lama> SKIP_MIGRATE=1 ./deploy.sh` |
| Lihat log | `docker compose -f docker-compose.prod.yml logs -f api` |
| Reset password user | `... exec api /app/server user set-password --email=EMAIL` (atau lewat Kelola User) |
| Sinkron manual | menu **Sinkron Data**, atau `... exec api /app/server sync run --date=YYYY-MM-DD` |

Sinkron manual punya dua mode: *Lengkapi* (hanya menambah yang belum ada) dan *Perbarui*
(menimpa data tersimpan — dipakai bila transaksi di-void/diubah setelah tersinkron).
Sinkron otomatis harian **tidak** menarik ulang tanggal lampau.

## 7. Backup & restore

- `deploy.sh` membuat `backups/<slug>-pre-migration.sql.gz` **hanya bila ada migration baru**,
  dan selalu menimpa file sebelumnya. Ini hanya melindungi dari migration terakhir.
- **Backup rutin volume `pgdata` (harian) adalah tanggung jawab DevOps.** Contoh:
  `docker compose -f docker-compose.prod.yml exec -T db pg_dump -U $POSTGRES_USER $POSTGRES_DB | gzip > /path/backup-$(date +%F).sql.gz`
- Restore (menimpa data production — **konfirmasi dulu**):
  `gunzip -c backups/<slug>-pre-migration.sql.gz | docker compose -f docker-compose.prod.yml exec -T db psql -U $POSTGRES_USER $POSTGRES_DB`

## 8. Batasan yang diketahui

- **Satu instance `api`**: scheduler dan pembatas login berjalan in-memory per proses
  (scheduler memakai advisory lock sehingga dua instance tidak menyinkron bersamaan, tetapi
  batas percobaan login tidak dibagi antar instance).
- **Login lokal bukan SSO.** Standar tim adalah operations-sso; belum dikerjakan. Sesi disimpan
  di tabel `sessions` sehingga SSO nanti bisa memakai lapisan yang sama.
- Belum ada "lupa password" mandiri (reset lewat admin) dan belum ada 2FA.
- Data ESB yang tersinkron hanya sebatas yang dikembalikan API ESB; membership (`raw_members`)
  belum disinkronkan oleh ETL.

## 9. Troubleshooting

| Gejala | Kemungkinan penyebab |
|---|---|
| Login berhasil tetapi langsung kembali ke halaman login | `COOKIE_SECURE=true` tetapi situs diakses lewat HTTP; atau proxy tidak meneruskan cookie |
| Step *Deploy via SSH* gagal | Secrets belum diisi / key salah / `docker login ghcr.io` belum dilakukan di server |
| Build image gagal: `invalid reference format` | Nama repo mengandung huruf besar/underscore — pakai nama huruf kecil |
| `deploy.sh`: health check GAGAL | `docker compose ... logs api`: biasanya `DATABASE_URL` salah atau migration gagal |
| `deploy.sh` error saat `source .env` | Ada nilai di `.env` yang mengandung karakter khusus tanpa tanda kutip |
| Sinkron gagal terus | `ESB_API_KEY` salah/kadaluarsa; lihat menu **Sinkron Data → Riwayat** dan log `api` |
| Semua user kena "terlalu banyak percobaan gagal" | `X-Forwarded-For` tidak diteruskan sehingga semua tampak satu IP (batas per-IP 30/15 menit) |
