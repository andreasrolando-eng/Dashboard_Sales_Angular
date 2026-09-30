import { Component, inject, signal } from '@angular/core';
import { Auth, LoginError } from '../../core/auth';

const MIN_LENGTH = 8;

@Component({
    selector: 'app-account',
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Akun</h2>
                <p class="page-sub">Data akun Anda dan penggantian password.</p>
            </div>

            <section class="card account-card">
                <header class="card-head"><h3>Profil</h3></header>
                <dl class="account-info">
                    <div><dt>Email</dt><dd>{{ auth.user()?.email }}</dd></div>
                    <div>
                        <dt>Peran</dt>
                        <dd><span class="badge" [class.badge-info]="auth.isAdmin()">{{ auth.isAdmin() ? 'Admin' : 'Viewer' }}</span></dd>
                    </div>
                </dl>
            </section>

            <section class="card account-card">
                <header class="card-head"><h3>Ganti password</h3></header>
                <form class="account-form" (submit)="submit($event, current.value, next.value, again.value)">
                    <label class="field-block">
                        <span>Password saat ini</span>
                        <input #current type="password" name="current" autocomplete="current-password" required [disabled]="busy()" />
                    </label>
                    <label class="field-block">
                        <span>Password baru</span>
                        <input #next type="password" name="next" autocomplete="new-password" required [disabled]="busy()" />
                        <small>Minimal {{ minLength }} karakter. Boleh berupa kalimat.</small>
                    </label>
                    <label class="field-block">
                        <span>Ulangi password baru</span>
                        <input #again type="password" name="again" autocomplete="new-password" required [disabled]="busy()" />
                    </label>

                    @if (error(); as e) {
                        <p class="login-error" role="alert">{{ e }}</p>
                    }
                    @if (done()) {
                        <p class="notice" role="status">Password berhasil diganti. Sesi Anda di perangkat lain sudah diakhiri.</p>
                    }
                    <div>
                        <button type="submit" class="btn" [disabled]="busy()">{{ busy() ? 'Menyimpan…' : 'Simpan password' }}</button>
                    </div>
                </form>
            </section>
        </div>
    `,
})
export class Account {
    protected readonly auth = inject(Auth);
    protected readonly minLength = MIN_LENGTH;
    protected readonly busy = signal(false);
    protected readonly error = signal('');
    protected readonly done = signal(false);

    protected async submit(event: Event, current: string, next: string, again: string): Promise<void> {
        event.preventDefault();
        const form = (event.target as HTMLFormElement | null) ?? null;
        this.done.set(false);
        if (!current || !next) return this.error.set('Semua kolom wajib diisi.');
        if (next.length < MIN_LENGTH) return this.error.set(`Password baru minimal ${MIN_LENGTH} karakter.`);
        if (next !== again) return this.error.set('Pengulangan password baru tidak sama.');
        if (next === current) return this.error.set('Password baru harus berbeda dari yang lama.');

        this.busy.set(true);
        this.error.set('');
        try {
            await this.auth.changePassword(current, next);
            form?.reset();
            this.done.set(true);
        } catch (e) {
            this.error.set(e instanceof LoginError ? e.message : 'Gagal mengganti password.');
        } finally {
            this.busy.set(false);
        }
    }
}
