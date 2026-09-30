import { Injectable, computed, signal } from '@angular/core';

const WIB_OFFSET_MS = 7 * 60 * 60 * 1000;

/** Calendar date (YYYY-MM-DD) in WIB, `daysAgo` days back from today. */
export function wibDate(daysAgo = 0): string {
    return new Date(Date.now() + WIB_OFFSET_MS - daysAgo * 86_400_000).toISOString().slice(0, 10);
}

/** Global dashboard filters shared by every page (date range + outlet). */
@Injectable({ providedIn: 'root' })
export class Filters {
    // Default window: the last 7 complete days (ETL syncs yesterday at 06:00 WIB).
    readonly dateStart = signal(wibDate(7));
    readonly dateEnd = signal(wibDate(1));
    readonly outlet = signal(''); // '' = all outlets

    readonly range = computed(() => ({ dateStart: this.dateStart(), dateEnd: this.dateEnd() }));
    readonly valid = computed(() => !!this.dateStart() && !!this.dateEnd() && this.dateStart() <= this.dateEnd());

}
