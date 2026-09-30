import { Component, computed } from '@angular/core';
import { apiResource } from '../../core/api';
import { duration, num, pct, rupiah } from '../../core/format';
import { OpsSummary } from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { ChartItem, HBars, Kpi, Panel } from '../../shared/ui';

@Component({
    selector: 'app-ops',
    imports: [FilterBar, Kpi, Panel, HBars],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Ops</h2>
                <p class="page-sub">Kualitas operasional: transaksi batal/void, durasi makan, dan diskon.</p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Transaksi selesai" icon="@tui.receipt" [value]="num(t()?.trans_count_finished)" [hint]="'dari ' + num(t()?.trans_count_all) + ' total'" [loading]="ops.isLoading()" />
                <app-kpi label="Batal / void" icon="@tui.ban" [value]="num((t()?.cancelled_count ?? 0) + (t()?.void_count ?? 0))" [hint]="cancelRate()" [loading]="ops.isLoading()" />
                <app-kpi label="Rata-rata durasi" icon="@tui.timer" [value]="duration(avgDwell())" hint="per transaksi selesai" [loading]="ops.isLoading()" />
                <app-kpi label="Total tamu (pax)" icon="@tui.users" [value]="num(t()?.pax_total_sum)" [loading]="ops.isLoading()" />
                <app-kpi label="Total diskon" icon="@tui.tag" [value]="rupiah(totalDiscount())" [loading]="ops.isLoading()" />
            </div>

            <div class="grid-2">
                <app-panel title="Revenue per channel" [loading]="ops.isLoading()" [error]="!!ops.error()" [empty]="!channels().length">
                    <app-hbars [items]="channels()" [format]="rupiah" />
                </app-panel>
                <app-panel title="Metode pembayaran" [loading]="ops.isLoading()" [error]="!!ops.error()" [empty]="!payments().length">
                    <app-hbars [items]="payments()" [format]="rupiah" />
                </app-panel>
            </div>

            <app-panel title="Rincian diskon" [loading]="ops.isLoading()" [error]="!!ops.error()" [empty]="!t()">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead><tr><th>Jenis</th><th class="r">Nilai</th><th class="r">% dari revenue</th></tr></thead>
                        <tbody>
                            @for (d of discounts(); track d.label) {
                                <tr><td>{{ d.label }}</td><td class="r">{{ rupiah(d.value) }}</td><td class="r">{{ pct(d.share) }}</td></tr>
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>
        </div>
    `,
})
export class Ops {
    protected readonly rupiah = rupiah;
    protected readonly num = num;
    protected readonly pct = pct;
    protected readonly duration = duration;

    protected readonly ops = apiResource<OpsSummary>('/ops/summary');
    protected readonly t = computed(() => this.ops.value()?.totals);

    protected readonly avgDwell = computed(() => {
        const t = this.t();
        return t && t.dwell_sample_count ? t.dwell_seconds_sum / t.dwell_sample_count : null;
    });
    protected readonly totalDiscount = computed(() => {
        const t = this.t();
        return t ? t.menu_discount_sum + t.promotion_discount_sum + t.voucher_discount_sum : null;
    });
    protected readonly cancelRate = computed(() => {
        const t = this.t();
        return t && t.trans_count_all ? pct(((t.cancelled_count + t.void_count) / t.trans_count_all) * 100) + ' dari total' : '';
    });
    protected readonly channels = computed<ChartItem[]>(() =>
        (this.ops.value()?.by_channel ?? []).map((c) => ({ label: `${c.channel} · ${c.trans_count} trx`, value: c.revenue })),
    );
    protected readonly payments = computed<ChartItem[]>(() =>
        (this.ops.value()?.by_payment_method ?? []).map((p) => ({
            label: `${p.payment_method_type_name} · ${p.payment_count} trx`, value: p.payment_amount,
        })),
    );
    protected readonly discounts = computed(() => {
        const t = this.t();
        if (!t) return [];
        const revenue = (this.ops.value()?.by_channel ?? []).reduce((s, c) => s + c.revenue, 0);
        const share = (v: number) => (revenue ? (v / revenue) * 100 : null);
        return [
            { label: 'Diskon menu', value: t.menu_discount_sum, share: share(t.menu_discount_sum) },
            { label: 'Diskon promosi', value: t.promotion_discount_sum, share: share(t.promotion_discount_sum) },
            { label: 'Diskon voucher', value: t.voucher_discount_sum, share: share(t.voucher_discount_sum) },
        ];
    });
}
