import { Component, computed } from '@angular/core';
import { apiResource } from '../../core/api';
import { compact, longDate, num, pct, ratio, rupiah, shortDate } from '../../core/format';
import {
    MembershipSummary, OpsSummary, OutletRevenueRow, PromoRow, SalesDailyRow, SalesSummary,
} from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { ChartItem, Columns, HBars, Kpi, Panel } from '../../shared/ui';

@Component({
    selector: 'app-overview',
    imports: [FilterBar, Kpi, Panel, Columns, HBars],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Overview</h2>
                <p class="page-sub">Ringkasan revenue, operasional, membership, dan promo lintas outlet.</p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Revenue" icon="@tui.banknote" [value]="rupiah(sales.value()?.revenue)" [loading]="sales.isLoading()" />
                <app-kpi label="Nett sales" icon="@tui.wallet" [value]="rupiah(sales.value()?.nett_sales)" [loading]="sales.isLoading()" />
                <app-kpi label="Transaksi" icon="@tui.receipt" [value]="num(sales.value()?.trans_count)" [loading]="sales.isLoading()" />
                <app-kpi label="Member aktif" icon="@tui.user-check" [value]="num(members.value()?.active_members)" [hint]="'retensi ' + pct(members.value()?.retention_pct)" [loading]="members.isLoading()" />
                <app-kpi label="Batal / void" icon="@tui.ban" [value]="num(cancelled())" hint="transaksi" [loading]="ops.isLoading()" />
            </div>

            <div class="grid-2">
                <app-panel title="Revenue harian" [loading]="daily.isLoading()" [error]="!!daily.error()" [empty]="!dailyItems().length">
                    <app-columns [items]="dailyItems()" [format]="compactRp" ariaLabel="Revenue harian" />
                </app-panel>
                <app-panel title="Revenue per outlet" [loading]="outlets.isLoading()" [error]="!!outlets.error()" [empty]="!outletItems().length">
                    <app-hbars [items]="outletItems()" [format]="rupiah" />
                </app-panel>
            </div>

            <app-panel title="Promo teratas" [loading]="promos.isLoading()" [error]="!!promos.error()" [empty]="!promos.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead><tr><th>Promo</th><th class="r">Redemption</th><th class="r">Revenue</th><th class="r">ROI</th><th>Status</th></tr></thead>
                        <tbody>
                            @for (p of topPromos(); track p.promotion_id) {
                                <tr>
                                    <td>{{ p.promotion_name }}</td>
                                    <td class="r">{{ num(p.redemptions) }}</td>
                                    <td class="r">{{ rupiah(p.promo_revenue) }}</td>
                                    <td class="r">{{ ratio(p.roi) }}</td>
                                    <td><span class="badge" [class.badge-pos]="p.status === 'Efektif'" [class.badge-neg]="p.status === 'Kurang Efektif'">{{ p.status }}</span></td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>
        </div>
    `,
})
export class Overview {
    protected readonly rupiah = rupiah;
    protected readonly compactRp = (n: number): string => (n ? compact(n) : '0');
    protected readonly num = num;
    protected readonly pct = pct;
    protected readonly ratio = ratio;

    protected readonly sales = apiResource<SalesSummary>('/sales/summary');
    protected readonly daily = apiResource<SalesDailyRow[]>('/sales/daily');
    protected readonly outlets = apiResource<OutletRevenueRow[]>('/sales/revenue-by-outlet', { useOutlet: false });
    protected readonly ops = apiResource<OpsSummary>('/ops/summary');
    protected readonly members = apiResource<MembershipSummary>('/membership/summary');
    protected readonly promos = apiResource<PromoRow[]>('/marketing/promo-performance');

    protected readonly cancelled = computed(() => {
        const t = this.ops.value()?.totals;
        return t ? t.cancelled_count + t.void_count : null;
    });
    protected readonly topPromos = computed(() => (this.promos.value() ?? []).slice(0, 5));
    protected readonly dailyItems = computed<ChartItem[]>(() => {
        const byDate = new Map<string, number>();
        for (const r of this.daily.value() ?? []) byDate.set(r.sales_date, (byDate.get(r.sales_date) ?? 0) + r.revenue);
        return [...byDate.entries()]
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([date, revenue]) => ({ label: shortDate(date), value: revenue, tip: `${longDate(date)}: ${rupiah(revenue)}` }));
    });
    protected readonly outletItems = computed<ChartItem[]>(() =>
        (this.outlets.value() ?? []).map((o) => ({ label: o.branch_name, value: o.revenue })),
    );
}
