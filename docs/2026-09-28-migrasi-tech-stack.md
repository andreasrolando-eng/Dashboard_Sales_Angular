# Migrasi tech stack: Next.js/Supabase → Angular + Go + PostgreSQL

## Latar belakang

`dashboard-sales` (repo lama: `andreasrolando-eng/Dashboard-Sales`, Next.js 16 + React 19 +
Supabase, hosting Vercel) adalah dashboard analytics harian untuk management/eksekutif ESB
(Overview, Sales, Ops, Membership, Marketing) plus MCP server (Node/TS) untuk 10 tools
analytics read-only.

Repo ini (`dashboard-sales-v2`) adalah rewrite penuh mengikuti standar tim Operations ESB
(skill `esb-ops-dev-stack`, pemilik standar: Eka): Angular+PrimeNG (Sakai-ng) di frontend,
Go+GORM+goose di backend, PostgreSQL self-hosted, operations-sso untuk auth.

## Keputusan (2026-09-28)

1. **Auth**: operations-sso (OIDC), bukan Google OAuth langsung. Catatan: dokumentasi
   project lama (PROGRESS.md/RUNBOOK.md) menyebut rencana multi-tenant SaaS untuk customer
   di luar ESB di masa depan — operations-sso kemungkinan hanya untuk identitas internal ESB,
   sehingga rencana multi-tenant tersebut perlu didesain ulang terpisah (perlu approval Eka)
   bila memang akan dieksekusi. Tidak menghalangi migrasi ini.
2. **MCP server**: di-port penuh ke Go sebagai bagian `./api` (satu bahasa backend, satu
   pipeline deploy), menggantikan `mcp-server/` (Node/TS) di repo lama.
3. **Database**: full lepas dari Supabase — Auth, Edge Functions, dan pg_cron semua diganti
   komponen Go/Postgres self-hosted. Data produksi perlu dipindah (pg_dump/restore) saat
   cutover; ETL harian (`sync-esb`) diimplementasi ulang sebagai job Go.
4. **Strategi eksekusi**: repo terpisah dari Next.js lama. Next.js lama tetap jalan apa
   adanya di repo lamanya sampai versi baru ini siap cutover — tidak ada perubahan ke
   repo/codebase lama sebagai bagian dari migrasi ini.

## Roadmap milestone

- **M0** (selesai): scaffold struktur `./api`, `./web-app`, `./docs`, tooling
  (Makefile, docker-compose, CI skeleton, .env.example) — tanpa business logic.
- **M1**: backend core — config, koneksi GORM, migration goose pertama (tabel inti, users/roles),
  auth operations-sso, `GET /api/me`.
- **M2**: endpoint API untuk tiap tab dashboard (sales, ops, membership, marketing, meta) +
  admin user management, menggantikan query Supabase langsung dari browser.
- **M3**: ETL — port `sync-esb` (Supabase Edge Function + pg_cron) jadi job Go terjadwal.
- **M4**: 10 MCP tools di-port ke Go (`internal/mcp`), reuse service layer yang sama dengan
  dashboard.
- **M5**: UI Angular — 5 tab dashboard + admin, PrimeNG Chart menggantikan Recharts, export
  PDF/Excel (jspdf/exceljs, tetap dipakai karena generik, bukan spesifik Next.js).
- **M6**: deployment — GitHub Actions/GHCR/docker compose ke server ESB, rencana cutover data
  produksi, retirement Next.js lama di Vercel.

Peta detail fitur lama → struktur baru ada di plan migrasi (diarsipkan bersama riwayat
percakapan; ringkasan tabelnya juga relevan sebagai referensi saat mengerjakan M1–M5).
