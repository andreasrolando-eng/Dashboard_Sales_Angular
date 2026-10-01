import { Injectable, computed, signal } from '@angular/core';
import { presetRange } from './dates';

const WIB_OFFSET_MS = 7 * 60 * 60 * 1000;

/** Calendar date (YYYY-MM-DD) in WIB, `daysAgo` days back from today. */
export function wibDate(daysAgo = 0): string {
    return new Date(Date.now() + WIB_OFFSET_MS - daysAgo * 86_400_000).toISOString().slice(0, 10);
}

/** Global dashboard filters shared by every page (date range + outlet). */
@Injectable({ providedIn: 'root' })
export class Filters {
    // Default window: this month so far, 1st through today (WIB) -- same as the
    // "Bulan ini" preset. Today's own bills only arrive with the next 06:00 sync.
    private readonly defaultRange = presetRange('thisMonth', wibDate(0));
    readonly dateStart = signal(this.defaultRange.start);
    readonly dateEnd = signal(this.defaultRange.end);
    readonly outlet = signal(''); // '' = all outlets

    readonly range = computed(() => ({ dateStart: this.dateStart(), dateEnd: this.dateEnd() }));
    readonly valid = computed(() => !!this.dateStart() && !!this.dateEnd() && this.dateStart() <= this.dateEnd());

}
