import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { Auth, Me } from '../core/auth';
import { Shell } from './shell';

const admin: Me = { id: 1, email: 'admin@esb.co.id', name: '', is_admin: true };
const viewer: Me = { id: 2, email: 'biasa@esb.co.id', name: 'Budi', is_admin: false };

describe('Shell', () => {
    const setup = async (me: Me) => {
        await TestBed.configureTestingModule({
            imports: [Shell],
            providers: [provideRouter([]), provideTaiga(), provideHttpClient(), provideHttpClientTesting()],
        }).compileComponents();
        TestBed.inject(Auth).user.set(me);
        const fixture = TestBed.createComponent(Shell);
        await fixture.whenStable();
        fixture.detectChanges();
        return { fixture, el: fixture.nativeElement as HTMLElement, http: TestBed.inject(HttpTestingController) };
    };
    const labels = (el: HTMLElement) => [...el.querySelectorAll('.side-link')].map((a) => a.textContent?.trim());

    it('shows the app name and the signed-in user', async () => {
        const { el } = await setup(viewer);
        expect(el.textContent).toContain('Dashboard Sales');
        expect(el.querySelector('.user-mail')?.textContent).toContain('Budi'); // name wins over email
        expect(el.querySelector('.avatar')?.textContent?.trim()).toBe('B');
    });

    it('shows every nav item to an admin', async () => {
        const { el } = await setup(admin);
        expect(labels(el)).toEqual(['Overview', 'Sales', 'Ops', 'Membership', 'Marketing', 'Non Sales', 'Kelola User', 'Sinkron Data']);
    });

    it('hides the admin-only pages from a regular user', async () => {
        const { el } = await setup(viewer);
        expect(labels(el)).toEqual(['Overview', 'Sales', 'Ops', 'Membership', 'Marketing', 'Non Sales']);
    });

    it('logs out through the API and returns to the login form', async () => {
        const { el, http } = await setup(admin);
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
        (el.querySelector('[aria-label=Keluar]') as HTMLButtonElement).click();
        const req = http.expectOne('/api/auth/logout');
        expect(req.request.method).toBe('POST');
        req.flush(null, { status: 204, statusText: 'No Content' });
        await new Promise((r) => setTimeout(r, 0));
        expect(TestBed.inject(Auth).user()).toBeNull();
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });

    it('toggles the theme and collapses the sidebar', async () => {
        const { fixture, el } = await setup(admin);
        const shell = el.querySelector('.shell') as HTMLElement;

        (el.querySelector('[aria-label="Buka/tutup menu"]') as HTMLButtonElement).click();
        fixture.detectChanges();
        expect(shell.classList.contains('collapsed')).toBe(true);

        const toggle = el.querySelector('[aria-label="Mode gelap"], [aria-label="Mode terang"]') as HTMLButtonElement;
        const before = toggle.getAttribute('aria-label');
        toggle.click();
        fixture.detectChanges();
        const after = (el.querySelector('[aria-label="Mode gelap"], [aria-label="Mode terang"]') as HTMLButtonElement).getAttribute('aria-label');
        expect(after).not.toBe(before);
    });
});
