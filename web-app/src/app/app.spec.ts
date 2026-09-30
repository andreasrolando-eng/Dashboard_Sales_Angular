import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { App } from './app';
import { routes } from './app.routes';

describe('App', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [App],
            providers: [provideRouter(routes), provideTaiga()],
        }).compileComponents();
    });

    it('should create the app', () => {
        const fixture = TestBed.createComponent(App);
        const app = fixture.componentInstance;
        expect(app).toBeTruthy();
    });

    it('should render the app name in the top bar', async () => {
        const fixture = TestBed.createComponent(App);
        await fixture.whenStable();
        const compiled = fixture.nativeElement as HTMLElement;
        expect(compiled.textContent).toContain('Dashboard Sales');
    });

    it('toggles the theme and the sidebar', async () => {
        const fixture = TestBed.createComponent(App);
        await fixture.whenStable();
        const el = fixture.nativeElement as HTMLElement;
        const app = el.querySelector('.app') as HTMLElement;
        const before = app.dataset['theme'];

        (el.querySelector('[aria-label="Mode gelap"], [aria-label="Mode terang"]') as HTMLButtonElement).click();
        fixture.detectChanges();
        expect(app.dataset['theme']).not.toBe(before);

        (el.querySelector('[aria-label="Buka/tutup menu"]') as HTMLButtonElement).click();
        fixture.detectChanges();
        expect(app.classList.contains('collapsed')).toBe(true);
    });

    it('should render every nav item', async () => {
        const fixture = TestBed.createComponent(App);
        await fixture.whenStable();
        const compiled = fixture.nativeElement as HTMLElement;
        for (const label of ['Overview', 'Sales', 'Ops', 'Membership', 'Marketing', 'Kelola User']) {
            expect(compiled.textContent).toContain(label);
        }
    });
});
