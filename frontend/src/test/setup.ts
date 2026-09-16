// jsdom has no ResizeObserver, but floating-ui (behind Zag's popper) calls it
// whenever a positioned surface opens. Without it the open effect throws an
// unhandled rejection part-way through and the machine is left half-wired.
class ResizeObserverStub implements ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (!('ResizeObserver' in globalThis)) {
  globalThis.ResizeObserver =
    ResizeObserverStub as unknown as typeof ResizeObserver
}

// jsdom implements no scrolling, but Zag's select scrolls its list to the top
// when it opens. The missing method threw and aborted the open transition.
if (typeof Element !== 'undefined' && !Element.prototype.scrollTo) {
  Element.prototype.scrollTo = () => {}
}
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}
