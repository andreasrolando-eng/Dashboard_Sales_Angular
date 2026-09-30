import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { apiResource } from '../../core/api';
import { Filters } from '../../core/filters';
import { compact, longDate, num, pct, rupiah, shortDate } from '../../core/format';
import {
    BillsPage, HourlyRow, MenuPerformanceRow, OutletRevenueRow, SalesDailyRow, SalesSummary, TopProduct,
} from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { ChartItem, Columns, HBars, Kpi, Panel } from '../../shared/ui';

const TREND_LABEL: Record<string, string> = { naik: 'Naik', turun: 'Turun', stagnan: 'Stagnan' };

@Component({
    selector: 'app-sales',
    imports: [FilterBar, Kpi, Panel, Columns, HBars],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Sales</h2>
                <p class="page-sub">Revenue, transaksi, dan performa menu per outlet.</p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Revenue" icon="@tui.banknote" [value]="rupiah(summary.value()?.revenue)" [loading]="summary.isLoading()" />
                <app-kpi label="Nett sales" icon="@tui.wallet" [value]="rupiah(summary.value()?.nett_sales)" [loading]="summary.isLoading()" />
                <app-kpi label="Transaksi" icon="@tui.receipt" [value]="num(summary.value()?.trans_count)" [loading]="summary.isLoading()" />
                <app-kpi label="Rata-rata per bill" icon="@tui.shopping-cart" [value]="rupiah(avgBill())" [loading]="summary.isLoading()" />
                <app-kpi label="Porsi member" icon="@tui.users" [value]="pct(memberShare())" [loading]="summary.isLoading()" hint="dari total revenue" />
            </div>

            <div class="grid-2">
                <app-panel title="Revenue harian" [loading]="daily.isLoading()" [error]="!!daily.error()" [empty]="!dailyItems().length">
                    <app-columns [items]="dailyItems()" [format]="compactRp" ariaLabel="Revenue harian" />
                </app-panel>
                <app-panel title="Jam ramai (revenue per jam)" [loading]="hourly.isLoading()" [error]="!!hourly.error()" [empty]="!hourlyItems().length">
                    <app-columns [items]="hourlyItems()" [format]="compactRp" ariaLabel="Revenue per jam" />
                </app-panel>
            </div>

            <div class="grid-2 start">
                <app-panel title="Revenue per outlet" [loading]="outlets.isLoading()" [error]="!!outlets.error()" [empty]="!outletItems().length">
                    <app-hbars [items]="outletItems()" [format]="rupiah" />
                </app-panel>
                <app-panel title="Top 10 produk" [loading]="top.isLoading()" [error]="!!top.error()" [empty]="!top.value()?.length">
                    <div class="tbl-wrap">
                        <table class="tbl">
                            <thead><tr><th>Menu</th><th>Kategori</th><th class="r">Qty</th><th class="r">Revenue</th></tr></thead>
                            <tbody>
                                @for (p of top.value(); track p.menu_id) {
                                    <tr>
                                        <td>{{ p.menu_name }}</td>
                                        <td>{{ p.category }}</td>
                                        <td class="r">{{ num(p.qty) }}</td>
                                        <td class="r">{{ rupiah(p.revenue) }}</td>
                                    </tr>
                                }
                            </tbody>
                        </table>
                    </div>
                </app-panel>
            </div>

            <app-panel title="Performa menu" [loading]="menus.isLoading()" [error]="!!menus.error()" [empty]="!menus.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead>
                            <tr><th>Menu</th><th>Kategori</th><th class="r">Qty</th><th class="r">Revenue</th><th class="r">Kontribusi</th><th>Tren</th><th>Catatan</th></tr>
                        </thead>
                        <tbody>
                            @for (m of menus.value(); track m.menu_id) {
                                <tr>
                                    <td>{{ m.menu_name }}</td>
                                    <td>{{ m.category }} · {{ m.category_detail }}</td>
                                    <td class="r">{{ num(m.qty) }}</td>
                                    <td class="r">{{ rupiah(m.revenue) }}</td>
                                    <td class="r">{{ pct(m.contribution_pct) }}</td>
                                    <td>
                                        @if (m.trend === 'stagnan') {
                                            <span class="muted">Stagnan</span>
                                        } @else {
                                            <span class="badge" [class.badge-pos]="m.trend === 'naik'" [class.badge-neg]="m.trend === 'turun'">{{ trendLabel(m.trend) }}</span>
                                        }
                                    </td>
                                    <td>@if (m.is_takeout_candidate) { <span class="badge badge-warn">Kandidat takeout</span> }</td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>

            <app-panel title="Daftar bill" [loading]="bills.isLoading()" [error]="!!bills.error()" [empty]="!bills.value()?.rows?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead><tr><th>No. bill</th><th>Tanggal</th><th>Outlet</th><th class="r">Grand total</th></tr></thead>
                        <tbody>
                            @for (b of bills.value()?.rows; track b.bill_num) {
                                <tr>
                                    <td>{{ b.bill_num }}</td>
                                    <td>{{ longDate(b.sales_date) }}</td>
                                    <td>{{ b.branch_code }}</td>
                                    <td class="r">{{ rupiah(b.grand_total) }}</td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
                <div class="pager">
                    <span>Halaman {{ page() }} dari {{ pageCount() }} · {{ num(bills.value()?.total_count) }} bill</span>
                    <button type="button" class="btn btn-ghost" [disabled]="page() <= 1" (click)="page.set(page() - 1)">Sebelumnya</button>
                    <button type="button" class="btn btn-ghost" [disabled]="page() >= pageCount()" (click)="page.set(page() + 1)">Berikutnya</button>
                </div>
            </app-panel>
        </div>
    `,
})
export class Sales {
    protected readonly rupiah = rupiah;
    protected readonly compactRp = (n: number): string => (n ? compact(n) : '0');
    protected readonly num = num;
    protected readonly pct = pct;
    protected readonly longDate = longDate;
    protected readonly pageSize = 20;
    protected readonly page = signal(1);

    private readonly filters = inject(Filters);

    constructor() {
        // A new filter means a new result set: go back to the first page of bills.
        effect(() => {
            this.filters.range();
            this.filters.outlet();
            untracked(() => this.page.set(1));
        });
    }

    protected readonly summary = apiResource<SalesSummary>('/sales/summary');
    protected readonly daily = apiResource<SalesDailyRow[]>('/sales/daily');
    protected readonly hourly = apiResource<HourlyRow[]>('/sales/hourly');
    // Revenue-by-outlet is a cross-outlet comparison, so it ignores the outlet filter.
    protected readonly outlets = apiResource<OutletRevenueRow[]>('/sales/revenue-by-outlet', { useOutlet: false });
    protected readonly top = apiResource<TopProduct[]>('/sales/top-products', { extra: () => ({ sortBy: 'revenue', limit: 10 }) });
    protected readonly menus = apiResource<MenuPerformanceRow[]>('/sales/menu-performance');
    protected readonly bills = apiResource<BillsPage>('/sales/bills', {
        extra: () => ({ page: this.page(), pageSize: this.pageSize }),
    });

    protected readonly avgBill = computed(() => {
        const s = this.summary.value();
        return s && s.trans_count ? s.revenue / s.trans_count : null;
    });
    protected readonly memberShare = computed(() => {
        const s = this.summary.value();
        return s && s.revenue ? (s.member_revenue / s.revenue) * 100 : null;
    });
    protected readonly pageCount = computed(() =>
        Math.max(1, Math.ceil((this.bills.value()?.total_count ?? 0) / this.pageSize)),
    );

    /** /sales/daily is per (date, outlet); the chart wants one bar per date. */
    protected readonly dailyItems = computed<ChartItem[]>(() => {
        const byDate = new Map<string, number>();
        for (const r of this.daily.value() ?? []) byDate.set(r.sales_date, (byDate.get(r.sales_date) ?? 0) + r.revenue);
        return [...byDate.entries()]
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([date, revenue]) => ({ label: shortDate(date), value: revenue, tip: `${longDate(date)}: ${rupiah(revenue)}` }));
    });
    protected readonly hourlyItems = computed<ChartItem[]>(() => {
        const rows = this.hourly.value() ?? [];
        if (!rows.length) return [];
        const byHour = new Map(rows.map((r) => [r.hour_of_day, r]));
        return Array.from({ length: 24 }, (_, h) => ({
            label: String(h).padStart(2, '0'),
            value: byHour.get(h)?.revenue ?? 0,
            tip: `${String(h).padStart(2, '0')}:00 — ${rupiah(byHour.get(h)?.revenue ?? 0)} (${byHour.get(h)?.trans_count ?? 0} transaksi)`,
        }));
    });
    protected readonly outletItems = computed<ChartItem[]>(() =>
        (this.outlets.value() ?? []).map((o) => ({ label: o.branch_name, value: o.revenue })),
    );

    protected trendLabel(t: string): string {
        return TREND_LABEL[t] ?? t;
    }
}
