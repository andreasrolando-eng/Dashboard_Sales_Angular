// Pure calendar-date helpers for the range picker. Dates are plain
// 'YYYY-MM-DD' strings and all arithmetic is done in UTC, so results never
// shift with the browser's timezone or DST.

const pad2 = (n: number): string => String(n).padStart(2, '0');

export const toIso = (y: number, m: number, d: number): string => {
    const dt = new Date(Date.UTC(y, m, d)); // normalises overflow (m = 12, d = 0, ...)
    return `${dt.getUTCFullYear()}-${pad2(dt.getUTCMonth() + 1)}-${pad2(dt.getUTCDate())}`;
};

export const parseIso = (iso: string): { y: number; m: number; d: number } => {
    const [y, m, d] = iso.split('-').map(Number);
    return { y, m: m - 1, d };
};

export const addDays = (iso: string, days: number): string => {
    const { y, m, d } = parseIso(iso);
    return toIso(y, m, d + days);
};

/** Trigger label format: 2026-09-01 -> "01-09-2026". */
export const formatDisplay = (iso: string): string => {
    const { y, m, d } = parseIso(iso);
    return `${pad2(d)}-${pad2(m + 1)}-${y}`;
};

/** "September 2026" (id-ID). */
export const monthTitle = (y: number, m: number): string =>
    new Date(Date.UTC(y, m, 1)).toLocaleDateString('id-ID', { month: 'long', year: 'numeric', timeZone: 'UTC' });

/** Sunday-first weekday header, matching the id-ID calendars people expect. */
export const WEEKDAYS = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab'] as const;

export interface DayCell {
    iso: string;
    day: number;
    /** Belongs to the previous/next month (shown faded to complete the week rows). */
    outside: boolean;
}

/** Full Sunday-first week rows covering month `m` (0-based) of year `y`. */
export function monthGrid(y: number, m: number): DayCell[] {
    const lead = new Date(Date.UTC(y, m, 1)).getUTCDay();
    const daysInMonth = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
    const total = Math.ceil((lead + daysInMonth) / 7) * 7;
    return Array.from({ length: total }, (_, i) => {
        const dt = new Date(Date.UTC(y, m, 1 - lead + i));
        return { iso: toIso(dt.getUTCFullYear(), dt.getUTCMonth(), dt.getUTCDate()), day: dt.getUTCDate(), outside: dt.getUTCMonth() !== m };
    });
}

/** Month shifted by `delta` months, as { y, m }. */
export function shiftMonth(y: number, m: number, delta: number): { y: number; m: number } {
    const t = y * 12 + m + delta;
    return { y: Math.floor(t / 12), m: ((t % 12) + 12) % 12 };
}

export type PresetKey = 'today' | 'last7' | 'last30' | 'thisMonth';

export const PRESETS: { key: PresetKey; label: string }[] = [
    { key: 'today', label: 'Hari ini' },
    { key: 'last7', label: '7 hari terakhir' },
    { key: 'last30', label: '30 hari terakhir' },
    { key: 'thisMonth', label: 'Bulan ini' },
];

/** Range for a preset, all ending on `today` (inclusive). */
export function presetRange(key: PresetKey, today: string): { start: string; end: string } {
    switch (key) {
        case 'today':
            return { start: today, end: today };
        case 'last7':
            return { start: addDays(today, -6), end: today };
        case 'last30':
            return { start: addDays(today, -29), end: today };
        case 'thisMonth': {
            const { y, m } = parseIso(today);
            return { start: toIso(y, m, 1), end: today };
        }
    }
}
