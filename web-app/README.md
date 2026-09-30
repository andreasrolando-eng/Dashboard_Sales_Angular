# Dashboard Sales ESB — web-app

Frontend Angular + Taiga UI untuk dashboard-sales-v2. Scaffold manual (`ng new` + `ng add taiga-ui`) karena starter resmi `esb-angular-starter` belum tersedia di org Operations-ESB — **request pembuatannya ke Eka**, lalu selaraskan layout ini begitu starter itu ada. Lihat `../docs/2026-09-29-migrasi-taiga-ui.md` untuk konteks migrasi dari PrimeNG/Sakai-ng.

## Development server

```bash
npm start
```

Buka `http://localhost:4200/`. `/api` dan `/auth` di-proxy ke backend Go di `localhost:8080` (lihat `proxy.conf.json`).

## Build

```bash
npm run build
```

Output di `dist/web-app/browser`.

## Test

```bash
npm test
```

Vitest, headless secara default (jsdom) — dipakai `make check` / CI. `src/test-setup.ts` mem-polyfill `window.matchMedia` yang tidak ada di jsdom tapi dibutuhkan Taiga UI untuk deteksi dark mode.

## Catatan Taiga UI

- Layout shell (`src/app/app.html`) adalah interpretasi terbaik dari spesifikasi layout di skill esb-ops-dev-stack (app-bar atas + sidebar `aside[tuiNavigationAside]` collapsible + `main[tuiNavigationMain]`), **bukan** disalin dari starter resmi karena belum ada. Perlu diselaraskan ulang begitu `esb-angular-starter` tersedia.
- Semua paket `@taiga-ui/*` versi `5.26.0`, kecuali `@taiga-ui/event-plugins` (5.1.0) dan `@taiga-ui/polymorpheus` (5.0.1) yang punya skema versi sendiri di luar release train utama.
- Tailwind hanya untuk layout/spacing, preflight mati (`src/styles.css`). Warna dari token `--tui-*`, jangan pakai kelas warna Tailwind.
