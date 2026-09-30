// jsdom (Vitest's default test environment) doesn't implement matchMedia --
// Taiga UI's dark-mode token reads it during provideTaiga()'s DI setup, so
// every test that touches a Taiga component needs this polyfilled first.
if (typeof window !== 'undefined' && !window.matchMedia) {
    window.matchMedia = (query: string): MediaQueryList =>
        ({
            matches: false,
            media: query,
            onchange: null,
            addListener: () => {},
            removeListener: () => {},
            addEventListener: () => {},
            removeEventListener: () => {},
            dispatchEvent: () => false,
        }) as MediaQueryList;
}
