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

// jsdom implements no drag-and-drop at all — neither DataTransfer nor
// DragEvent exist — so file-drop behaviour is untestable without these.
// The shape is dictated by @zag-js/file-utils' getFileEntries, which reads
// `items` (not `files`) and expects kind/webkitGetAsEntry/getAsFile per item.
if (!('DataTransfer' in globalThis)) {
  class DataTransferStub {
    private _files: File[] = []
    dropEffect = 'none'
    effectAllowed = 'all'
    types: string[] = []

    items = {
      add: (file: File) => {
        this._files.push(file)
        this.types = ['Files']
      },
      // Array.from() over this is what getFileEntries walks.
      [Symbol.iterator]: () => this._entries()[Symbol.iterator](),
      get length() {
        return 0
      },
    } as unknown as DataTransferItemList

    private _entries() {
      return this._files.map((file) => ({
        kind: 'file' as const,
        type: file.type,
        getAsFile: () => file,
        webkitGetAsEntry: () => ({
          isFile: true,
          isDirectory: false,
          name: file.name,
        }),
      }))
    }

    get files() {
      const list = [...this._files] as File[] & { item(i: number): File | null }
      list.item = (i: number) => list[i] ?? null
      return list as unknown as FileList
    }

    getData() {
      return ''
    }
    setData() {}
  }
  globalThis.DataTransfer = DataTransferStub as unknown as typeof DataTransfer
}

if (!('DragEvent' in globalThis)) {
  class DragEventStub extends Event {
    dataTransfer: DataTransfer | null
    constructor(
      type: string,
      init: EventInit & { dataTransfer?: DataTransfer } = {},
    ) {
      super(type, init)
      this.dataTransfer = init.dataTransfer ?? null
    }
  }
  globalThis.DragEvent = DragEventStub as unknown as typeof DragEvent
}
