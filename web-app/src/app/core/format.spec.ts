import { describe, expect, it } from 'vitest';
import { duration, num, pct, rupiah } from './format';
import { wibDate } from './filters';

describe('format', () => {
    it('formats rupiah with id-ID thousands separators', () => {
        expect(rupiah(624550)).toBe('Rp 624.550');
        expect(rupiah(0)).toBe('Rp 0');
    });

    it('renders an en dash for missing values', () => {
        expect(rupiah(null)).toBe('–');
        expect(num(undefined)).toBe('–');
        expect(pct(null)).toBe('–');
    });

    it('formats percentages with one decimal and a decimal comma', () => {
        expect(pct(55.5)).toBe('55,5%');
    });

    it('formats durations', () => {
        expect(duration(0)).toBe('–');
        expect(duration(600)).toBe('10 menit');
        expect(duration(4500)).toBe('1 j 15 m');
    });
});

describe('wibDate', () => {
    it('returns YYYY-MM-DD and steps back by whole days', () => {
        expect(wibDate(0)).toMatch(/^\d{4}-\d{2}-\d{2}$/);
        const diff = (new Date(wibDate(0)).getTime() - new Date(wibDate(7)).getTime()) / 86_400_000;
        expect(diff).toBe(7);
    });
});
