import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { Auth } from '../../../core/auth';
import { AdminUsers } from './users';

const user = (id: number, email: string, over: object = {}) => ({
    id, email, name: '', is_admin: false, created_at: '2026-09-28T00:00:00Z', has_password: true, last_login_at: null, ...over,
});

describe('AdminUsers', () => {
    const setup = async () => {
        await TestBed.configureTestingModule({
            imports: [AdminUsers],
            providers: [provideRouter([]), provideTaiga(), provideHttpClient(), provideHttpClientTesting()],
        }).compileComponents();
        TestBed.inject(Auth).user.set({ id: 1, email: 'Admin@esb.co.id', name: '', is_admin: true });
        const fixture = TestBed.createComponent(AdminUsers);
        const http = TestBed.inject(HttpTestingController);
        fixture.detectChanges();
        http.expectOne('/api/admin/users').flush([
            user(1, 'admin@esb.co.id', { is_admin: true }),
            user(2, 'budi@esb.co.id'),
            user(3, 'baru@esb.co.id', { has_password: false }),
        ]);
        await fixture.whenStable();
        fixture.detectChanges();
        return { fixture, http, el: fixture.nativeElement as HTMLElement };
    };
    const rows = (el: HTMLElement) => [...el.querySelectorAll('tbody tr:not(.edit-row)')] as HTMLElement[];

    it('shows who can sign in and who still needs a password', async () => {
        const { el } = await setup();
        const text = rows(el).map((r) => r.textContent?.replace(/\s+/g, ' '));
        expect(text[1]).toContain('Sudah diatur');
        expect(text[2]).toContain('Belum diatur');
    });

    it('marks the current user and gives them no Hapus button (case-insensitive match)', async () => {
        const { el } = await setup();
        const [me, other] = rows(el);
        expect(me.textContent).toContain('Anda');
        expect([...me.querySelectorAll('button')].map((b) => b.textContent?.trim())).toEqual(['Atur password']);
        expect([...other.querySelectorAll('button')].map((b) => b.textContent?.trim())).toEqual(['Atur password', 'Hapus']);
    });

    it('sends the initial password when adding a user, and refuses a short one locally', async () => {
        const { fixture, http, el } = await setup();
        const form = el.querySelector('form.filter-bar') as HTMLFormElement;
        const [email, password] = [...form.querySelectorAll('input')] as HTMLInputElement[];
        email.value = 'Baru2@esb.co.id';
        password.value = 'pendek';
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        fixture.detectChanges();
        http.expectNone((r) => r.method === 'POST');
        expect(el.querySelector('.notice')?.textContent).toContain('minimal 8');

        password.value = 'cukup-panjang';
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        const req = http.expectOne((r) => r.method === 'POST' && r.url === '/api/admin/users');
        expect(req.request.body).toEqual({ email: 'Baru2@esb.co.id', is_admin: false, password: 'cukup-panjang' });
        req.flush({});
    });

    it('resets a password through the inline form and hits the admin endpoint', async () => {
        const { fixture, http, el } = await setup();
        ([...rows(el)[1].querySelectorAll('button')].find((b) => b.textContent?.includes('Atur password')) as HTMLButtonElement).click();
        fixture.detectChanges();
        const edit = el.querySelector('.edit-row form') as HTMLFormElement;
        (edit.querySelector('input') as HTMLInputElement).value = 'reset-baru-99';
        edit.dispatchEvent(new Event('submit', { cancelable: true }));
        const req = http.expectOne('/api/admin/users/budi%40esb.co.id/password');
        expect(req.request.body).toEqual({ password: 'reset-baru-99' });
        req.flush(null, { status: 204, statusText: 'No Content' });
        fixture.detectChanges();
        http.expectOne('/api/admin/users').flush([]);
    });
});
