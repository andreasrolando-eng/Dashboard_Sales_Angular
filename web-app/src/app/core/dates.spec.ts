import { describe, expect, it } from 'vitest';
import { addDays, formatDisplay, monthGrid, monthTitle, presetRange, shiftMonth } from './dates';

describe('dates', () => {
    it('formats the trigger label as dd-MM-yyyy', () => {
        expect(formatDisplay('2026-09-01')).toBe('01-09-2026');
    });

    it('adds days across month and year boundaries', () => {
        expect(addDays('2026-09-01', -1)).toBe('2026-08-31');
        expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
        expect(addDays('2026-03-01', -1)).toBe('2026-02-28');
    });

    it('shifts months across year boundaries in both directions', () => {
        expect(shiftMonth(2026, 11, 1)).toEqual({ y: 2027, m: 0 });
        expect(shiftMonth(2026, 0, -1)).toEqual({ y: 2025, m: 11 });
    });

    it('titles months in Indonesian', () => {
        expect(monthTitle(2026, 9)).toBe('Oktober 2026');
    });

    it('builds September 2026 like the reference (Sunday-first, 5 rows, leading 30/31)', () => {
        const grid = monthGrid(2026, 8);
        expect(grid.length).toBe(35);
        expect(grid[0]).toEqual({ iso: '2026-08-30', day: 30, outside: true });
        expect(grid[1].iso).toBe('2026-08-31');
        expect(grid[2]).toEqual({ iso: '2026-09-01', day: 1, outside: false });
        expect(grid[31].iso).toBe('2026-09-30');
        expect(grid[34]).toEqual({ iso: '2026-10-03', day: 3, outside: true });
    });

    it('only adds rows that are needed (Feb 2026 starts on Sunday: exactly 4 rows)', () => {
        expect(monthGrid(2026, 1).length).toBe(28);
    });

    it('computes presets ending today', () => {
        expect(presetRange('today', '2026-09-29')).toEqual({ start: '2026-09-29', end: '2026-09-29' });
        expect(presetRange('last7', '2026-09-29')).toEqual({ start: '2026-09-23', end: '2026-09-29' });
        expect(presetRange('last30', '2026-09-29')).toEqual({ start: '2026-08-31', end: '2026-09-29' });
        expect(presetRange('thisMonth', '2026-09-29')).toEqual({ start: '2026-09-01', end: '2026-09-29' });
    });
});
