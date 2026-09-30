import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { apiResource } from '../../core/api';
import { Filters } from '../../core/filters';
import { compact, longDate, num, rupiah, shortDate } from '../../core/format';
import { NonSalesBillsPage, NonSalesDaily, NonSalesMenu, NonSalesOutlet, NonSalesSummary } from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { ChartItem, Columns, HBars, Kpi, Panel } from '../../shared/ui';

/**
 * Non sales: Finished bills paid with payment method type 7 (e.g. compliment).
 * They are excluded from every sales figure on the other pages and shown here.
 */
@Component({
    selector: 'app-non-sales',
    imports: [FilterBar, Kpi, Panel, Columns, HBars],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Non Sales</h2>
                <p class="page-sub">
                    Transaksi dengan metode pembayaran tipe 7 (mis. compliment). Tidak dihitung di revenue dan transaksi halaman lain.
                </p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Nilai non sales" icon="@tui.hand-coins" [value]="rupiah(summary.value()?.value)" [loading]="summary.isLoading()" hint="total grand total bill" />
                <app-kpi label="Transaksi" icon="@tui.receipt" [value]="num(summary.value()?.trans_count)" [loading]="summary.isLoading()" />
                <app-kpi label="Rata-rata per bill" icon="@tui.shopping-cart" [value]="rupiah(summary.value()?.trans_count ? summary.value()?.avg_value : null)" [loading]="summary.isLoading()" />
                <app-kpi label="Outlet" icon="@tui.store" [value]="num(summary.value()?.outlet_count)" [loading]="summary.isLoading()" hint="yang punya non sales" />
            </div>

            <div class="grid-2">
                <app-panel title="Nilai non sales harian" [loading]="daily.isLoading()" [error]="!!daily.error()" [empty]="!dailyItems().length">
                    <app-columns [items]="dailyItems()" [format]="compactRp" ariaLabel="Nilai non sales harian" />
                </app-panel>
                <app-panel title="Non sales per outlet" [loading]="outlets.isLoading()" [error]="!!outlets.error()" [empty]="!outletItems().length">
                    <app-hbars [items]="outletItems()" [format]="rupiah" />
                </app-panel>
            </div>

            <app-panel title="Menu terbanyak (non sales)" [loading]="menus.isLoading()" [error]="!!menus.error()" [empty]="!menus.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead><tr><th>Menu</th><th>Kategori</th><th class="r">Qty</th><th class="r">Nilai</th></tr></thead>
                        <tbody>
                            @for (m of menus.value(); track m.menu_id) {
                                <tr>
                                    <td>{{ m.menu_name || m.menu_id }}</td>
                                    <td>{{ m.category || '–' }}</td>
                                    <td class="r">{{ num(m.qty) }}</td>
                                    <td class="r">{{ rupiah(m.value) }}</td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>

            <app-panel title="Daftar bill non sales" [loading]="bills.isLoading()" [error]="!!bills.error()" [empty]="!bills.value()?.rows?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead>
                            <tr><th>No. bill</th><th>Tanggal</th><th>Outlet</th><th>Metode pembayaran</th><th>Member</th><th class="r">Grand total</th></tr>
                        </thead>
                        <tbody>
                            @for (b of bills.value()?.rows; track $index) {
                                <tr>
                                    <td>{{ b.bill_num || '–' }}</td>
                                    <td>{{ longDate(b.sales_date) }}</td>
                                    <td>{{ b.branch_code }}</td>
                                    <td><span class="badge badge-info">{{ b.payment_method || 'Tipe 7' }}</span></td>
                                    <td>{{ b.member_name || '–' }}</td>
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
export class NonSales {
    protected readonly rupiah = rupiah;
    protected readonly num = num;
    protected readonly longDate = longDate;
    protected readonly compactRp = (n: number): string => (n ? compact(n) : '0');
    protected readonly pageSize = 20;
    protected readonly page = signal(1);
    private readonly filters = inject(Filters);

    constructor() {
        // A new filter means a new result set: back to the first page of bills.
        effect(() => {
            this.filters.range();
            this.filters.outlet();
            untracked(() => this.page.set(1));
        });
    }

    protected readonly summary = apiResource<NonSalesSummary>('/non-sales/summary');
    protected readonly daily = apiResource<NonSalesDaily[]>('/non-sales/daily');
    // Cross-outlet comparison: ignores the outlet filter, like Sales.
    protected readonly outlets = apiResource<NonSalesOutlet[]>('/non-sales/by-outlet', { useOutlet: false });
    protected readonly menus = apiResource<NonSalesMenu[]>('/non-sales/top-menus', { extra: () => ({ limit: 10 }) });
    protected readonly bills = apiResource<NonSalesBillsPage>('/non-sales/bills', {
        extra: () => ({ page: this.page(), pageSize: this.pageSize }),
    });

    protected readonly pageCount = computed(() => Math.max(1, Math.ceil((this.bills.value()?.total_count ?? 0) / this.pageSize)));

    protected readonly dailyItems = computed<ChartItem[]>(() =>
        (this.daily.value() ?? []).map((d) => ({
            label: shortDate(d.sales_date),
            value: d.value,
            tip: `${longDate(d.sales_date)}: ${rupiah(d.value)} (${d.trans_count} bill)`,
        })),
    );
    protected readonly outletItems = computed<ChartItem[]>(() =>
        (this.outlets.value() ?? []).map((o) => ({ label: `${o.branch_name} · ${o.trans_count} bill`, value: o.value })),
    );
}
