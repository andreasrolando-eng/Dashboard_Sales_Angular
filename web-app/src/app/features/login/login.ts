import { TUI_DARK_MODE, TuiIcon } from '@taiga-ui/core';
import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Auth, LoginError } from '../../core/auth';

/** Only same-app paths are allowed as a post-login target (no open redirect). */
export function safeReturnUrl(url: string | null | undefined): string {
    if (!url || !url.startsWith('/') || url.startsWith('//') || url.startsWith('/login') || url.includes('\\')) return '/';
    return url;
}

@Component({
    selector: 'app-login',
    imports: [TuiIcon],
    template: `
        <main class="login-page">
            <button type="button" class="icon-btn login-theme" (click)="toggleTheme()" [attr.aria-label]="dark() ? 'Mode terang' : 'Mode gelap'">
                <tui-icon [icon]="dark() ? '@tui.sun' : '@tui.moon'" />
            </button>

            <section class="login-card" aria-labelledby="login-title">
                <img class="login-logo" src="icon.png" alt="" width="56" height="56" />
                <h1 id="login-title">Dashboard Sales ESB</h1>
                <p class="login-sub">Masuk untuk melanjutkan</p>

                @if (expired()) {
                    <p class="notice" role="status">Sesi Anda sudah berakhir. Silakan masuk lagi.</p>
                }

                <form (submit)="submit($event, email.value, password.value)" novalidate>
                    <label class="login-field">
                        <span>Email</span>
                        <input
                            #email
                            type="email"
                            name="email"
                            autocomplete="username"
                            placeholder="nama@esb.co.id"
                            required
                            autofocus
                            [disabled]="busy()"
                            (input)="error.set('')"
                        />
                    </label>

                    <label class="login-field">
                        <span>Password</span>
                        <span class="login-password">
                            <input
                                #password
                                [type]="reveal() ? 'text' : 'password'"
                                name="password"
                                autocomplete="current-password"
                                placeholder="Password"
                                required
                                [disabled]="busy()"
                                (input)="error.set('')"
                            />
                            <button
                                type="button"
                                class="icon-btn login-reveal"
                                (click)="reveal.set(!reveal())"
                                [attr.aria-label]="reveal() ? 'Sembunyikan password' : 'Tampilkan password'"
                                [attr.aria-pressed]="reveal()"
                            >
                                <tui-icon [icon]="reveal() ? '@tui.eye-off' : '@tui.eye'" />
                            </button>
                        </span>
                    </label>

                    @if (error(); as e) {
                        <p class="login-error" role="alert">{{ e }}</p>
                    }

                    <button type="submit" class="btn login-submit" [disabled]="busy()">
                        {{ busy() ? 'Memeriksa…' : 'Masuk' }}
                    </button>
                </form>

                <p class="login-help">Lupa password? Hubungi admin untuk mengatur ulang.</p>
            </section>
        </main>
    `,
})
export class Login {
    private readonly auth = inject(Auth);
    private readonly router = inject(Router);
    private readonly route = inject(ActivatedRoute);
    protected readonly dark = inject(TUI_DARK_MODE);

    protected readonly busy = signal(false);
    protected readonly error = signal('');
    protected readonly reveal = signal(false);
    protected readonly expired = signal(this.route.snapshot.queryParamMap.get('expired') === '1');

    protected toggleTheme(): void {
        this.dark.set(!this.dark());
    }

    protected async submit(event: Event, email: string, password: string): Promise<void> {
        event.preventDefault();
        if (this.busy()) return;
        if (!email.trim() || !password) {
            this.error.set('Email dan password wajib diisi.');
            return;
        }
        this.busy.set(true);
        this.error.set('');
        this.expired.set(false);
        try {
            await this.auth.login(email.trim(), password);
            await this.router.navigateByUrl(safeReturnUrl(this.route.snapshot.queryParamMap.get('returnUrl')));
        } catch (e) {
            this.error.set(e instanceof LoginError ? e.message : 'Terjadi kesalahan. Coba lagi.');
            this.busy.set(false);
        }
    }
}
