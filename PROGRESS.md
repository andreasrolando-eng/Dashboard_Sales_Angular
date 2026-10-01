# Progress — dashboard-sales-v2

Rewrite dashboard-sales dari Next.js/Supabase ke Angular+Go+PostgreSQL (standar tim Operations ESB, skill `esb-ops-dev-stack`). Repo lama (Next.js) di `D:\KANTOR\dashboard sales` — **masih production, tidak disentuh sama sekali** oleh rewrite ini.

Update terakhir: 2026-10-01 (sejak 09-29: login lokal, deploy demo Railway, sinkron manual satu mode, segmen Non Sales — detail di bagian bertanggal paling bawah; M4 tetap ditunda sampai milestone lain selesai — source MCP lama tidak ada di mesin ini).

## Cara lanjut di sesi baru — baca ini dulu

1. **Postgres dev** kemungkinan masih jalan (container `dashboard-sales-db`, port **5555** karena port 5432 host dipakai Postgres lain buat Odoo). Cek: `docker ps --filter name=dashboard-sales-db`. Kalau mati: `docker start dashboard-sales-db`.
2. **Backend Go** biasanya perlu di-start ulang tiap sesi baru (proses tidak bertahan lintas sesi):
   ```bash
   cd D:/KANTOR/dashboard-sales-v2/api
   go run ./cmd/server serve
   ```
   Jalankan dari dalam folder `api/` (bukan root) — `.env` di root baru kebaca lewat fallback `../.env`.
3. **Frontend Angular**: `cd D:/KANTOR/dashboard-sales-v2/web-app && npm start` → `http://localhost:4200`.
4. `.env` di root sudah terisi (`DATABASE_URL` ke port 5555, `ESB_API_KEY`/`ESB_API_BASE_URL` buat ETL sungguhan). **Jangan commit `.env`** (sudah di `.gitignore`).
5. Database dev sekarang isinya **data ESB asli** (bukan dummy) — hasil `server sync run` dari staging. Kalau butuh data dummy lagi buat testing volume: `go run ./cmd/server seed` (truncate + generate 45 hari data sample, JANGAN dipakai bareng data asli tanpa sadar — selalu truncate dulu).

## Status milestone

| Milestone | Status | Catatan |
|---|---|---|
| **M0** — Scaffold Angular+Go+Postgres | ✅ Selesai | Awalnya PrimeNG/Sakai-ng, **di-migrasi ulang ke Taiga UI** 2026-09-29 (lihat di bawah) |
| **M1** — Skema DB inti (users + raw tables sales/ops/membership) | ✅ Selesai | 3 migration goose, 7 tabel, GORM models |
| **M2a-e** — Endpoint API (meta, admin, sales, ops, membership, marketing) | ✅ Selesai | 23 endpoint, port persis dari logic MCP+dashboard lama |
| **M3** — ETL (sync-esb → Go) | ✅ Selesai (core) | Live-tested ke API ESB staging sungguhan, berhasil |
| **Migrasi Taiga UI** | ✅ Selesai | PrimeNG dilarang (lisensi komersial v22+), ganti Taiga UI |
| **Login lokal (email + password)** | ✅ Selesai | Atas permintaan user: halaman `/login`, sesi server-side, seluruh `/api/*` diproteksi (lihat bawah) |
| **SSO (operations-sso)** | ⏸️ Ditunda | Standar tim; login lokal dibuat supaya sesinya bisa dipakai SSO nanti |
| **M3b** — Scheduling produksi ETL | ✅ Selesai | Scheduler in-process di `serve`: 06:00 WIB + catchup + advisory lock + webhook alert (lihat bawah) |
| **M4** — Port 10 tools MCP ke Go | ❌ Belum | Reuse service layer M2 |
| **M5** — Wiring UI (6 halaman) | ✅ Selesai | Overview, Sales, Ops, Membership, Marketing, Kelola User tersambung ke API asli (lihat bawah) |
| **M6** — Deploy production | 🟡 Siap serah-terima | Pipeline + panduan lengkap di `docs/deploy-production.md`, template `.env.production.example`. **Menunggu:** repo di org Operations-ESB (nama huruf kecil), server + secrets dari DevOps, kredensial ESB production |

