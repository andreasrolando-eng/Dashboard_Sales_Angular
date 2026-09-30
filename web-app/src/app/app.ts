import { TUI_DARK_MODE, TuiRoot } from '@taiga-ui/core';
import { Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';

/**
 * Root: the Taiga root, the light/dark theme wrapper (`.app` carries the
 * --app-* tokens) and the router outlet. The sidebar/top bar live in
 * `layout/Shell`, so the login page can render without them.
 */
@Component({
    imports: [RouterOutlet, TuiRoot],
    selector: 'app-root',
    template: `
        <tui-root>
            <div class="app" [attr.data-theme]="dark() ? 'dark' : 'light'">
                <router-outlet />
            </div>
        </tui-root>
    `,
})
export class App {
    protected readonly dark = inject(TUI_DARK_MODE);
}
