# Deploy demo ke Railway

Untuk **demo/UAT** saja. Production tetap lewat DevOps (`docs/deploy-production.md`).
Pakai **kredensial ESB staging**, jangan production.

## Cara kerjanya

Satu service Railway menjalankan satu container dari `Dockerfile` di root repo:
- build Angular + Go dalam satu image (±75 MB),
- saat start: `server migrate up` lalu `server serve`,
- API Go juga menyajikan aplikasi Angular (`STATIC_DIR=/app/public`), jadi web dan API
  **satu domain** — cookie login bekerja tanpa pengaturan CORS,
- mendengarkan `$PORT` dari Railway (fallback `APP_PORT`, lalu 8080).

Konfigurasi build/deploy ada di `railway.json` (builder Dockerfile, health check `/healthz`).
File `api/Dockerfile`, `web-app/Dockerfile` dan `docker-compose*.yml` tidak berubah.

## Langkah

1. **Push kode ke GitHub** (repo pribadi boleh).
2. Daftar di railway.com **lewat GitHub**. Pastikan status akun *Trial*, bukan *Limited Trial*
   (Limited membatasi koneksi keluar → sinkron ke ESB bisa gagal).
3. **New Project → Deploy from GitHub repo** → pilih repo. Railway membaca `railway.json`.
4. **Database** — pilih salah satu:
   - Railway: di project yang sama **+ New → Database → PostgreSQL**. Di service aplikasi
     tambahkan variabel `DATABASE_URL` = `${{Postgres.DATABASE_URL}}` (referensi; nama service
     database menyesuaikan).
   - Neon (gratis permanen 0,5 GB): buat project, salin connection string, isi `DATABASE_URL`
     (sudah berisi `sslmode=require`).
5. **Variabel service aplikasi** (Variables):

   | Variabel | Nilai |
   |---|---|
   | `DATABASE_URL` | lihat langkah 4 |
   | `ESB_API_BASE_URL` | URL ESB **staging** |
   | `ESB_API_KEY` | key ESB **staging** |
   | `SYNC_ALERT_WEBHOOK_URL` | opsional |

   `APP_ENV=production` dan `STATIC_DIR` sudah di-set di image. Jangan set `COOKIE_SECURE=false`
   — domain Railway sudah HTTPS.
6. **Settings → Networking → Generate Domain** → dapat `https://<nama>.up.railway.app`.
7. Tunggu deploy hijau. Di log harus ada:
   `goose: successfully migrated database …` lalu `listening on :<port>` dan
   `sync-esb scheduler: next run at …`.
8. **Buat password admin** (lihat bagian berikut), buka domain, login.
9. Isi data: menu **Sinkron Data** (maks. 62 hari per sinkron). Scheduler juga mengejar 7 hari
   terakhir saat start.

## Password admin pertama

**Cara termudah (tanpa laptop):** tambahkan dua variabel di service aplikasi, lalu tunggu deploy ulang:

| Variabel | Nilai |
|---|---|
| `BOOTSTRAP_ADMIN_EMAIL` | `andreas.rolando@esb.co.id` |
| `BOOTSTRAP_ADMIN_PASSWORD` | password pilihan (min. 8 karakter) |

Saat start, aplikasi memberi password itu ke akun tersebut (dibuat sebagai admin bila belum ada)
**hanya jika akun itu belum punya password** — tidak pernah menimpa password yang sudah ada.
Log menampilkan `bootstrap admin: … siap login sebagai admin` (atau alasan gagal, mis. password
kurang dari 8 karakter). Setelah berhasil login, **hapus `BOOTSTRAP_ADMIN_PASSWORD`** dan ganti
password lewat menu **Akun**. User lain ditambahkan dari **Kelola User**.

**Alternatif (dari laptop):**

Migration pertama otomatis membuat user `andreas.rolando@esb.co.id` (admin) **tanpa
password**, jadi belum bisa login. Atur password dari laptop dengan menunjuk ke database online:

```bash
cd api
# Database Railway: Postgres → Settings → Networking → aktifkan TCP Proxy,
# lalu salin DATABASE_PUBLIC_URL (matikan lagi TCP Proxy setelah selesai — egress ditagih).
DATABASE_URL="<url database publik>" go run ./cmd/server user set-password --email=andreas.rolando@esb.co.id --admin
```

Password diminta dua kali (tersembunyi). User lain ditambahkan dari **Kelola User**.

## Biaya (per 2026-09)

- Trial: kredit **$5 sekali**, maks. 30 hari. Setelah itu paket Free **$1/bulan** atau Hobby $5/bulan.
- Volume database yang dibuat saat trial **dihapus 30 hari setelah kredit habis** bila tidak upgrade.
- Perkiraan kasar: API saja ±$0,7/bulan; API + Postgres Railway ±$3–4/bulan. Pantau di *Usage*.

## Catatan

- **Satu replica saja** (scheduler dan pembatas login berjalan di memori proses).
- Push ke `main` juga menjalankan workflow GitHub Actions `Deploy production`; di repo pribadi
  langkah build/push ke GHCR org akan gagal. Itu tidak mempengaruhi Railway. Bisa dimatikan di
  tab *Actions → Deploy production → Disable workflow*.
- Tes lokal image yang sama:
  ```bash
  docker build -t ds-demo .
  docker run --rm -p 9000:7777 -e PORT=7777 -e COOKIE_SECURE=false \
    -e DATABASE_URL="postgres://…" ds-demo
  ```
  (`COOKIE_SECURE=false` hanya karena `http://localhost`.)
