import { Component, computed, inject, signal } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { apiResource } from '../../core/api';
import { Filters } from '../../core/filters';
import { longDate, num, pct, ratio, rupiah } from '../../core/format';
import { MembershipSummary, NewMembersWeek, TopMember } from '../../core/models';
import { FilterBar } from '../../shared/filter-bar';
import { Select, SelectOption } from '../../shared/select';
import { ChartItem, Columns, Kpi, Panel } from '../../shared/ui';

interface MemberOption { member_code: string; member_name: string }
interface MenuPurchase { menu_id: string; menu_name: string; qty: number; revenue: number; last_purchase_date: string }

@Component({
    selector: 'app-membership',
    imports: [FilterBar, Kpi, Panel, Columns, Select],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Membership</h2>
                <p class="page-sub">Member aktif, retensi, dan member dengan spending tertinggi.</p>
            </div>
            <app-filter-bar />

            <div class="kpi-grid">
                <app-kpi label="Total member" icon="@tui.users" [value]="num(s()?.total_members)" [loading]="summary.isLoading()" />
                <app-kpi label="Member aktif" icon="@tui.user-check" [value]="num(s()?.active_members)" [hint]="pct(s()?.active_pct) + ' dari total'" [loading]="summary.isLoading()" />
                <app-kpi label="Retensi" icon="@tui.trending-up" [value]="pct(s()?.retention_pct)" hint="vs periode sebelumnya" [loading]="summary.isLoading()" />
                <app-kpi label="Churn" icon="@tui.circle-user-round" [value]="pct(s()?.churn_pct)" [loading]="summary.isLoading()" />
                <app-kpi label="Kunjungan" icon="@tui.store" [value]="ratio(s()?.visit_frequency)" hint="rata-rata per member aktif" [loading]="summary.isLoading()" />
            </div>

            <div class="grid-2">
                <app-panel title="Member baru per minggu" [loading]="weekly.isLoading()" [error]="!!weekly.error()" [empty]="!weeklyItems().length">
                    <app-columns [items]="weeklyItems()" [format]="wholeOnly" ariaLabel="Member baru per minggu" />
                </app-panel>

                <app-panel title="Top member" [loading]="top.isLoading()" [error]="!!top.error()" [empty]="!top.value()?.length">
                    <div class="tbl-wrap">
                        <table class="tbl">
                            <thead><tr><th>Member</th><th>Tier</th><th class="r">Kunjungan</th><th class="r">Spending</th><th>Menu favorit</th></tr></thead>
                            <tbody>
                                @for (m of top.value(); track m.member_code) {
                                    <tr>
                                        <td>{{ m.member_name }}<br /><small class="page-sub">{{ m.outlet_name }}</small></td>
                                        <td><span class="badge">{{ m.tier || '–' }}</span></td>
                                        <td class="r">{{ num(m.visits) }}</td>
                                        <td class="r">{{ rupiah(m.spending) }}</td>
                                        <td>{{ m.favorite_menu || '–' }}</td>
                                    </tr>
                                }
                            </tbody>
                        </table>
                    </div>
                </app-panel>
            </div>

            <app-panel title="Riwayat menu per member" [loading]="purchases.isLoading() && !!member()" [error]="!!purchases.error()">
                <div panel-actions>
                    <app-select [options]="memberOptions()" [(value)]="member" placeholder="Pilih member…" ariaLabel="Pilih member" />
                </div>
                @if (!member()) {
                    <p class="state">Pilih member untuk melihat menu yang pernah dibeli.</p>
                } @else if (!purchases.value()?.length) {
                    <p class="state">Member ini belum punya pembelian di rentang tanggal ini.</p>
                } @else {
                    <div class="tbl-wrap">
                        <table class="tbl">
                            <thead><tr><th>Menu</th><th class="r">Qty</th><th class="r">Revenue</th><th>Terakhir dibeli</th></tr></thead>
                            <tbody>
                                @for (p of purchases.value(); track p.menu_id) {
                                    <tr>
                                        <td>{{ p.menu_name }}</td>
                                        <td class="r">{{ num(p.qty) }}</td>
                                        <td class="r">{{ rupiah(p.revenue) }}</td>
                                        <td>{{ longDate(p.last_purchase_date) }}</td>
                                    </tr>
                                }
                            </tbody>
                        </table>
                    </div>
                }
            </app-panel>
        </div>
    `,
})
export class Membership {
    protected readonly num = num;
    /** Member counts are whole numbers; hide fractional axis ticks. */
    protected readonly wholeOnly = (n: number): string => (Number.isInteger(n) ? String(n) : '');
    protected readonly pct = pct;
    protected readonly ratio = ratio;
    protected readonly rupiah = rupiah;
    protected readonly longDate = longDate;
    protected readonly member = signal('');

    protected readonly summary = apiResource<MembershipSummary>('/membership/summary');
    protected readonly s = computed(() => this.summary.value());
    protected readonly top = apiResource<TopMember[]>('/membership/top-members', { extra: () => ({ limit: 10 }) });
    protected readonly weekly = apiResource<NewMembersWeek[]>('/membership/new-weekly');
    protected readonly options = apiResource<MemberOption[]>('/membership/member-options', { dated: false });

    private readonly filters = inject(Filters);
    protected readonly purchases = httpResource<MenuPurchase[]>(() => {
        const code = this.member();
        if (!code || !this.filters.valid()) return undefined;
        const params: Record<string, string> = { ...this.filters.range() };
        if (this.filters.outlet()) params['outlet'] = this.filters.outlet();
        return { url: `/api/membership/members/${encodeURIComponent(code)}/menu-purchases`, params };
    });

    protected readonly memberOptions = computed<SelectOption[]>(() =>
        (this.options.value() ?? []).map((o) => ({ value: o.member_code, label: `${o.member_name} (${o.member_code})` })),
    );

    protected readonly weeklyItems = computed<ChartItem[]>(() =>
        (this.weekly.value() ?? []).map((w) => ({
            label: longDate(w.week_start), value: w.new_members, tip: `Minggu ${longDate(w.week_start)}: ${w.new_members} member baru`,
        })),
    );
}
