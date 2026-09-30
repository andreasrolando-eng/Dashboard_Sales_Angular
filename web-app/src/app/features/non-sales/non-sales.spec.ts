import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { NonSales } from './non-sales';

describe('NonSales page', () => {
    const setup = async () => {
        await TestBed.configureTestingModule({
            imports: [NonSales],
            providers: [provideRouter([]), provideTaiga(), provideHttpClient(), provideHttpClientTesting()],
        }).compileComponents();
        const fixture = TestBed.createComponent(NonSales);
        const http = TestBed.inject(HttpTestingController);
        fixture.detectChanges(); // issues the requests; answer them before awaiting stability
        const flush = (path: string, body: object | null) => http.expectOne((r) => r.url === path).flush(body);
        flush('/api/meta/outlets', []);
        flush('/api/meta/last-sync', null);
        flush('/api/non-sales/summary', { value: 12345678901, trans_count: 3, avg_value: 4115226300, outlet_count: 2 });
        flush('/api/non-sales/daily', [{ sales_date: '2026-09-22T00:00:00Z', value: 40000, trans_count: 1 }]);
        flush('/api/non-sales/by-outlet', [{ branch_code: 'BR01', branch_name: 'Cabang A', value: 40000, trans_count: 1 }]);
        flush('/api/non-sales/top-menus', [{ menu_id: 'M1', menu_name: 'Kopi Tamu', category: 'Minuman', qty: 2, value: 40000 }]);
        flush('/api/non-sales/bills', {
            rows: [{ bill_num: 'B-1', sales_date: '2026-09-22T00:00:00Z', branch_code: 'BR01', grand_total: 40000, payment_method: 'Compliment', member_name: null }],
            total_count: 1,
        });
        await fixture.whenStable();
        fixture.detectChanges();
        return { el: fixture.nativeElement as HTMLElement, http };
    };

    it('asks the non sales endpoints with the global date range', async () => {
        const { http } = await setup();
        http.verify(); // every request above was matched exactly once
    });

    it('shows totals, top menus and bills with their payment method', async () => {
        const { el } = await setup();
        const values = [...el.querySelectorAll('.kpi-value')].map((k) => k.textContent?.trim());
        expect(values[0]).toBe('Rp 12.345.678.901');
        expect(values[1]).toBe('3');
        expect(el.textContent).toContain('Kopi Tamu');
        const billRow = [...el.querySelectorAll('tbody tr')].find((r) => r.textContent?.includes('B-1'));
        expect(billRow?.textContent).toContain('Compliment');
        expect(billRow?.textContent).toContain('Rp 40.000');
    });
});
