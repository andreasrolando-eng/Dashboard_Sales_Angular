import { TUI_DARK_MODE, TuiIcon, TuiRoot } from '@taiga-ui/core';
import { Component, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

interface NavItem {
    label: string;
    path: string;
    icon: string;
}

const MOBILE_BREAKPOINT = 768;
const isMobile = (): boolean => typeof window !== 'undefined' && window.innerWidth < MOBILE_BREAKPOINT;

@Component({
    imports: [RouterOutlet, RouterLink, RouterLinkActive, TuiRoot, TuiIcon],
    selector: 'app-root',
    templateUrl: './app.html',
})
export class App {
    protected readonly dark = inject(TUI_DARK_MODE);
    // Desktop: sidebar expanded by default and toggles to icon-only.
    // Mobile: sidebar is an overlay drawer, closed by default.
    protected readonly expanded = signal(!isMobile());
    protected readonly drawerOpen = signal(false);

    // TODO(SSO): replace with the real session from GET /api/me.
    protected readonly userEmail = signal('andreas.rolando@esb.co.id');
    protected readonly initial = () => this.userEmail().charAt(0).toUpperCase();

    protected readonly navItems: NavItem[] = [
        { label: 'Overview', path: '/', icon: '@tui.layout-dashboard' },
        { label: 'Sales', path: '/sales', icon: '@tui.chart-column' },
        { label: 'Ops', path: '/ops', icon: '@tui.clock' },
        { label: 'Membership', path: '/membership', icon: '@tui.users' },
        { label: 'Marketing', path: '/marketing', icon: '@tui.megaphone' },
        { label: 'Kelola User', path: '/admin/users', icon: '@tui.user-cog' },
        { label: 'Sinkron Data', path: '/admin/sync', icon: '@tui.refresh-cw' },
    ];

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
}