## Yang sudah jadi (detail)

### M1 — Skema database
`api/migrations/`: `20260928170001_users.sql`, `20260928170002_raw_tables.sql`, `20260928170003_raw_tables_indexes.sql`. Tabel: `users`, `outlets`, `raw_sales`, `raw_sales_payments`, `raw_sales_menu_items` (PK `sales_num,line_seq`), `raw_members`, `sync_logs`. GORM models di `api/internal/model/`.

### M2 — API (`api/internal/service/` + `api/internal/handler/`)
- **meta**: `GET /api/meta/{outlets,categories,category-details,last-sync}`
- **admin**: `GET/POST /api/admin/users`, `DELETE /api/admin/users/{email}` — **catatan**: aturan "tidak bisa hapus akses sendiri" belum diaktifkan (butuh session asli dari SSO, lihat `TODO(SSO)` di `service/admin.go`)
- **sales**: `GET /api/sales/{summary,daily,hourly,revenue-by-outlet,revenue-by-category,top-products,menu-performance,bills,bills/export}`
- **ops**: `GET /api/ops/summary`
- **membership**: `GET /api/membership/{summary,top-members,member-options,new-weekly}`, `GET /api/membership/members/{memberCode}/menu-purchases`
- **marketing**: `GET /api/marketing/promo-performance`

Semua logic di-port **persis** dari SQL final project lama (bukan disederhanakan) — formula `nett_sales`, contribution_pct, trend ±10%, dwell cap 8 jam, retention/churn membership, lift/ROI/status promo, dst. Detail lengkap tiap quirk ada di comment masing-masing fungsi service.

### M3 — ETL (`api/internal/etl/`)
Port dari `supabase/functions/sync-esb/` (project lama). Subcommand: `server sync run [--date=YYYY-MM-DD | --from=... --to=...]` (default: kemarin WIB). **Sudah live-tested ke staging ESB sungguhan** — berhasil fetch+transform+upsert data asli.

Belum di-port dari versi lama: refresh materialized view (kita query live, tidak pakai matview).

### M3b — Scheduler (`api/internal/etl/schedule.go`)
Goroutine di dalam `server serve` (compose prod cuma punya satu service `api`). Aktif default hanya kalau `APP_ENV=production` (atau `SYNC_SCHEDULER_ENABLED=true`) dan kredensial ESB terisi. Saat start langsung catch-up, lalu tiap hari `SYNC_HOUR_WIB` (default 6) WIB. Yang disinkron = semua tanggal dalam `SYNC_CATCHUP_DAYS` (default 7) hari terakhir yang belum punya `sync_logs` sukses — jadi hari gagal otomatis di-retry malam berikutnya. Pakai `pg_try_advisory_xact_lock` supaya dua instance nggak sync bersamaan. Kalau ada hari gagal: alert ke `SYNC_ALERT_WEBHOOK_URL` (payload kompatibel Slack + Discord) + ping healthchecks. `server sync run` (CLI) juga kirim alert kalau ada hari gagal. Sudah di-smoke-test live 2026-10-01 (lihat bagian Refresh harian di bawah).

