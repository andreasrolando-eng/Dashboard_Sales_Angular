import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, computed, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';

export interface Me {
    id: number;
    email: string;
    name: string;
    is_admin: boolean;
}

/** A failed sign-in, already turned into a message the form can show. */
export class LoginError extends Error {}

/**
 * The signed-in user, backed by the server session (an HttpOnly cookie the
 * browser sends by itself -- no token ever touches JavaScript).
 *
 * `user`: undefined = not asked yet, null = signed out, Me = signed in.
 */
@Injectable({ providedIn: 'root' })
export class Auth {
    private readonly http = inject(HttpClient);
    private readonly router = inject(Router);

    readonly user = signal<Me | null | undefined>(undefined);
    readonly isAdmin = computed(() => this.user()?.is_admin === true);
    readonly displayName = computed(() => this.user()?.name || this.user()?.email || '');

    private loading: Promise<Me | null> | null = null;

    /** Asks the server who we are, once; later calls reuse the answer. */
    ensureLoaded(): Promise<Me | null> {
        if (this.user() !== undefined) return Promise.resolve(this.user() ?? null);
        this.loading ??= firstValueFrom(this.http.get<Me>('/api/me'))
            .catch(() => null)
            .then((me) => {
                this.user.set(me);
                this.loading = null;
                return me;
            });
        return this.loading;
    }

    async login(email: string, password: string): Promise<Me> {
        try {
            const me = await firstValueFrom(this.http.post<Me>('/api/auth/login', { email, password }));
            this.user.set(me);
            return me;
        } catch (e) {
            throw new LoginError(loginMessage(e));
        }
    }

    async logout(): Promise<void> {
        try {
            await firstValueFrom(this.http.post('/api/auth/logout', {}));
        } catch {
            // Even if the request fails, drop the local state and go to the form.
        }
        this.user.set(null);
        await this.router.navigate(['/login']);
    }

    /** Called when any API answers 401: the session is gone. */
    sessionExpired(): void {
        if (this.user() === null) return;
        this.user.set(null);
        void this.router.navigate(['/login'], { queryParams: { returnUrl: this.router.url, expired: 1 } });
    }

    async changePassword(current: string, next: string): Promise<void> {
        try {
            await firstValueFrom(this.http.post('/api/auth/password', { current_password: current, new_password: next }));
        } catch (e) {
            throw new LoginError(apiMessage(e) ?? 'Gagal mengganti password.');
        }
    }
}

function apiMessage(e: unknown): string | null {
    const err = e as HttpErrorResponse;
    const msg = err?.error?.error;
    return typeof msg === 'string' && msg ? capitalise(msg) : null;
}

function capitalise(s: string): string {
    return s.charAt(0).toUpperCase() + s.slice(1);
}

function loginMessage(e: unknown): string {
    const err = e as HttpErrorResponse;
    if (err?.status === 401) return 'Email atau password salah.';
    if (err?.status === 429) {
        const wait = Number(err.headers?.get('Retry-After'));
        const minutes = wait > 0 ? Math.ceil(wait / 60) : 0;
        return minutes
            ? `Terlalu banyak percobaan gagal. Coba lagi dalam ${minutes} menit.`
            : 'Terlalu banyak percobaan gagal. Coba lagi beberapa menit lagi.';
    }
    if (err?.status === 400) return apiMessage(e) ?? 'Email dan password wajib diisi.';
    if (err?.status === 0) return 'Tidak bisa terhubung ke server. Periksa koneksi lalu coba lagi.';
    return 'Terjadi kesalahan di server. Coba lagi sebentar lagi.';
}
