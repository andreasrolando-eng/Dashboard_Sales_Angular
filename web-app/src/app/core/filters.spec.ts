import { TestBed } from '@angular/core/testing';
import { Filters } from './filters';

describe('Filters default range', () => {
    afterEach(() => vi.useRealTimers());

    const defaultsAt = (utc: string) => {
        vi.useFakeTimers({ toFake: ['Date'] });
        vi.setSystemTime(new Date(utc));
        TestBed.resetTestingModule();
        return TestBed.inject(Filters).range();
    };

    it('is this month so far: the 1st through today', () => {
        expect(defaultsAt('2026-10-15T05:00:00Z')).toEqual({ dateStart: '2026-10-01', dateEnd: '2026-10-15' });
    });

    it('uses the WIB calendar date, not UTC', () => {
        // 17:30 UTC on Sep 30 is 00:30 WIB on Oct 1 -> October, not September.
        expect(defaultsAt('2026-09-30T17:30:00Z')).toEqual({ dateStart: '2026-10-01', dateEnd: '2026-10-01' });
    });
});
