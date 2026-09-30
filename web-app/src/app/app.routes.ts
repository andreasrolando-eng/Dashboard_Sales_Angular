import { Routes } from '@angular/router';

export const routes: Routes = [
    { path: '', loadComponent: () => import('./features/overview/overview').then((m) => m.Overview) },
    { path: 'sales', loadComponent: () => import('./features/sales/sales').then((m) => m.Sales) },
    { path: 'ops', loadComponent: () => import('./features/ops/ops').then((m) => m.Ops) },
    { path: 'membership', loadComponent: () => import('./features/membership/membership').then((m) => m.Membership) },
    { path: 'marketing', loadComponent: () => import('./features/marketing/marketing').then((m) => m.Marketing) },
    { path: 'admin/sync', loadComponent: () => import('./features/admin/sync/sync').then((m) => m.AdminSync) },
    { path: 'admin/users', loadComponent: () => import('./features/admin/users/users').then((m) => m.AdminUsers) },
    { path: '**', redirectTo: '' },
];
