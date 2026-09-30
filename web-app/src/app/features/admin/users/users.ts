import { HttpClient, httpResource } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { Auth } from '../../../core/auth';
import { longDate, wibTime } from '../../../core/format';
import { AdminUser } from '../../../core/models';
import { Panel } from '../../../shared/ui';

const MIN_PASSWORD = 8;

@Component({
    selector: 'app-admin-users',
    imports: [Panel],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Kelola User</h2>
                <p class="page-sub">
                    Akun yang boleh mengakses dashboard. Isi password agar user bisa masuk lewat halaman login.
                    Menambah email yang sudah ada akan menimpa status admin-nya.
                </p>
            </div>

            <form class="filter-bar" (submit)="add($event, email, password, isAdmin.checked)">
                <label>
                    <span>Email</span>
                    <input #email type="email" required placeholder="nama@esb.co.id" autocomplete="off" />
                </label>
                <label>
                    <span>Password awal (opsional)</span>
                    <input #password type="text" placeholder="min. {{ minPassword }} karakter" autocomplete="off" spellcheck="false" />
                </label>
                <label class="check">
                    <input #isAdmin type="checkbox" />
                    <span>Admin</span>
                </label>
                <button type="submit" class="btn" [disabled]="busy()">Tambah user</button>
            </form>
            @if (message(); as m) {
                <p class="notice" role="status">{{ m }}</p>
            }

            <app-panel title="Daftar user" [loading]="users.isLoading()" [error]="!!users.error()" [empty]="!users.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead>
                            <tr><th>Email</th><th>Peran</th><th>Password</th><th>Login terakhir</th><th>Ditambahkan</th><th></th></tr>
                        </thead>
                        <tbody>
                            @for (u of users.value(); track u.id) {
                                <tr>
                                    <td>
                                        {{ u.email }}
                                        @if (isSelf(u)) {
                                            <span class="badge badge-info self-badge">Anda</span>
                                        }
                                    </td>
                                    <td><span class="badge" [class.badge-pos]="u.is_admin">{{ u.is_admin ? 'Admin' : 'Viewer' }}</span></td>
                                    <td>
                                        <span class="badge" [class.badge-pos]="u.has_password" [class.badge-warn]="!u.has_password">
                                            {{ u.has_password ? 'Sudah diatur' : 'Belum diatur' }}
                                        </span>
                                    </td>
                                    <td>{{ u.last_login_at ? wibTime(u.last_login_at) : '–' }}</td>
                                    <td>{{ longDate(u.created_at) }}</td>
                                    <td class="r">
                                        <button type="button" class="btn btn-ghost" (click)="startEdit(u)">Atur password</button>
                                        @if (!isSelf(u)) {
                                            <button type="button" class="btn btn-danger" (click)="remove(u)">Hapus</button>
                                        }
                                    </td>
                                </tr>
                                @if (editing() === u.email) {
                                    <tr class="edit-row">
                                        <td colspan="6">
                                            <form class="inline-form" (submit)="savePassword($event, u, pw.value)">
                                                <label class="sr-only" [attr.for]="'pw-' + u.id">Password baru untuk {{ u.email }}</label>
                                                <input #pw [id]="'pw-' + u.id" type="text" autocomplete="off" spellcheck="false"
                                                       placeholder="Password baru (min. {{ minPassword }} karakter)" />
                                                <button type="submit" class="btn" [disabled]="busy()">Simpan</button>
                                                <button type="button" class="btn btn-ghost" (click)="editing.set(null)">Batal</button>
                                                <small class="muted">Semua sesi login user ini akan diakhiri.</small>
                                            </form>
                                        </td>
                                    </tr>
                                }
                            }
                        </tbody>
                    </table>
                </div>
            </app-panel>
        </div>
    `,
})
export class AdminUsers {
    private readonly http = inject(HttpClient);
    private readonly auth = inject(Auth);

    protected readonly longDate = longDate;
    protected readonly wibTime = wibTime;
    protected readonly minPassword = MIN_PASSWORD;

    protected readonly users = httpResource<AdminUser[]>(() => '/api/admin/users');
    protected readonly busy = signal(false);
    protected readonly message = signal('');
    protected readonly editing = signal<string | null>(null);
    private readonly myEmail = computed(() => this.auth.user()?.email ?? '');

    protected isSelf(u: AdminUser): boolean {
        return u.email.toLowerCase() === this.myEmail().toLowerCase();
    }

    protected add(event: Event, email: HTMLInputElement, password: HTMLInputElement, isAdmin: boolean): void {
        event.preventDefault();
        const value = email.value.trim();
        if (!value.includes('@')) {
            this.message.set('Email tidak valid.');
            return;
        }
        const pw = password.value;
        if (pw && pw.length < MIN_PASSWORD) {
            this.message.set(`Password minimal ${MIN_PASSWORD} karakter.`);
            return;
        }
        this.busy.set(true);
        const body: Record<string, unknown> = { email: value, is_admin: isAdmin };
        if (pw) body['password'] = pw;
        this.http.post('/api/admin/users', body).subscribe({
            next: () => {
                this.message.set(`${value.toLowerCase()} ditambahkan${pw ? ' dengan password' : ' (belum bisa login sampai password diatur)'}.`);
                email.value = '';
                password.value = '';
                this.busy.set(false);
                this.users.reload();
            },
            error: (e) => {
                this.message.set(apiError(e, 'Gagal menambah user.'));
                this.busy.set(false);
            },
        });
    }

    protected startEdit(u: AdminUser): void {
        this.editing.set(this.editing() === u.email ? null : u.email);
    }

    protected savePassword(event: Event, u: AdminUser, password: string): void {
        event.preventDefault();
        if (password.length < MIN_PASSWORD) {
            this.message.set(`Password minimal ${MIN_PASSWORD} karakter.`);
            return;
        }
        this.busy.set(true);
        this.http.post(`/api/admin/users/${encodeURIComponent(u.email)}/password`, { password }).subscribe({
            next: () => {
                this.message.set(`Password ${u.email} diatur. Sesi login lamanya diakhiri.`);
                this.editing.set(null);
                this.busy.set(false);
                this.users.reload();
            },
            error: (e) => {
                this.message.set(apiError(e, 'Gagal mengatur password.'));
                this.busy.set(false);
            },
        });
    }

    protected remove(user: AdminUser): void {
        if (!confirm(`Hapus akses ${user.email}?`)) return;
        this.http.delete(`/api/admin/users/${encodeURIComponent(user.email)}`).subscribe({
            next: () => {
                this.message.set(`${user.email} dihapus.`);
                this.users.reload();
            },
            error: (e) => this.message.set(apiError(e, 'Gagal menghapus user.')),
        });
    }
}

function apiError(e: { error?: { error?: string } }, fallback: string): string {
    const msg = e?.error?.error;
    return typeof msg === 'string' && msg ? msg.charAt(0).toUpperCase() + msg.slice(1) + '.' : fallback;
}
