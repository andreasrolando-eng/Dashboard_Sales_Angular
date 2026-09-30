import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router, UrlTree, provideRouter } from '@angular/router';
import { Auth, LoginError, Me } from './auth';
import { adminGuard, authGuard, guestGuard, unauthorizedInterceptor } from './auth.guards';
import { safeReturnUrl } from '../features/login/login';

const admin: Me = { id: 1, email: 'admin@esb.co.id', name: '', is_admin: true };
const viewer: Me = { id: 2, email: 'biasa@esb.co.id', name: '', is_admin: false };

function setup() {
    TestBed.configureTestingModule({
        providers: [provideRouter([]), provideHttpClient(withInterceptors([unauthorizedInterceptor])), provideHttpClientTesting()],
    });
    return { auth: TestBed.inject(Auth), http: TestBed.inject(HttpTestingController), router: TestBed.inject(Router) };
}

describe('Auth service', () => {
    it('asks /api/me once and remembers the answer', async () => {
        const { auth, http } = setup();
        const first = auth.ensureLoaded();
        const second = auth.ensureLoaded(); // concurrent: must share the request
        http.expectOne('/api/me').flush(admin);
        expect(await first).toEqual(admin);
        expect(await second).toEqual(admin);
        expect(auth.isAdmin()).toBe(true);
        await auth.ensureLoaded(); // cached: no new request
        http.verify();
    });

    it('treats a 401 from /api/me as "signed out", not as an error', async () => {
        const { auth, http } = setup();
        const p = auth.ensureLoaded();
        http.expectOne('/api/me').flush({ error: 'belum login' }, { status: 401, statusText: 'Unauthorized' });
        expect(await p).toBeNull();
        expect(auth.user()).toBeNull();
    });

    it('login stores the user; failures become readable messages', async () => {
        const { auth, http } = setup();
        const ok = auth.login('a@esb.co.id', 'pw');
        http.expectOne('/api/auth/login').flush(viewer);
        expect(await ok).toEqual(viewer);
        expect(auth.user()).toEqual(viewer);

        const cases: [number, string, Record<string, string>?][] = [
            [401, 'Email atau password salah.'],
            [429, 'Terlalu banyak percobaan gagal. Coba lagi dalam 5 menit.', { 'Retry-After': '290' }],
            [0, 'Tidak bisa terhubung ke server'],
            [500, 'Terjadi kesalahan di server'],
        ];
        for (const [status, text, headers] of cases) {
            const p = auth.login('a@esb.co.id', 'pw');
            http.expectOne('/api/auth/login').flush({ error: 'x' }, { status, statusText: 'err', headers });
            await expect(p).rejects.toBeInstanceOf(LoginError);
            await expect(p).rejects.toThrow(text);
        }
    });

    it('logout clears the user and goes to /login even if the API call fails', async () => {
        const { auth, http, router } = setup();
        auth.user.set(admin);
        const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);
        const p = auth.logout();
        http.expectOne('/api/auth/logout').flush(null, { status: 500, statusText: 'err' });
        await p;
        expect(auth.user()).toBeNull();
        expect(navigate).toHaveBeenCalledWith(['/login']);
    });

    it('changePassword surfaces the server message', async () => {
        const { auth, http } = setup();
        const p = auth.changePassword('lama', 'baru-12345');
        http.expectOne('/api/auth/password').flush({ error: 'password saat ini salah' }, { status: 400, statusText: 'Bad Request' });
        await expect(p).rejects.toThrow('Password saat ini salah');
    });
});

describe('Route guards', () => {
    const run = (guard: typeof authGuard, url = '/sales') =>
        TestBed.runInInjectionContext(() => guard({} as never, { url } as never)) as Promise<boolean | UrlTree>;

    it('authGuard lets a signed-in user in', async () => {
        const { auth } = setup();
        auth.user.set(viewer);
        expect(await run(authGuard)).toBe(true);
    });

    it('authGuard sends a visitor to /login and remembers where they were going', async () => {
        const { auth, router } = setup();
        auth.user.set(null);
        const tree = (await run(authGuard, '/sales')) as UrlTree;
        expect(router.serializeUrl(tree)).toBe('/login?returnUrl=%2Fsales');
    });

    it('adminGuard: admin passes, viewer goes home, visitor goes to /login', async () => {
        const { auth, router } = setup();
        auth.user.set(admin);
        expect(await run(adminGuard)).toBe(true);
        auth.user.set(viewer);
        expect(router.serializeUrl((await run(adminGuard)) as UrlTree)).toBe('/');
        auth.user.set(null);
        expect(router.serializeUrl((await run(adminGuard)) as UrlTree)).toBe('/login');
    });

    it('guestGuard skips the login page for signed-in users', async () => {
        const { auth, router } = setup();
        auth.user.set(null);
        expect(await run(guestGuard, '/login')).toBe(true);
        auth.user.set(viewer);
        expect(router.serializeUrl((await run(guestGuard, '/login')) as UrlTree)).toBe('/');
    });
});

describe('unauthorizedInterceptor', () => {
    it('treats a 401 from a data call as an expired session (once)', () => {
        const { auth, http, router } = setup();
        auth.user.set(admin);
        const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);
        const client = TestBed.inject(HttpClient);

        client.get('/api/sales/summary').subscribe({ error: () => undefined });
        http.expectOne('/api/sales/summary').flush({}, { status: 401, statusText: 'Unauthorized' });
        client.get('/api/ops/summary').subscribe({ error: () => undefined });
        http.expectOne('/api/ops/summary').flush({}, { status: 401, statusText: 'Unauthorized' });

        expect(auth.user()).toBeNull();
        expect(navigate).toHaveBeenCalledTimes(1); // the second 401 must not re-navigate
        expect(navigate.mock.calls[0][1]?.queryParams).toMatchObject({ expired: 1 });
    });

    it('ignores 401s from the sign-in call itself', () => {
        const { auth, http, router } = setup();
        auth.user.set(admin);
        const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);
        TestBed.inject(HttpClient).post('/api/auth/login', {}).subscribe({ error: () => undefined });
        http.expectOne('/api/auth/login').flush({}, { status: 401, statusText: 'Unauthorized' });
        expect(auth.user()).toEqual(admin);
        expect(navigate).not.toHaveBeenCalled();
    });
});

describe('safeReturnUrl', () => {
    it('allows in-app paths only', () => {
        expect(safeReturnUrl('/sales?x=1')).toBe('/sales?x=1');
        expect(safeReturnUrl('/admin/users')).toBe('/admin/users');
        for (const bad of [null, undefined, '', 'https://evil.example', '//evil.example', '/\\evil', 'javascript:alert(1)', '/login', '/login?returnUrl=/x']) {
            expect(safeReturnUrl(bad)).toBe('/');
        }
    });
});
