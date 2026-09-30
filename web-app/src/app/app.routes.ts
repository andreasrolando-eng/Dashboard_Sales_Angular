import { Routes } from '@angular/router';
import { adminGuard, authGuard, guestGuard } from './core/auth.guards';

export const routes: Routes = [
    // Public: the sign-in form (renders without the sidebar/top bar).
    { path: 'login', canActivate: [guestGuard], loadComponent: () => import('./features/login/login').then((m) => m.Login) },

    // Everything else needs a signed-in user and lives inside the Shell.
    {
        path: '',
        canActivate: [authGuard],
        loadComponent: () => import('./layout/shell').then((m) => m.Shell),
        children: [
            { path: '', loadComponent: () => import('./features/overview/overview').then((m) => m.Overview) },
            { path: 'sales', loadComponent: () => import('./features/sales/sales').then((m) => m.Sales) },
            { path: 'ops', loadComponent: () => import('./features/ops/ops').then((m) => m.Ops) },
            { path: 'membership', loadComponent: () => import('./features/membership/membership').then((m) => m.Membership) },
            { path: 'marketing', loadComponent: () => import('./features/marketing/marketing').then((m) => m.Marketing) },
            { path: 'non-sales', loadComponent: () => import('./features/non-sales/non-sales').then((m) => m.NonSales) },
            { path: 'account', loadComponent: () => import('./features/account/account').then((m) => m.Account) },
            {
                path: 'admin',
                canActivate: [adminGuard],
                children: [
                    { path: 'users', loadComponent: () => import('./features/admin/users/users').then((m) => m.AdminUsers) },
                    { path: 'sync', loadComponent: () => import('./features/admin/sync/sync').then((m) => m.AdminSync) },
                ],
            },
        ],
    },

    { path: '**', redirectTo: '' },
];
