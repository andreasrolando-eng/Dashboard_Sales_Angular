import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideTaiga } from '@taiga-ui/core';
import { ManualJob } from '../../../core/models';
import { AdminSync } from './sync';

const job = (over: Partial<ManualJob> = {}): ManualJob => ({
    id: '1', mode: 'fill', status: 'done', date_from: '2026-09-22', date_to: '2026-09-23', started_at: '2026-09-29T10:00:00Z',
    days: [
        { date: '2026-09-22', status: 'done', result: { date: '2026-09-22', ok: true, sales_new: 2, sales_refreshed: 3, sales: 2, payments: 1, menu_items: 3, found: { outlets: 1, sales: 5, payments: 4, items: 9 }, existing: { outlets: 1, sales: 3, payments: 3, items: 6 } } },
        { date: '2026-09-23', status: 'done', result: { date: '2026-09-23', ok: true, sales_new: 0, sales_refreshed: 0, found: { outlets: 0, sales: 0, payments: 0, items: 0 }, existing: { outlets: 0, sales: 0, payments: 0, items: 0 } } },
    ],
    ...over,
});

describe('AdminSync', () => {
    const setup = async (current: ManualJob | null) => {
        await TestBed.configureTestingModule({
            imports: [AdminSync],
            providers: [provideTaiga(), provideHttpClient(), provideHttpClientTesting()],
        }).compileComponents();
        const fixture = TestBed.createComponent(AdminSync);
        const http = TestBed.inject(HttpTestingController);
        fixture.detectChanges();
        http.expectOne('/api/admin/sync').flush(current);
        http.expectOne('/api/admin/sync/logs?limit=30').flush([]);
        await fixture.whenStable();
        fixture.detectChanges();
        return { fixture, http, el: fixture.nativeElement as HTMLElement };
    };

    it('explains that only missing data is added and starts enabled for the default range', async () => {
        const { el } = await setup(null);
        expect(el.textContent).toContain('Hanya melengkapi');
        expect(el.textContent).toContain('1 hari akan disinkronkan');
        expect((el.querySelector('.form-row .btn') as HTMLButtonElement).disabled).toBe(false);
    });

    it('posts the chosen range to the API', async () => {
        const { fixture, http, el } = await setup(null);
        (el.querySelector('.form-row .btn') as HTMLButtonElement).click();
        const req = http.expectOne('/api/admin/sync');
        expect(req.request.method).toBe('POST');
        expect(Object.keys(req.request.body).sort()).toEqual(['date_from', 'date_to', 'mode']);
        expect(req.request.body.mode).toBe('fill'); // the safe, non-overwriting mode is the default
        req.flush(job({ status: 'running' }), { status: 202, statusText: 'Accepted' });
        fixture.detectChanges();
        // the page re-reads job + logs after starting
        http.expectOne('/api/admin/sync').flush(job({ status: 'running' }));
        http.expectOne('/api/admin/sync/logs?limit=30').flush([]);
    });

    it('shows the API error message when the server refuses to start', async () => {
        const { fixture, http, el } = await setup(null);
        (el.querySelector('.form-row .btn') as HTMLButtonElement).click();
        http.expectOne((r) => r.method === 'POST').flush({ error: 'sinkron manual lain masih berjalan' }, { status: 409, statusText: 'Conflict' });
        fixture.detectChanges(); // lets the resource issue its reload
        http.expectOne('/api/admin/sync').flush(null);
        fixture.detectChanges();
        expect(el.querySelector('[role=alert]')?.textContent).toContain('sinkron manual lain masih berjalan');
    });

    it('reports totals and per-day zeros for a finished job', async () => {
        const { el } = await setup(job());
        const values = [...el.querySelectorAll('.kpi-value')].map((k) => k.textContent?.trim());
        expect(values[0]).toBe('2'); // bill baru ditambahkan
        expect(values[1]).toBe('3'); // sudah ada (dilewati)
        expect(values[2]).toBe('1'); // pembayaran baru
        expect(values[3]).toBe('3'); // item menu baru
        const rows = el.querySelectorAll('tbody tr');
        expect(rows[1].textContent).toContain('Tidak ada transaksi di ESB');
        // a day with nothing inserted shows 0, not a dash
        expect(rows[1].querySelectorAll('td')[3].textContent?.trim()).toBe('0');
    });

    it('disables the button while a job is running', async () => {
        const { el } = await setup(job({ status: 'running' }));
        expect((el.querySelector('.form-row .btn') as HTMLButtonElement).disabled).toBe(true);
    });

    describe('refresh mode (pick up voids)', () => {
        const refreshJob = (): ManualJob =>
            job({
                mode: 'refresh',
                days: [
                    {
                        date: '2026-09-09', status: 'done',
                        result: {
                            date: '2026-09-09', ok: true, sales_new: 0, sales_refreshed: 4,
                            found: { outlets: 1, sales: 4, payments: 3, items: 9 },
                            status_changes: [{
                                sales_num: 'SIET1', bill_num: 'IET202609090004', branch_code: 'IET', status: 'Finished',
                                grand_total: 575000, from: 'Finished', to: 'Void',
                            }],
                            not_in_esb: [{ sales_num: 'SGONE', bill_num: 'IET202609090009', branch_code: 'IET', status: 'Finished', grand_total: 1000 }],
                        },
                    },
                ],
            });

        it('defaults to fill mode and posts mode "refresh" once chosen', async () => {
            const { fixture, http, el } = await setup(null);
            const button = () => el.querySelector('.form-row .btn') as HTMLButtonElement;
            expect((el.querySelector('input[name=sync-mode]:checked') as HTMLInputElement).value).toBe('fill');
            expect(button().textContent).toContain('Mulai sinkron');

            (el.querySelectorAll('input[name=sync-mode]')[1] as HTMLInputElement).click();
            fixture.detectChanges();
            expect(button().textContent).toContain('Mulai perbarui');

            button().click();
            const req = http.expectOne((r) => r.method === 'POST');
            expect(req.request.body.mode).toBe('refresh');
            req.flush(refreshJob(), { status: 202, statusText: 'Accepted' });
            fixture.detectChanges();
            http.expectOne('/api/admin/sync').flush(refreshJob());
            http.expectOne('/api/admin/sync/logs?limit=30').flush([]);
        });

        it('lists the bills whose status changed, e.g. Finished -> Void', async () => {
            const { el } = await setup(refreshJob());
            const labels = [...el.querySelectorAll('.kpi-label')].map((k) => k.textContent?.trim());
            expect(labels).toEqual(['Bill baru ditambahkan', 'Bill diperbarui', 'Status berubah', 'Tidak ada di ESB']);
            const values = [...el.querySelectorAll('.kpi-value')].map((k) => k.textContent?.trim());
            expect(values).toEqual(['0', '4', '1', '1']);

            const panel = [...el.querySelectorAll('.card')].find((c) => c.querySelector('h3')?.textContent === 'Perubahan status');
            expect(panel).toBeTruthy();
            expect(panel!.textContent).toContain('IET202609090004');
            const badges = [...panel!.querySelectorAll('.badge')].map((b) => b.textContent?.trim());
            expect(badges).toEqual(['Finished', 'Void']);
            expect(panel!.querySelector('.badge-neg')?.textContent?.trim()).toBe('Void');
        });

        it('reports bills ESB no longer returns without implying they were changed', async () => {
            const { el } = await setup(refreshJob());
            const panel = [...el.querySelectorAll('.card')].find((c) => c.querySelector('h3')?.textContent?.includes('tidak lagi dikembalikan ESB'));
            expect(panel).toBeTruthy();
            expect(panel!.textContent).toContain('IET202609090009');
            expect(panel!.textContent).toContain('tidak diubah');
        });

        it('shows no status panels for a plain fill job', async () => {
            const { el } = await setup(job());
            const titles = [...el.querySelectorAll('.card h3')].map((h) => h.textContent);
            expect(titles).not.toContain('Perubahan status');
        });
    });
});
