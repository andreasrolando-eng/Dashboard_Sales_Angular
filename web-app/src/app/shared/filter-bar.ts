import { Component, computed, inject, input } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { Filters } from '../core/filters';
import { LastSync, Outlet } from '../core/models';
import { DateRangePicker } from './date-range-picker';
import { Select, SelectOption } from './select';

@Component({
    selector: 'app-filter-bar',
    imports: [DateRangePicker, Select],
    template: `
        <div class="filter-bar">
            <div class="field">
                <span>Periode</span>
                <app-date-range [(start)]="f.dateStart" [(end)]="f.dateEnd" />
            </div>
            @if (showOutlet()) {
                <div class="field">
                    <span>Outlet</span>
                    <app-select [options]="outletOptions()" [(value)]="f.outlet" ariaLabel="Outlet" />
                </div>
            }
            @if (lastSyncText(); as t) {
                <span class="sync-note">Sinkron terakhir: {{ t }}</span>
            }
        </div>
        @if (!f.valid()) {
            <p class="notice">Rentang tanggal tidak valid — tanggal awal harus sebelum atau sama dengan tanggal akhir.</p>
        }
    `,
})
export class FilterBar {
    protected readonly f = inject(Filters);
    protected readonly outlets = httpResource<Outlet[]>(() => '/api/meta/outlets');
    private readonly lastSync = httpResource<LastSync>(() => '/api/meta/last-sync');
    readonly showOutlet = input(true);

    protected readonly outletOptions = computed<SelectOption[]>(() => [
        { value: '', label: 'Semua outlet' },
        ...(this.outlets.value() ?? []).map((o) => ({ value: o.branch_code, label: o.branch_name })),
    ]);

    protected readonly lastSyncText = computed(() => {
        const s = this.lastSync.value();
        if (!s?.finished_at) return '';
        const when = new Date(s.finished_at).toLocaleString('id-ID', {
            day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta',
        });
        return s.status === 'success' ? when + ' WIB' : `${when} WIB (${s.status})`;
    });
}
