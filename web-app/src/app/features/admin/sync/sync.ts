import { HttpClient, httpResource } from '@angular/common/http';
import { Component, DestroyRef, computed, effect, inject, signal } from '@angular/core';
import { addDays } from '../../../core/dates';
import { wibDate } from '../../../core/filters';
import { longDate, num, rupiah, wibTime } from '../../../core/format';
import { BillRef, ManualDay, ManualJob, StatusChange, SyncLog, SyncMode, SyncStatus } from '../../../core/models';
import { DateRangePicker } from '../../../shared/date-range-picker';
import { Kpi, Panel } from '../../../shared/ui';

/** Mirrors etl.MaxManualDays on the server. */
const MAX_DAYS = 62;
/** Ask for confirmation above this many days (each day is at least one ESB request). */
const CONFIRM_ABOVE_DAYS = 31;

const STATUS_LABEL: Record<SyncStatus, string> = { pending: 'Menunggu', running: 'Berjalan', done: 'Selesai', failed: 'Gagal' };
const STATUS_BADGE: Record<SyncStatus, string> = { pending: '', running: 'badge-info', done: 'badge-pos', failed: 'badge-neg' };

const LOG_KIND: Record<string, string> = {
    'sync-esb': 'Harian',
    'sync-esb-manual': 'Manual · lengkapi',
    'sync-esb-manual-refresh': 'Manual · perbarui',
};

interface DatedChange extends StatusChange { date: string }
interface DatedRef extends BillRef { date: string }

