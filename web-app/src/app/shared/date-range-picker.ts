import { TuiIcon } from '@taiga-ui/core';
import { Component, ElementRef, computed, inject, input, model, signal } from '@angular/core';
import {
    PRESETS, PresetKey, WEEKDAYS, formatDisplay, monthGrid, monthTitle, parseIso, presetRange, shiftMonth,
} from '../core/dates';
import { wibDate } from '../core/filters';

interface View { y: number; m: number }

/**
 * Date-range filter: a trigger showing "dd-MM-yyyy → dd-MM-yyyy" that opens a
 * two-month calendar (one month on phones) with quick presets. Click a start
 * day, then an end day; the range applies (and the popover closes) on the
 * second click. Days after today can't be picked.
 */
@Component({
    selector: 'app-date-range',
    imports: [TuiIcon],
    host: {
        '(document:click)': 'onDocumentClick($event)',
        '(keydown.escape)': 'close()',
    },
    template: `
        <div class="drp">
            <button
                type="button"
                class="drp-trigger"
                [class.open]="open()"
                aria-haspopup="dialog"
                [attr.aria-expanded]="open()"
                (click)="toggle()"
            >
                <span>{{ label() }}</span>
                <tui-icon icon="@tui.calendar" />
            </button>

            @if (open()) {
                <div class="drp-pop" role="dialog" aria-label="Pilih rentang tanggal">
                    <div class="drp-months">
                        @for (mo of months(); track mo.key; let i = $index) {
                            <div class="drp-month" [class.second]="i === 1">
                                <div class="drp-head">
                                    <button type="button" class="drp-nav" [class.away]="i === 1" aria-label="Bulan sebelumnya" (click)="shift(-1)">
                                        <tui-icon icon="@tui.chevron-left" />
                                    </button>
                                    <strong>{{ mo.title }}</strong>
                                    <button type="button" class="drp-nav" [class.away]="i === 0" [class.narrow-only]="i === 0" aria-label="Bulan berikutnya" (click)="shift(1)">
                                        <tui-icon icon="@tui.chevron-right" />
                                    </button>
                                </div>
                                <div class="drp-week" aria-hidden="true">
                                    @for (w of weekdays; track w) {
                                        <span>{{ w }}</span>
                                    }
                                </div>
                                <div class="drp-days">
                                    @for (c of mo.cells; track c.iso) {
                                        @if (c.outside) {
                                            <span class="drp-day outside" [class.in]="inRange(c.iso)" aria-hidden="true"><span>{{ c.day }}</span></span>
                                        } @else {
                                            <button
                                                type="button"
                                                class="drp-day"
                                                [class.in]="inRange(c.iso)"
                                                [class.edge]="isEdge(c.iso)"
                                                [class.today]="c.iso === today"
                                                [disabled]="c.iso > max()"
                                                [attr.aria-label]="c.iso"
                                                [attr.aria-pressed]="isEdge(c.iso)"
                                                (click)="pick(c.iso)"
                                                (mouseenter)="hover.set(c.iso)"
                                            >
                                                <span>{{ c.day }}</span>
                                            </button>
                                        }
                                    }
                                </div>
                            </div>
                        }
                    </div>
                    <div class="drp-presets">
                        @for (p of presets; track p.key) {
                            <button type="button" (click)="applyPreset(p.key)">{{ p.label }}</button>
                        }
                    </div>
                </div>
            }
        </div>
    `,
})
export class DateRangePicker {
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    /** Applied range (YYYY-MM-DD). Bind two-way: [(start)] / [(end)]. */
    readonly start = model.required<string>();
    readonly end = model.required<string>();
    /** Last selectable day; defaults to today (WIB). */
    readonly max = input<string>(wibDate(0));

    protected readonly today = wibDate(0);
    protected readonly weekdays = WEEKDAYS;
    protected readonly presets = PRESETS;

    protected readonly open = signal(false);
    /** First day chosen in an in-progress selection; null when not selecting. */
    private readonly pending = signal<string | null>(null);
    protected readonly hover = signal<string | null>(null);
    /** Month shown in the left calendar (the right one is the next month). */
    private readonly view = signal<View>({ y: 0, m: 0 });

    protected readonly label = computed(() => `${formatDisplay(this.start())} → ${formatDisplay(this.end())}`);

    protected readonly months = computed(() => {
        const left = this.view();
        return [left, shiftMonth(left.y, left.m, 1)].map(({ y, m }) => ({
            key: `${y}-${m}`, title: monthTitle(y, m), cells: monthGrid(y, m),
        }));
    });

    /** [lo, hi] currently highlighted: the live selection preview, else the applied filter. */
    private readonly shown = computed<[string, string]>(() => {
        const p = this.pending();
        if (p) {
            const h = this.hover() ?? p;
            return p <= h ? [p, h] : [h, p];
        }
        return [this.start(), this.end()];
    });

    protected inRange(iso: string): boolean {
        const [lo, hi] = this.shown();
        return lo !== hi && iso >= lo && iso <= hi;
    }

    protected isEdge(iso: string): boolean {
        const [lo, hi] = this.shown();
        return iso === lo || iso === hi;
    }

    protected toggle(): void {
        if (this.open()) return this.close();
        // Show the applied range: start month on the left, unless the end is
        // further than one month away, then keep the end visible on the right.
        const s = parseIso(this.start());
        const e = parseIso(this.end());
        const gap = (e.y * 12 + e.m) - (s.y * 12 + s.m);
        this.view.set(gap > 1 ? shiftMonth(e.y, e.m, -1) : { y: s.y, m: s.m });
        this.pending.set(null);
        this.hover.set(null);
        this.open.set(true);
    }

    protected close(): void {
        this.open.set(false);
        this.pending.set(null);
        this.hover.set(null);
    }

    protected shift(delta: number): void {
        const v = this.view();
        this.view.set(shiftMonth(v.y, v.m, delta));
    }

    protected pick(iso: string): void {
        if (iso > this.max()) return;
        const first = this.pending();
        if (!first) {
            this.pending.set(iso);
            this.hover.set(iso);
            return;
        }
        this.apply(first <= iso ? first : iso, first <= iso ? iso : first);
    }

    protected applyPreset(key: PresetKey): void {
        const range = presetRange(key, this.today);
        // Presets end today; clamp to the last selectable day.
        this.apply(range.start > this.max() ? this.max() : range.start, range.end > this.max() ? this.max() : range.end);
    }

    private apply(start: string, end: string): void {
        this.start.set(start);
        this.end.set(end);
        this.close();
    }

    protected onDocumentClick(event: Event): void {
        if (this.open() && !event.composedPath().includes(this.host.nativeElement)) this.close();
    }
}
