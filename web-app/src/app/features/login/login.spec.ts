import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { Login } from './login';

describe('Login page', () => {
    const setup = async (query: Record<string, string> = {}) => {
        await TestBed.configureTestingModule({
            imports: [Login],
            providers: [
                provideRouter([]),
                provideTaiga(),
                provideHttpClient(),
                provideHttpClientTesting(),
                { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap(query) } } },
            ],
        }).compileComponents();
        const fixture = TestBed.createComponent(Login);
        await fixture.whenStable();
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;
        const email = el.querySelector('input[name=email]') as HTMLInputElement;
        const password = el.querySelector('input[name=password]') as HTMLInputElement;
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
        const submit = async () => {
            (el.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit', { cancelable: true }));
            fixture.detectChanges();
            await fixture.whenStable();
        };
        // Zoneless: whenStable() doesn't track plain promise chains, so wait a macrotask.
        const settle = async () => {
            await new Promise((r) => setTimeout(r, 0));
            fixture.detectChanges();
        };
        return { fixture, el, email, password, navigate, submit, settle, http: TestBed.inject(HttpTestingController) };
    };
    const alertText = (el: HTMLElement) => el.querySelector('[role=alert]')?.textContent?.trim();

    it('has labelled email and password fields with the right autocomplete hints', async () => {
        const { el, email, password } = await setup();
        expect(email.type).toBe('email');
        expect(email.autocomplete).toBe('username');
        expect(password.type).toBe('password');
        expect(password.autocomplete).toBe('current-password');
        expect(el.textContent).toContain('Masuk untuk melanjutkan');
    });

    it('does not call the API when a field is empty', async () => {
        const { el, email, submit, http } = await setup();
        email.value = 'a@esb.co.id';
        await submit();
        expect(alertText(el)).toBe('Email dan password wajib diisi.');
        http.expectNone('/api/auth/login');
    });

    it('shows the server message for a wrong password and lets the user retry', async () => {
        const { fixture, el, email, password, submit, settle, http } = await setup();
        email.value = 'a@esb.co.id';
        password.value = 'salah';
        await submit();
        const req = http.expectOne('/api/auth/login');
        expect(req.request.body).toEqual({ email: 'a@esb.co.id', password: 'salah' });
        req.flush({ error: 'email atau password salah' }, { status: 401, statusText: 'Unauthorized' });
        await settle();
        expect(alertText(el)).toBe('Email atau password salah.');
        expect((el.querySelector('button[type=submit]') as HTMLButtonElement).disabled).toBe(false);

        password.dispatchEvent(new Event('input')); // typing clears the error
        fixture.detectChanges();
        expect(alertText(el)).toBeUndefined();
    });

    it('goes back to the page the visitor wanted after signing in', async () => {
        const { email, password, navigate, submit, settle, http } = await setup({ returnUrl: '/sales?x=1' });
        email.value = ' a@esb.co.id ';
        password.value = 'benar-123';
        await submit();
        const req = http.expectOne('/api/auth/login');
        expect(req.request.body.email).toBe('a@esb.co.id'); // trimmed
        req.flush({ id: 1, email: 'a@esb.co.id', name: '', is_admin: false });
        await settle();
        expect(navigate).toHaveBeenCalledWith('/sales?x=1');
    });

    it('ignores an off-site returnUrl (no open redirect)', async () => {
        const { email, password, navigate, submit, settle, http } = await setup({ returnUrl: '//evil.example.com' });
        email.value = 'a@esb.co.id';
        password.value = 'benar-123';
        await submit();
        http.expectOne('/api/auth/login').flush({ id: 1, email: 'a@esb.co.id', name: '', is_admin: false });
        await settle();
        expect(navigate).toHaveBeenCalledWith('/');
    });

    it('toggles password visibility', async () => {
        const { fixture, el, password } = await setup();
        (el.querySelector('.login-reveal') as HTMLButtonElement).click();
        fixture.detectChanges();
        expect(password.type).toBe('text');
        (el.querySelector('.login-reveal') as HTMLButtonElement).click();
        fixture.detectChanges();
        expect(password.type).toBe('password');
    });

    it('tells the user when their session ended', async () => {
        const { el } = await setup({ expired: '1' });
        expect(el.querySelector('[role=status]')?.textContent).toContain('Sesi Anda sudah berakhir');
    });
});