@Component({
    selector: 'app-admin-sync',
    imports: [DateRangePicker, Kpi, Panel],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Sinkron Data</h2>
                <p class="page-sub">Tarik data penjualan dari ESB secara manual untuk tanggal yang dipilih.</p>
            </div>

            <section class="card">
                <header class="card-head"><h3>Tarik data manual</h3></header>

                <fieldset class="mode-group" [disabled]="running()">
                    <legend>Mode sinkron</legend>
                    <label class="mode-opt" [class.selected]="mode() === 'fill'">
                        <input type="radio" name="sync-mode" value="fill" [checked]="mode() === 'fill'" (change)="mode.set('fill')" />
                        <span>
                            <strong>Lengkapi data yang belum ada</strong>
                            <small>Hanya menambahkan yang belum tersimpan. Data yang sudah ada tidak diubah.</small>
                        </span>
                    </label>
                    <label class="mode-opt" [class.selected]="mode() === 'refresh'">
                        <input type="radio" name="sync-mode" value="refresh" [checked]="mode() === 'refresh'" (change)="mode.set('refresh')" />
                        <span>
                            <strong>Perbarui data yang sudah ada</strong>
                            <small>Ganti dengan data terbaru dari ESB. Pakai ini jika ada transaksi yang di-void atau diubah setelah tersimpan.</small>
                        </span>
                    </label>
                </fieldset>

                <div class="form-row">
                    <div class="field">
                        <span>Tanggal transaksi</span>
                        <app-date-range [(start)]="start" [(end)]="end" />
                    </div>
                    <button type="button" class="btn" [disabled]="!canStart()" (click)="startSync()">
                        {{ running() ? 'Sedang berjalan…' : starting() ? 'Memulai…' : mode() === 'refresh' ? 'Mulai perbarui' : 'Mulai sinkron' }}
                    </button>
                </div>
                <ul class="hint-list">
                    @if (mode() === 'fill') {
                        <li>
                            <strong>Hanya melengkapi:</strong> bill, pembayaran, dan item yang sudah tersimpan tidak diubah dan tidak digandakan —
                            yang ditambahkan hanya yang belum ada.
                        </li>
                    } @else {
                        <li>
                            <strong>Memperbarui:</strong> bill yang sudah tersimpan ditimpa dengan data ESB saat ini (tetap tidak ada yang digandakan).
                            Pilih <em>tanggal transaksi</em> aslinya — bill yang di-void hari ini tetap berada di tanggal kejadiannya.
                        </li>
                        <li>Bill yang statusnya berubah (mis. Finished → Void) ditampilkan di bawah, dan langsung keluar dari perhitungan revenue.</li>
                    }
                    <li>{{ rangeHint() }}</li>
                    <li>Sinkron otomatis harian (06:00 WIB) tetap berjalan seperti biasa dan tidak terpengaruh.</li>
                </ul>
                @if (error(); as e) {
                    <p class="notice" role="alert">{{ e }}</p>
                }
            </section>

            @if (job.value(); as j) {
                <div class="kpi-grid">
                    <app-kpi label="Bill baru ditambahkan" icon="@tui.receipt" [value]="num(totals().sales)" [hint]="'dari ' + num(totals().found) + ' bill di ESB'" />
                    @if (j.mode === 'refresh') {
                        <app-kpi label="Bill diperbarui" icon="@tui.refresh-cw" [value]="num(totals().refreshed)" hint="data terbaru dari ESB" />
                        <app-kpi label="Status berubah" icon="@tui.ban" [value]="num(changes().length)" hint="mis. Finished → Void" />
                        <app-kpi label="Tidak ada di ESB" icon="@tui.circle-user-round" [value]="num(missing().length)" hint="tidak diubah" />
                    } @else {
                        <app-kpi label="Sudah ada (dilewati)" icon="@tui.check" [value]="num(totals().refreshed)" hint="tidak diubah" />
                        <app-kpi label="Pembayaran baru" icon="@tui.credit-card" [value]="num(totals().payments)" />
                        <app-kpi label="Item menu baru" icon="@tui.shopping-cart" [value]="num(totals().items)" />
                    }
                </div>

                <app-panel [title]="'Progres: ' + longDate(j.date_from) + (j.date_from === j.date_to ? '' : ' – ' + longDate(j.date_to))">
                    <div panel-actions>
                        <span class="badge" [class.badge-info]="j.status === 'running'" [class.badge-pos]="j.status === 'done'" [class.badge-neg]="j.status === 'failed'">
                            {{ jobLabel(j) }}
                        </span>
                    </div>
                    @if (j.error) {
                        <p class="notice" role="alert">{{ j.error }}</p>
                    }
                    <div class="tbl-wrap">
                        <table class="tbl">
                            <thead>
                                <tr>
                                    <th>Tanggal</th><th>Status</th><th class="r">Bill di ESB</th><th class="r">Bill baru</th>
                                    <th class="r">{{ j.mode === 'refresh' ? 'Diperbarui' : 'Sudah ada' }}</th>
                                    @if (j.mode === 'refresh') {
                                        <th class="r">Status berubah</th>
                                    }
                                    <th>Keterangan</th>
                                </tr>
                            </thead>
                            <tbody>
                                @for (d of j.days; track d.date) {
                                    <tr>
                                        <td>{{ longDate(d.date) }}</td>
                                        <td><span class="badge" [class]="'badge ' + badge(d.status)">{{ label(d.status) }}</span></td>
                                        <td class="r">{{ d.result ? num(d.result.found.sales) : '–' }}</td>
                                        <td class="r">{{ d.result?.ok ? num(d.result?.sales_new ?? 0) : '–' }}</td>
                                        <td class="r">{{ d.result?.ok ? num(d.result?.sales_refreshed ?? 0) : '–' }}</td>
                                        @if (j.mode === 'refresh') {
                                            <td class="r">{{ d.result?.ok ? num(d.result?.status_changes?.length ?? 0) : '–' }}</td>
                                        }
                                        <td class="muted-cell">{{ note(d, j.mode) }}</td>
                                    </tr>
                                }
                            </tbody>
                        </table>
                    </div>
                </app-panel>

                @if (changes().length) {
                    <app-panel title="Perubahan status">
                        <div class="tbl-wrap">
                            <table class="tbl">
                                <thead>
                                    <tr><th>Tanggal transaksi</th><th>No. bill</th><th>Outlet</th><th>Perubahan status</th><th class="r">Total</th></tr>
                                </thead>
                                <tbody>
                                    @for (c of changes(); track c.sales_num) {
                                        <tr>
                                            <td>{{ longDate(c.date) }}</td>
                                            <td>{{ c.bill_num || c.sales_num }}</td>
                                            <td>{{ c.branch_code }}</td>
                                            <td>
                                                <span class="badge" [class]="'badge ' + statusBadge(c.from)">{{ c.from || '–' }}</span>
                                                <span class="arrow" aria-label="menjadi">→</span>
                                                <span class="badge" [class]="'badge ' + statusBadge(c.to)">{{ c.to || '–' }}</span>
                                            </td>
                                            <td class="r">{{ rupiah(c.grand_total) }}</td>
                                        </tr>
                                    }
                                </tbody>
                            </table>
                        </div>
                    </app-panel>
                }

                @if (missing().length) {
                    <app-panel title="Tersimpan tapi tidak lagi dikembalikan ESB">
                        <p class="page-sub">
                            Bill berikut ada di database untuk tanggal ini tetapi tidak muncul di jawaban ESB. Bill ini <strong>tidak diubah</strong> —
                            periksa di ESB apakah dihapus atau dipindah tanggal.
                        </p>
                        <div class="tbl-wrap">
                            <table class="tbl">
                                <thead><tr><th>Tanggal transaksi</th><th>No. bill</th><th>Outlet</th><th>Status tersimpan</th><th class="r">Total</th></tr></thead>
                                <tbody>
                                    @for (m of missing(); track m.sales_num) {
                                        <tr>
                                            <td>{{ longDate(m.date) }}</td>
                                            <td>{{ m.bill_num || m.sales_num }}</td>
                                            <td>{{ m.branch_code }}</td>
                                            <td><span class="badge" [class]="'badge ' + statusBadge(m.status)">{{ m.status || '–' }}</span></td>
                                            <td class="r">{{ rupiah(m.grand_total) }}</td>
                                        </tr>
                                    }
                                </tbody>
                            </table>
                        </div>
                    </app-panel>
                }
            }

            <app-panel title="Riwayat sinkron" [loading]="logs.isLoading() && !logs.value()" [error]="!!logs.error()" [empty]="!logs.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead>
                            <tr><th>Tanggal data</th><th>Jenis</th><th>Status</th><th class="r">Baris</th><th>Selesai (WIB)</th><th>Keterangan</th></tr>
                        </thead>
                        <tbody>
                            @for (l of logs.value(); track l.id) {
                                <tr>
                                    <td>{{ l.target_date ? longDate(l.target_date) : '–' }}</td>
                                    <td><span class="badge" [class.badge-info]="l.job_name !== 'sync-esb'">{{ logKind(l.job_name) }}</span></td>
                                    <td>
                                        <span class="badge" [class.badge-pos]="l.status === 'success'" [class.badge-neg]="l.status === 'failed'" [class.badge-info]="l.status === 'running'">
                                            {{ logStatus(l.status) }}
                                        </span>
                                    </td>
                                    <td class="r">{{ l.rows_synced == null ? '–' : num(l.rows_synced) }}</td>
                                    <td>{{ wibTime(l.finished_at) }}</td>
                                    <td class="muted-cell">{{ l.error_message ?? '' }}</td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>
        </div>
    `,
})
export class AdminSync {
    private readonly http = inject(HttpClient);

    protected readonly num = num;
    protected readonly rupiah = rupiah;
    protected readonly longDate = longDate;
    protected readonly wibTime = wibTime;

    protected readonly mode = signal<SyncMode>('fill');
    protected readonly start = signal(wibDate(1));
    protected readonly end = signal(wibDate(1));
    protected readonly starting = signal(false);
    protected readonly error = signal('');

    protected readonly job = httpResource<ManualJob | null>(() => '/api/admin/sync');
    protected readonly logs = httpResource<SyncLog[]>(() => '/api/admin/sync/logs?limit=30');

    protected readonly running = computed(() => this.job.value()?.status === 'running');
    protected readonly dayCount = computed(() => {
        const s = this.start(), e = this.end();
        if (!s || !e || s > e) return 0;
        let n = 1;
        for (let d = s; d < e; d = addDays(d, 1)) n++;
        return n;
    });
    protected readonly canStart = computed(() => !this.running() && !this.starting() && this.dayCount() >= 1 && this.dayCount() <= MAX_DAYS);
    protected readonly rangeHint = computed(() => {
        const n = this.dayCount();
        if (n > MAX_DAYS) return `Rentang terlalu panjang: maksimal ${MAX_DAYS} hari per sinkron (dipilih ${n} hari).`;
        return n ? `${n} hari akan disinkronkan (maksimal ${MAX_DAYS} hari per sinkron).` : 'Pilih rentang tanggal yang valid.';
    });

    /** Sums over the successfully synced days of the current job. */
    protected readonly totals = computed(() => {
        const t = { found: 0, sales: 0, refreshed: 0, payments: 0, items: 0 };
        for (const d of this.job.value()?.days ?? []) {
            const r = d.result;
            if (!r?.ok) continue;
            t.found += r.found.sales;
            t.sales += r.sales_new ?? 0;
            t.refreshed += r.sales_refreshed ?? 0;
            t.payments += r.payments ?? 0;
            t.items += r.menu_items ?? 0;
        }
        return t;
    });

    protected readonly changes = computed<DatedChange[]>(() =>
        (this.job.value()?.days ?? []).flatMap((d) => (d.result?.status_changes ?? []).map((c) => ({ ...c, date: d.date }))),
    );
    protected readonly missing = computed<DatedRef[]>(() =>
        (this.job.value()?.days ?? []).flatMap((d) => (d.result?.not_in_esb ?? []).map((b) => ({ ...b, date: d.date }))),
    );

    private previous: string | undefined;

    constructor() {
        // Poll while a job runs so the per-day table fills in live.
        const timer = setInterval(() => {
            if (this.running()) this.job.reload();
        }, 1000);
        inject(DestroyRef).onDestroy(() => clearInterval(timer));

        // When a job finishes, the history has new rows.
        effect(() => {
            const status = this.job.value()?.status;
            if (this.previous === 'running' && status && status !== 'running') this.logs.reload();
            this.previous = status;
        });
    }

    protected startSync(): void {
        const n = this.dayCount();
        if (n > CONFIRM_ABOVE_DAYS && !confirm(`Sinkronkan ${n} hari sekaligus? Ini akan memanggil API ESB minimal ${n} kali.`)) return;

        this.error.set('');
        this.starting.set(true);
        this.http.post<ManualJob>('/api/admin/sync', { date_from: this.start(), date_to: this.end(), mode: this.mode() }).subscribe({
            next: () => {
                this.starting.set(false);
                this.job.reload();
                this.logs.reload();
            },
            error: (e) => {
                this.starting.set(false);
                this.error.set(e?.error?.error ?? 'Gagal memulai sinkron. Coba lagi.');
                this.job.reload();
            },
        });
    }

    protected label(s: SyncStatus): string { return STATUS_LABEL[s]; }
    protected badge(s: SyncStatus): string { return STATUS_BADGE[s]; }
    protected logKind(jobName: string): string { return LOG_KIND[jobName] ?? jobName; }

    /** Green for Finished, red for Void/Cancelled, neutral otherwise. */
    protected statusBadge(status: string): string {
        if (status === 'Finished') return 'badge-pos';
        if (status === 'Void' || status === 'Cancelled') return 'badge-neg';
        return '';
    }

    protected jobLabel(j: ManualJob): string {
        if (j.status === 'running') return `Berjalan (${j.days.filter((d) => d.status === 'done' || d.status === 'failed').length}/${j.days.length})`;
        if (j.status === 'failed') return 'Gagal';
        return j.days.some((d) => d.status === 'failed') ? 'Selesai (ada yang gagal)' : 'Selesai';
    }

    protected note(d: ManualDay, mode: SyncMode): string {
        const r = d.result;
        if (!r) return d.status === 'running' ? 'Mengambil data dari ESB…' : '';
        if (!r.ok) return r.error ?? 'Gagal';
        if (r.found.sales === 0 && !(r.not_in_esb?.length)) return 'Tidak ada transaksi di ESB';
        if (mode === 'refresh') {
            const parts: string[] = [];
            if (r.status_changes?.length) parts.push(`${r.status_changes.length} status berubah`);
            if (r.not_in_esb?.length) parts.push(`${r.not_in_esb.length} tidak ada di ESB`);
            return parts.length ? parts.join(', ') : 'Sudah sama dengan ESB';
        }
        return (r.sales_new ?? 0) === 0 ? 'Sudah lengkap' : 'Data baru ditambahkan';
    }

    protected logStatus(s: string): string {
        return s === 'success' ? 'Sukses' : s === 'failed' ? 'Gagal' : s === 'running' ? 'Berjalan' : s;
    }
}
