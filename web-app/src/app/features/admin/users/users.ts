import { HttpClient, httpResource } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { longDate } from '../../../core/format';
import { AdminUser } from '../../../core/models';
import { Panel } from '../../../shared/ui';

@Component({
    selector: 'app-admin-users',
    imports: [Panel],
    template: `
        <div class="page">
            <div>
                <h2 class="page-title">Kelola User</h2>
                <p class="page-sub">Daftar akun yang boleh mengakses dashboard. Menambah email yang sudah ada akan menimpa status admin-nya.</p>
            </div>

            <form class="filter-bar" (submit)="add($event, email, isAdmin.checked)">
                <label>
                    <span>Email</span>
                    <input #email type="email" required placeholder="nama@esb.co.id" autocomplete="off" />
                </label>
                <label class="check">
                    <input #isAdmin type="checkbox" />
                    <span>Admin</span>
                </label>
                <button type="submit" class="btn" [disabled]="busy()">Tambah user</button>
            </form>
            @if (message(); as m) {
                <p class="notice">{{ m }}</p>
            }

            <app-panel title="Daftar user" [loading]="users.isLoading()" [error]="!!users.error()" [empty]="!users.value()?.length">
                <div class="tbl-wrap">
                    <table class="tbl">
                        <thead><tr><th>Email</th><th>Peran</th><th>Ditambahkan</th><th></th></tr></thead>
                        <tbody>
                            @for (u of users.value(); track u.id) {
                                <tr>
                                    <td>{{ u.email }}</td>
                                    <td><span class="badge" [class.badge-pos]="u.is_admin">{{ u.is_admin ? 'Admin' : 'Viewer' }}</span></td>
                                    <td>{{ longDate(u.created_at) }}</td>
                                    <td class="r"><button type="button" class="btn btn-danger" (click)="remove(u)">Hapus</button></td>
                                </tr>
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
    protected readonly longDate = longDate;
    protected readonly users = httpResource<AdminUser[]>(() => '/api/admin/users');
    protected readonly busy = signal(false);
    protected readonly message = signal('');

    protected add(event: Event, email: HTMLInputElement, isAdmin: boolean): void {
        event.preventDefault();
        const value = email.value.trim();
        if (!value.includes('@')) {
            this.message.set('Email tidak valid.');
            return;
        }
        this.busy.set(true);
        this.http.post('/api/admin/users', { email: value, is_admin: isAdmin }).subscribe({
            next: () => {
                this.message.set(`${value.toLowerCase()} ditambahkan.`);
                email.value = '';
                this.busy.set(false);
                this.users.reload();
            },
            error: () => {
                this.message.set('Gagal menambah user.');
                this.busy.set(false);
            },
        });
    }

    protected remove(user: AdminUser): void {
        // TODO(SSO): block removing your own access once a real session exists (see service/admin.go).
        if (!confirm(`Hapus akses ${user.email}?`)) return;
        this.http.delete(`/api/admin/users/${encodeURIComponent(user.email)}`).subscribe({
            next: () => {
                this.message.set(`${user.email} dihapus.`);
                this.users.reload();
            },
            error: () => this.message.set('Gagal menghapus user.'),
        });
    }
}
