import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideTaiga } from '@taiga-ui/core';
import { Select, SelectOption } from './select';

@Component({
    imports: [Select],
    template: `<app-select [options]="options" [(value)]="value" placeholder="Pilih…" />`,
})
class Host {
    options: SelectOption[] = [
        { value: '', label: 'Semua' },
        { value: 'a', label: 'Alpha' },
        { value: 'b', label: 'Beta' },
    ];
    value = signal('');
}

describe('Select', () => {
    const setup = async () => {
        await TestBed.configureTestingModule({ imports: [Host], providers: [provideTaiga()] }).compileComponents();
        const fixture = TestBed.createComponent(Host);
        await fixture.whenStable();
        const el = fixture.nativeElement as HTMLElement;
        const trigger = () => el.querySelector('.sel-trigger') as HTMLButtonElement;
        const flush = async () => { fixture.detectChanges(); await fixture.whenStable(); };
        return { fixture, el, trigger, flush };
    };

    it('shows the selected label and opens a listbox on click', async () => {
        const { el, trigger, flush } = await setup();
        expect(trigger().textContent).toContain('Semua');
        expect(el.querySelector('.sel-pop')).toBeNull();
        trigger().click();
        await flush();
        expect(el.querySelectorAll('.sel-opt').length).toBe(3);
    });

    it('writes the chosen value back to the bound signal and closes', async () => {
        const { fixture, el, trigger, flush } = await setup();
        trigger().click();
        await flush();
        (el.querySelectorAll('.sel-opt')[2] as HTMLElement).click();
        await flush();
        expect(fixture.componentInstance.value()).toBe('b');
        expect(trigger().textContent).toContain('Beta');
        expect(el.querySelector('.sel-pop')).toBeNull();
    });

    it('supports the keyboard: ArrowDown opens and moves, Enter picks, Escape closes', async () => {
        const { fixture, el, trigger, flush } = await setup();
        const press = async (key: string) => {
            trigger().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
            await flush();
        };
        await press('ArrowDown'); // open
        expect(el.querySelector('.sel-pop')).not.toBeNull();
        await press('ArrowDown'); // Semua -> Alpha
        await press('Enter');
        expect(fixture.componentInstance.value()).toBe('a');

        trigger().click();
        await flush();
        el.querySelector('app-select')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        await flush();
        expect(el.querySelector('.sel-pop')).toBeNull();
    });
});