### M5 — UI (`web-app/src/app`)
- `core/`: `api.ts` (`apiResource` = wrapper `httpResource` reaktif, otomatis refetch saat filter berubah), `filters.ts` (state global tanggal + outlet, default 7 hari terakhir s/d kemarin WIB), `models.ts` (tipe respons API), `format.ts` (rupiah/persen/tanggal id-ID).
- `shared/`: `filter-bar`, `ui.ts` (Kpi, Panel dgn state loading/error/kosong, Columns, HBars — chart CSS murni, tanpa library).
- Tampilan (permintaan user: nuansa Sakai-ng, tetap Taiga): shell custom di `app.html` (topbar putih, sidebar terang, aksen emerald), semua warna dari variabel `--app-*` di `styles.css` (terang/gelap lewat `data-theme`, toggle di topbar, tersimpan oleh Taiga di localStorage `tuiDark`). Taiga dipakai untuk `TuiRoot`, ikon (`tui-icon`), font, dan state gelap/terang. Desktop: sidebar bisa collapse jadi icon-only; < 768px: drawer overlay. **Keputusan:** PrimeNG/Sakai-ng asli TIDAK dikembalikan (bentrok standar tim Taiga + lisensi PrimeNG v22 komersial); kalau mau, butuh approval Eka.
- Filter tanggal: `shared/date-range-picker.ts` (+ logika murni `core/dates.ts`, dites di `dates.spec.ts`) — tombol `dd-MM-yyyy → dd-MM-yyyy`, popover 2 bulan (1 bulan di ponsel), band biru + lingkaran awal/akhir, klik 2× untuk pilih rentang (urutan terbalik otomatis dibetulkan), preset Hari ini / 7 hari terakhir / 30 hari terakhir / Bulan ini (semua berakhir hari ini), Esc & klik luar menutup, tanggal setelah hari ini nonaktif. Default rentang halaman tetap 7 hari lengkap s/d kemarin (data hari ini baru masuk setelah ETL 06:00 besok).
- Dropdown: `shared/select.ts` (`app-select`) menggantikan `<select>` native (daftar native digambar OS, tidak bisa di-rounded) — popover rounded senada kalender, keyboard (panah/Home/End/Enter/Esc), tes di `select.spec.ts`. Dipakai di filter Outlet dan pilih member. Jangan bungkus komponen popover dengan `<label>` (klik non-interaktif diteruskan ke tombol pertama dan membuka ulang popover).
- Branding: logo dari `icon.png` (root repo, 256×256) disalin ke `web-app/public/icon.png`, dipakai sebagai favicon/apple-touch-icon (`index.html`) dan logo topbar (`app.html`); `favicon.ico` bawaan Angular dihapus.
- Detail polish: chart punya skala sumbu-y "rapi" (500 rb / 250 rb) + garis bantu, skeleton loading, angka desimal format id-ID (52,19×), badge "Stagnan" dibuat muted, route `**` redirect ke Overview, judul tab "Dashboard Sales ESB".
- Sengaja **belum**: filter kategori di Top produk / Performa menu (API-nya sudah mendukung), export PDF/Excel bills (`/sales/bills/export` sudah ada), chart library sungguhan (Taiga addon-charts), login/logout nyata (email di header masih hardcode, tombol Logout belum fungsi — menunggu SSO).
- Bug backend yang ketemu saat wiring: `DELETE /api/admin/users/{email}` diam-diam tidak menghapus karena `@` dikirim browser sebagai `%40` dan tidak di-decode (sudah diperbaiki + test); JSON `User` sebelumnya PascalCase, sekarang snake_case seperti endpoint lain.

