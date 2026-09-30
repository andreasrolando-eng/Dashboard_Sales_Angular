import { TuiIcon } from '@taiga-ui/core';
import { Component, ElementRef, computed, inject, input, model, signal } from '@angular/core';

export interface SelectOption {
    value: string;
    label: string;
}

let nextId = 0;

/**
 * Dropdown with a rounded popover (the native <select> list is drawn by the
 * OS and can't be styled). Keyboard: Enter/Space/ArrowDown open, arrows move,
 * Home/End jump, Enter/Space pick, Esc/Tab close.
 */
@Component({
    selector: 'app-select',
    imports: [TuiIcon],
    host: {
        '(document:click)': 'onDocumentClick($event)',
        '(keydown.escape)': 'close(true)',
    },
    template: `
        <div class="sel">
            <button
                type="button"
                class="sel-trigger"
                [class.open]="open()"
                [class.placeholder]="!selected()"
                role="combobox"
                aria-haspopup="listbox"
                [attr.aria-expanded]="open()"
                [attr.aria-controls]="listId"
                [attr.aria-label]="ariaLabel() || null"
                (click)="toggle()"
                (keydown)="onTriggerKey($event)"
            >
                <span class="sel-value">{{ selected()?.label ?? placeholder() }}</span>
                <tui-icon icon="@tui.chevron-down" />
            </button>

            @if (open()) {
                <ul class="sel-pop" role="listbox" [id]="listId" tabindex="-1">
                    @for (o of options(); track o.value; let i = $index) {
                        <li
                            role="option"
                            class="sel-opt"
                            [class.active]="i === active()"
                            [class.chosen]="o.value === value()"
                            [attr.aria-selected]="o.value === value()"
                            (click)="choose(o)"
                            (mouseenter)="active.set(i)"
                        >
                            <span>{{ o.label }}</span>
                            @if (o.value === value()) {
                                <tui-icon icon="@tui.check" />
                            }
                        </li>
                    }
                </ul>
            }
        </div>
    `,
})
export class Select {
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
    protected readonly listId = `app-select-${nextId++}`;

    readonly options = input.required<SelectOption[]>();
    readonly value = model('');
    readonly placeholder = input('Pilih…');
    readonly ariaLabel = input('');

    protected readonly open = signal(false);
    protected readonly active = signal(0);
    protected readonly selected = computed(() => this.options().find((o) => o.value === this.value()));

    protected toggle(): void {
        if (this.open()) this.close(false);
        else this.show();
    }

    private show(): void {
        const idx = this.options().findIndex((o) => o.value === this.value());
        this.active.set(Math.max(idx, 0));
        this.open.set(true);
        // Keep the chosen option in view for long lists.
        queueMicrotask(() =>
            this.host.nativeElement.querySelector('.sel-opt.chosen')?.scrollIntoView?.({ block: 'nearest' }),
        );
    }

    protected close(refocus: boolean): void {
        if (!this.open()) return;
        this.open.set(false);
        if (refocus) this.host.nativeElement.querySelector<HTMLElement>('.sel-trigger')?.focus();
    }

    protected choose(o: SelectOption): void {
        this.value.set(o.value);
        this.close(true);
    }

    protected onTriggerKey(e: KeyboardEvent): void {
        const n = this.options().length;
        if (!n) return;
        if (!this.open()) {
            if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
                e.preventDefault();
                this.show();
            }
            return;
        }
        switch (e.key) {
            case 'ArrowDown': this.move(1, n); break;
            case 'ArrowUp': this.move(-1, n); break;
            case 'Home': this.active.set(0); break;
            case 'End': this.active.set(n - 1); break;
            case 'Enter':
            case ' ': this.choose(this.options()[this.active()]); break;
            case 'Tab': this.close(false); return;
            default: return;
        }
        e.preventDefault();
        queueMicrotask(() =>
            this.host.nativeElement.querySelector('.sel-opt.active')?.scrollIntoView?.({ block: 'nearest' }),
        );
    }

    private move(delta: number, n: number): void {
        this.active.set((this.active() + delta + n) % n);
    }

    protected onDocumentClick(event: Event): void {
        if (this.open() && !event.composedPath().includes(this.host.nativeElement)) this.close(false);
    }
}
