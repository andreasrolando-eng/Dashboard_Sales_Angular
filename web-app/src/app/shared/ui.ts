import { TuiIcon } from '@taiga-ui/core';
import { Component, computed, input } from '@angular/core';

@Component({
    selector: 'app-kpi',
    imports: [TuiIcon],
    template: `
        <div class="kpi">
            <span class="kpi-label">{{ label() }}</span>
            <strong class="kpi-value" [title]="loading() ? '' : value()">{{ loading() ? '…' : value() }}</strong>
            @if (hint()) {
                <span class="kpi-hint">{{ hint() }}</span>
            }
            @if (icon()) {
                <span class="kpi-icon"><tui-icon [icon]="icon()" /></span>
            }
        </div>
    `,
})
export class Kpi {
    readonly label = input.required<string>();
    readonly value = input<string>('–');
    readonly hint = input<string>('');
    readonly loading = input(false);
    readonly icon = input<string>('');
}

/** Card with a title and loading / error / empty states around projected content. */
@Component({
    selector: 'app-panel',
    template: `
        <section class="card">
            <header class="card-head">
                <h3>{{ title() }}</h3>
                <ng-content select="[panel-actions]" />
            </header>
            @if (error()) {
                <p class="state state-error">Gagal memuat data. Coba ubah filter atau muat ulang halaman.</p>
            } @else if (loading()) {
                <div class="skeleton" aria-busy="true" aria-label="Memuat">
                    <span></span><span></span><span></span>
                </div>
            } @else if (empty()) {
                <p class="state">Tidak ada data untuk filter ini.</p>
            } @else {
                <ng-content />
            }
        </section>
    `,
})
export class Panel {
    readonly title = input.required<string>();
    readonly loading = input(false);
    readonly error = input(false);
    readonly empty = input(false);
}

export interface ChartItem {
    label: string;
    value: number;
    tip?: string;
}

/** Vertical bar chart (time series, hourly). */
@Component({
    selector: 'app-columns',
    template: `
        <div class="chart">
            <div class="chart-y" aria-hidden="true">
                <span>{{ axis().max }}</span>
                <span>{{ axis().mid }}</span>
                <span>0</span>
            </div>
            <div class="columns" role="img" [attr.aria-label]="ariaLabel()">
                @for (b of bars(); track $index) {
                    <div class="col" [title]="b.tip">
                        <div class="col-bar" [style.height.%]="b.pct"></div>
                        <span class="col-label">{{ b.showLabel ? b.label : '' }}</span>
                    </div>
                }
            </div>
        </div>
    `,
})
export class Columns {
    readonly items = input.required<ChartItem[]>();
    readonly ariaLabel = input('Grafik batang');
    /** Formats the y-axis labels (defaults to the plain number). */
    readonly format = input<(n: number) => string>((n) => String(n));
    /** Axis ceiling rounded up to 1 / 2 / 2.5 / 5 x 10^k so the labels read cleanly. */
    private readonly ceiling = computed(() => {
        const m = Math.max(...this.items().map((i) => i.value), 0);
        if (m <= 0) return 1;
        const p = 10 ** Math.floor(Math.log10(m));
        const n = m / p;
        return p * (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10);
    });
    protected readonly axis = computed(() => ({
        max: this.format()(this.ceiling()),
        mid: this.format()(this.ceiling() / 2),
    }));
    protected readonly bars = computed(() => {
        const items = this.items();
        const max = this.ceiling();
        const step = Math.ceil(items.length / 12);
        return items.map((i, idx) => ({
            ...i,
            tip: i.tip ?? `${i.label}: ${i.value}`,
            pct: Math.max((i.value / max) * 100, i.value > 0 ? 2 : 0),
            showLabel: idx % step === 0,
        }));
    });
}

/** Horizontal ranked bars (revenue by outlet, top products). */
@Component({
    selector: 'app-hbars',
    template: `
        <ul class="hbars">
            @for (r of rows(); track r.label) {
                <li>
                    <div class="hbar-top">
                        <span class="hbar-name">{{ r.label }}</span>
                        <span class="hbar-val">{{ r.display }}</span>
                    </div>
                    <div class="hbar-track"><div class="hbar-fill" [style.width.%]="r.pct"></div></div>
                </li>
            }
        </ul>
    `,
})
export class HBars {
    readonly items = input.required<ChartItem[]>();
    readonly format = input<(n: number) => string>((n) => String(n));
    protected readonly rows = computed(() => {
        const items = this.items();
        const max = Math.max(...items.map((i) => i.value), 0) || 1;
        return items.map((i) => ({ label: i.label, display: this.format()(i.value), pct: (i.value / max) * 100 }));
    });
}
