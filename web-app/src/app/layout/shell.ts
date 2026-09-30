import { TUI_DARK_MODE, TuiIcon } from '@taiga-ui/core';
import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { Auth } from '../core/auth';

interface NavItem {
    label: string;
    path: string;
    icon: string;
    adminOnly?: boolean;
}

const MOBILE_BREAKPOINT = 768;
const isMobile = (): boolean => typeof window !== 'undefined' && window.innerWidth < MOBILE_BREAKPOINT;

const NAV: NavItem[] = [
    { label: 'Overview', path: '/', icon: '@tui.layout-dashboard' },
    { label: 'Sales', path: '/sales', icon: '@tui.chart-column' },
    { label: 'Ops', path: '/ops', icon: '@tui.clock' },
    { label: 'Membership', path: '/membership', icon: '@tui.users' },
    { label: 'Marketing', path: '/marketing', icon: '@tui.megaphone' },
    { label: 'Non Sales', path: '/non-sales', icon: '@tui.hand-coins' },
    { label: 'Kelola User', path: '/admin/users', icon: '@tui.user-cog', adminOnly: true },
    { label: 'Sinkron Data', path: '/admin/sync', icon: '@tui.refresh-cw', adminOnly: true },
];

/** Top bar + sidebar around every page that needs a signed-in user. */
@Component({
    selector: 'app-shell',
    imports: [RouterOutlet, RouterLink, RouterLinkActive, TuiIcon],
    templateUrl: './shell.html',
})
export class Shell {
    protected readonly auth = inject(Auth);
    protected readonly dark = inject(TUI_DARK_MODE);

    // Desktop: sidebar expanded by default and toggles to icon-only.
    // Mobile: sidebar is an overlay drawer, closed by default.
    protected readonly expanded = signal(!isMobile());
    protected readonly drawerOpen = signal(false);

    protected readonly initial = computed(() => this.auth.displayName().charAt(0).toUpperCase());
    /** Admin-only pages are hidden (and blocked by a guard + the API) for everyone else. */
    protected readonly navItems = computed(() => NAV.filter((item) => !item.adminOnly || this.auth.isAdmin()));

    protected toggleMenu(): void {
        if (isMobile()) this.drawerOpen.update((v) => !v);
        else this.expanded.update((v) => !v);
    }

    protected closeDrawer(): void {
        this.drawerOpen.set(false);
    }

    protected toggleTheme(): void {
        this.dark.set(!this.dark());
    }

    protected logout(): void {
        void this.auth.logout();
    }
}