### Sinkron manual (`etl/manual.go`, `handler/sync.go`, UI `/admin/sync`)
Tarik data ESB untuk rentang tanggal pilihan (maks 62 hari, sampai hari ini) dari halaman **Sinkron Data**. Berjalan di background (satu job sekali jalan), progres per tanggal tampil langsung.
- **Hanya melengkapi, tidak menimpa, tidak dobel:** mode `ModeFillMissing` = `INSERT … ON CONFLICT DO NOTHING` di semua tabel (outlet, bill, pembayaran, item menu), dan semua tabel punya primary key alami dari ESB. Bill yang sudah ada tidak diubah; kalau bill sudah ada tapi pembayaran/itemnya ada yang hilang, baris yang hilang itu diisi. Yang dilaporkan hanya baris yang benar-benar baru vs "sudah ada". Sync harian tetap `ModeUpsert` (menimpa) supaya status bill yang berubah ikut ter-refresh.
- Dicatat di `sync_logs` dengan `job_name = sync-esb-manual`: TIDAK dianggap "sudah disinkron" oleh catch-up scheduler dan TIDAK mengubah "Sinkron terakhir" di header.
- Satu advisory lock (`etl.WithSyncLock`) dipakai bersama scheduler malam: kalau sedang bentrok, sync manual gagal dengan pesan jelas (tidak menabrak).
- Endpoint: `POST /api/admin/sync {date_from,date_to}` → 202; `GET /api/admin/sync` (job terakhir, disimpan di memori — hilang saat server restart); `GET /api/admin/sync/logs?limit=`. 503 kalau kredensial ESB belum diisi.
- **Dua mode** (pilihan di UI, field `mode` di POST): `fill` = "Lengkapi" (default, di atas) dan `refresh` = "Perbarui data yang sudah ada" (`ModeUpsert`, menimpa bill yang tersimpan dengan data ESB saat ini). Refresh dibuat untuk kasus transaksi kemarin di-void hari ini: ESB mengembalikan bill itu di **tanggal transaksi aslinya** dengan `statusName: "Void"` (+ `editedDate` baru), jadi pilih tanggal transaksinya, bukan tanggal void. Semua query dashboard menghitung `status_name = 'Finished'`, jadi bill yang jadi Void langsung keluar dari revenue.
- Refresh melaporkan (dihitung SEBELUM menulis, `etl/changes.go`): bill baru vs diperbarui, **perubahan status per bill** (mis. Finished → Void, dengan no. bill/outlet/total), dan bill tersimpan yang tidak lagi dikembalikan ESB untuk tanggal itu (hanya dilaporkan, TIDAK diubah/dihapus — hilang dari jawaban ESB belum tentu void). Dicatat sebagai `job_name = sync-esb-manual-refresh` (tetap tidak dianggap sync harian).
- Diuji end-to-end ke ESB staging: bill `IET202609090004` (Void di ESB) dikembalikan ke Finished di DB dev, lalu Perbarui tanggal 9 Sep → terdeteksi Finished → Void, revenue 9 Sep turun Rp 2.530.000 → Rp 1.955.000 (−575.000), jumlah baris bill tetap 4.
- **Void susulan:** sejak 2026-10-01 scheduler juga menarik ulang `SYNC_REFRESH_DAYS` (default 3) hari terakhir yang sudah tersinkron (lihat bagian M3b). Void yang terjadi lebih lama dari itu tetap hanya masuk lewat Sinkron manual.
- **TODO(SSO):** endpoint ini (seperti /api/admin/*) belum dibatasi admin karena belum ada sesi.

### Login lokal (`api/internal/auth`, `web-app/src/app/core/auth*.ts`, `features/login`)
Email + password, dibuat atas permintaan user karena SSO ditunda. **Catatan:** standar tim (skill esb-ops-dev-stack) mewajibkan operations-sso; ini bukan penggantinya. Lapisan sesinya (cookie → tabel `sessions` → user) sengaja terpisah dari cara sesi dibuat, jadi SSO nanti cukup membuat baris `sessions` yang sama.
- **Sesi di database** (migration `20260930100001_auth`): cookie `ds_session` (HttpOnly, SameSite=Lax, Secure di production) berisi token acak 256-bit; yang disimpan hanya SHA-256-nya. Sesi 12 jam (`SESSION_TTL_HOURS`). Menghapus user atau mengganti/mereset password mencabut sesinya.
- **Password:** bcrypt cost 12, minimal 8 karakter, maksimal 72 byte (batas bcrypt). Kegagalan login selalu berpesan sama ("Email atau password salah") dan memakan waktu bcrypt yang sama, jadi tidak membocorkan email mana yang terdaftar.
- **Anti brute-force** (`auth/limiter.go`, in-memory per proses): 5 gagal / 15 menit per email dan 30 per IP → 429 + `Retry-After`, berlaku juga untuk password yang benar. IP diambil dari `X-Forwarded-For` (nginx meneruskannya); pembatas per-email adalah perlindungan utama.
- **Akses:** hanya `/healthz` dan `POST /api/auth/login` yang publik; sisanya butuh login, `/api/admin/*` (Kelola User, Sinkron Data) hanya admin. Ada test yang menelusuri SEMUA route dan gagal kalau ada yang lolos tanpa login (`cmd/server/router_test.go`). Semua respons `/api` ber-`Cache-Control: no-store`.
- **Endpoint:** `POST /api/auth/login|logout|password`, `GET /api/me`, `POST /api/admin/users/{email}/password`. `POST /api/admin/users` menerima `password` opsional. Admin tidak bisa menghapus akunnya sendiri (aturan lama fn_admin_remove_user, akhirnya bisa dipasang karena sudah ada sesi).
- **UI:** `/login` (tanpa sidebar), `/account` (ganti password sendiri), guard `authGuard/adminGuard/guestGuard`, interceptor 401 → kembali ke login dengan pemberitahuan "sesi berakhir", `returnUrl` hanya path internal (anti open-redirect), menu admin disembunyikan untuk non-admin. Layout dipisah: `App` (tema) → `layout/Shell` (topbar+sidebar) → halaman.
- **Password admin pertama** (tidak ada UI untuk itu, sengaja): `server user set-password --email=EMAIL --admin` (prompt tersembunyi, atau dikirim lewat stdin; TIDAK lewat flag supaya tidak masuk shell history). Di server: `docker compose exec api /app/server user set-password --email=EMAIL --admin`. Membuat user kalau belum ada.
- **Gotcha produksi:** `COOKIE_SECURE` default true di production → kalau app dilayani lewat HTTP polos, browser membuang cookie dan login berulang. Set `COOKIE_SECURE=false` hanya bila memang tanpa TLS.
- Belum ada: "lupa password" mandiri (reset lewat admin), 2FA, dan pembatas yang dibagi antar-instance (in-memory).

### Testing
- **Go**: 70+ test (`model`+`service`+`etl`), semua pakai database ephemeral per-test (`api/internal/testutil/db.go`) — jalan konkuren tanpa race. `TEST_DATABASE_URL` harus di-set buat test yang butuh DB asli, kalau tidak di-set otomatis skip.
- **Angular**: 54 test (Vitest, headless via jsdom default) — app shell + formatter.
- `go test ./...` dan `npm test` sama-sama harus lulus sebelum push (`make check` belum pernah dijalankan penuh di mesin ini karena beberapa sub-step butuh Docker yang kadang lambat — tapi semua komponennya sudah diverifikasi manual satu-satu).

### Bug nyata yang ketemu & diperbaiki selama development (referensi kalau ada yang mirip muncul lagi)
1. Race antar-package saat test (setiap test sekarang pakai database ephemeral sendiri).
2. Parameter tanggal terbalik di CTE `period_days` (`menu_performance`) — previous-period jadi range invalid.
3. GORM named-parameter (`@nama`) tidak jalan di kombinasi driver ini — semua query pakai positional `?`.
4. `flexNumber` (tipe custom buat `paxTotal` ESB) tidak punya `MarshalJSON` — merusak kolom `raw` jsonb kalau di-marshal ulang.
5. `Outlet.FirstSeenAt`/`LastSeenAt` ke-insert sebagai zero-value Go (`0001-01-01`) alih-alih `default now()` — GORM tetap kirim field kosong secara eksplisit kalau tidak di-`Omit()`.
6. Migrasi Taiga UI: import `TuiAppBarComponent` salah dari `@taiga-ui/core` (harusnya `@taiga-ui/layout`); atribut `tuiNavigationAside` bentrok dengan property binding di elemen yang sama.

## Keputusan arsitektur yang sudah dikunci (jangan diubah tanpa alasan kuat)

- **Auth**: operations-sso (OIDC) begitu dikerjakan nanti — bukan Google OAuth langsung seperti versi lama. Ditunda dulu atas keputusan user.
- **MCP server**: akan di-port penuh ke Go (M4), jadi bagian `./api`, bukan Node terpisah.
- **Database**: full lepas dari Supabase, Postgres self-hosted, ETL sendiri (bukan Supabase Edge Function).
- **Agregasi**: semua logic (nett_sales, dst) di service layer Go pakai raw SQL/GORM — **bukan** Postgres view/materialized view seperti versi lama (kecuali kalau nanti ada alasan performa kuat buat matview).
- **Frontend**: Taiga UI (bukan PrimeNG — dilarang sejak lisensi v22 komersial). **`esb-angular-starter` belum ada di org** — layout `web-app/src/app/app.html` saat ini interpretasi manual dari spesifikasi skill, **perlu diselaraskan ulang begitu starter resmi ada** (minta Eka buatkan).

## Yang perlu diminta ke Eka / belum beres

- [ ] Repo `dashboard-sales-v2` belum ada di org **Operations-ESB** — sementara di-push ke repo pribadi `andreasrolando-eng/Dashboard_Sales_Angular` (remote `origin`).
- [ ] Starter resmi **esb-angular-starter** belum ada — layout `web-app` perlu diselaraskan begitu ada.
- [ ] Client `operations-sso` production (kalau nanti SSO dikerjakan).
- [ ] Kredensial `ESB_API_BASE_URL`/`ESB_API_KEY` production (yang di `.env` sekarang kemungkinan staging).

## Next step yang disarankan

M4 ditunda (keputusan user: dikerjakan terakhir; butuh source `mcp-server/` dari repo lama `andreasrolando-eng/Dashboard-Sales`). Sisa: M6 (deploy — butuh repo di org Operations-ESB dulu), lalu M4. SSO juga masih ditunda.

## Refresh harian + uji live scheduler (2026-10-01)
- Scheduler (`etl.RefreshDates`, `Scheduler.RunOnce`) kini setelah catch-up juga menarik ulang `SYNC_REFRESH_DAYS` (default 3; 0 = mati) hari terakhir s/d kemarin yang tidak baru saja diambil, mode upsert + laporan perubahan status (dicatat ke log server). Job name `sync-esb-refresh` — tidak dihitung catch-up / "Sinkron terakhir"; di riwayat Sinkron Data tampil "Harian (perbarui)". Jalan juga saat server start (3 fetch tambahan per restart — kecil).
- **Uji live ke ESB staging (DB dev, port 8099):** run pertama catch-up 29–30 Sep + refresh 28 Sep; restart → hanya refresh 28–30 Sep, tidak ada tanggal terlewat/dobel, "next run at 2026-10-02T06:00+07:00". M3b sekarang sudah di-smoke-test live.

## Deploy demo Railway (2026-09-30)
`Dockerfile` + `.dockerignore` + `railway.json` di root: satu container (Angular disajikan oleh API Go lewat `STATIC_DIR`, `handler.SPA`), migration otomatis saat start, membaca `$PORT`. Pool koneksi DB diatur untuk Postgres terkelola (idle 2 menit). Diuji lokal dengan Postgres kosong + `PORT=7777`: migration, SPA/deep link, cache bundle, 401 tanpa login, login+cookie, restart idempoten, dan alur browser. Panduan: `docs/deploy-railway.md`. Production tetap `docs/deploy-production.md`.

## Sinkron manual jadi satu mode (2026-09-30)
Permintaan user: UI **Sinkron Data** tidak lagi memilih "lengkapi/perbarui". Satu tombol = mode `refresh` (upsert): bill yang belum ada ditambahkan, yang sudah ada ditimpa data ESB terbaru, perubahan status dilaporkan, tanpa duplikat. `POST /api/admin/sync` tanpa `mode` kini default `refresh`; `mode: "fill"` (insert-only) masih diterima API untuk pemanggil lain. Riwayat menampilkan keduanya sebagai "Manual".

## Segmen Non Sales (2026-09-30)
Definisi (keputusan user): bill = **non sales** bila dibayar dengan `paymentMethodTypeID = 7`. ESB tidak mengizinkan tipe 7 digabung tipe lain, jadi seluruh bill non sales.
- Kolom `raw_sales.is_non_sales` (migration `20260930150001_non_sales`, mengisi data lama dari `raw_sales_payments`), diisi ETL di `etl.Transform` (`etl.NonSalesPaymentTypeID`). Sinkron manual/harian (upsert) ikut memperbaruinya.
- **Dikeluarkan dari semua angka sales:** Overview, Sales (summary, harian, per jam, per outlet, kategori, top produk, performa menu, bill, export), Ops (termasuk hitungan batal/void dan metode bayar), Membership (kunjungan, spending, riwayat menu), Marketing (redemption + baseline). Daftar kategori menu (meta) tidak diubah.
- **Menu baru Non Sales** (`/non-sales`, semua user login): KPI nilai/transaksi/rata-rata/outlet, harian, per outlet, menu terbanyak, daftar bill + metode bayar. API `GET /api/non-sales/{summary,daily,by-outlet,top-menus,bills}`.
- Data staging belum punya pembayaran tipe 7; diuji dengan simulasi satu bill di DB dev (revenue sales turun tepat sebesar bill itu, bill muncul di Non Sales), lalu dikembalikan.
