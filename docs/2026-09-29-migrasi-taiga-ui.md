# Migrasi frontend: PrimeNG/Sakai-ng → Taiga UI

## Latar belakang

`web-app` awalnya di-scaffold di M0 (2026-09-28) pakai Angular 21 + PrimeNG + template Sakai-ng, sesuai skill `esb-ops-dev-stack` yang berlaku saat itu.

Pada 2026-09-29, standar tim berubah: PrimeNG sejak v22 (Juni 2026) berlisensi komersial per developer dan ESB tidak memenuhi syarat lisensi komunitas. Skill diperbarui — stack Angular resmi sekarang **Taiga UI** (Apache-2.0) dari starter **esb-angular-starter**.

## Keputusan

Migrasi langsung (bukan tunggu approval Eka dulu) atas instruksi user, karena UI (M5) belum dikerjakan — belum ada sunk cost besar yang hilang. `web-app` versi PrimeNG di-backup ke `web-app-old-primeng-backup/` sebelum discaffold ulang, lalu dihapus setelah migrasi terverifikasi jalan (build+test lulus).

**Starter `esb-angular-starter` belum ada di org Operations-ESB** — scaffold dilakukan manual (`ng new` + `ng add taiga-ui`), mengikuti spesifikasi layout di skill (app-bar atas, sidebar kolaps, dst) sebagai interpretasi terbaik. **User perlu request pembuatan starter resmi ke Eka**, lalu `web-app` di sini perlu diselaraskan ulang begitu starter itu ada — layout saat ini bukan hasil salin dari starter resmi.

## Yang berubah

- Angular 21 → **22** (mayor terbaru saat migrasi).
- PrimeNG + `@primeuix/themes` + Sakai-ng → **Taiga UI 5.26.0** (`@taiga-ui/core`, `cdk`, `kit`, `icons`, `layout`, `addon-table`; `event-plugins`@5.1.0 dan `polymorpheus`@5.0.1 karena versi keduanya di luar release train utama).
- Karma/Jasmine → **Vitest** (default baru Angular 22, headless via jsdom secara default — tidak perlu setup ChromeHeadless manual seperti sebelumnya).
- Tailwind tetap dipakai, tapi **preflight dimatikan** dan kelas warna Tailwind tidak dipakai lagi — warna dari token `--tui-*`.
- 6 route placeholder (Overview/Sales/Ops/Membership/Marketing/Kelola User) dan `proxy.conf.json`/`nginx.conf`/`Dockerfile` dipertahankan konsepnya (framework-agnostic), cuma path dist yang disesuaikan (`dist/web-app/browser`, sebelumnya `dist/dashboard-sales-web-app/browser`).

## Yang belum diketahui / perlu diverifikasi begitu starter resmi ada

- Markup layout yang benar-benar dipakai `esb-angular-starter` (app-bar+aside yang saya susun adalah interpretasi dari deskripsi skill, bukan disalin dari kode nyata).
- Konvensi ikon (`@taiga-ui/icons`) — belum dipakai sama sekali di layout saat ini karena API input ikon Taiga tidak sempat diverifikasi terhadap dokumentasi resmi (offline).
