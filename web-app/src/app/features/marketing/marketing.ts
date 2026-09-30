import { Component, computed } from '@angular/core';
import { apiResource } from '../../core/api';
import { num, pct, ratio, rupiah } from '../../core/format';
import { PromoRow } from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { Kpi, Panel } from '../../shared/ui';

@Component({
    selector: 'app-marketing',
    imports: [FilterBar, Kpi, Panel],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Marketing</h2>
                <p class="page-sub">Efektivitas promo: lift penjualan dan ROI dibanding biaya diskon.</p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Promo aktif" icon="@tui.megaphone" [value]="num(rows().length)" [loading]="promos.isLoading()" />
                <app-kpi label="Total redemption" icon="@tui.gift" [value]="num(totals().redemptions)" [loading]="promos.isLoading()" />
                <app-kpi label="Revenue dari promo" icon="@tui.banknote" [value]="rupiah(totals().revenue)" [loading]="promos.isLoading()" />
                <app-kpi label="Biaya diskon" icon="@tui.tag" [value]="rupiah(totals().cost)" [loading]="promos.isLoading()" />
            </div>

            <app-panel title="Performa promo" [loading]="promos.isLoading()" [error]="!!promos.error()" [empty]="!rows().length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead>
                            <tr>
                                <th>Promo</th><th class="r">Redemption</th><th class="r">Revenue</th><th class="r">Biaya diskon</th>
                                <th class="r">Lift</th><th class="r">ROI</th><th>Status</th>
                            </tr>
                        </thead>
                        <tbody>
                            @for (p of rows(); track p.promotion_id) {
                                <tr>
                                    <td>{{ p.promotion_name }}</td>
                                    <td class="r">{{ num(p.redemptions) }}</td>
                                    <td class="r">{{ rupiah(p.promo_revenue) }}</td>
                                    <td class="r">{{ rupiah(p.discount_cost) }}</td>
                                    <td class="r">{{ pct(p.lift_pct) }}</td>
                                    <td class="r">{{ ratio(p.roi) }}</td>
                                    <td>
                                        <span class="badge" [class.badge-pos]="p.status === 'Efektif'" [class.badge-neg]="p.status === 'Kurang Efektif'">
                                            {{ p.status }}
                                        </span>
                                    </td>
                                </tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>
        </div>
    `,
})
export class Marketing {
    protected readonly num = num;
    protected readonly pct = pct;
    protected readonly ratio = ratio;
    protected readonly rupiah = rupiah;

    protected readonly promos = apiResource<PromoRow[]>('/marketing/promo-performance');
    protected readonly rows = computed(() => this.promos.value() ?? []);
    protected readonly totals = computed(() =>
        this.rows().reduce(
            (a, p) => ({ redemptions: a.redemptions + p.redemptions, revenue: a.revenue + p.promo_revenue, cost: a.cost + p.discount_cost }),
            { redemptions: 0, revenue: 0, cost: 0 },
        ),
    );
}
