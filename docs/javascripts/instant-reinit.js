// Re-initialize third-party embeds after instant navigation.
//
// With `navigation.instant` enabled, Zensical/Material swaps page content via
// XHR instead of a full reload, so libraries that scan the DOM only on initial
// load must be re-run on every navigation. `document$` emits once on first load
// and again after each instant navigation.
//
// glightbox and mermaid subscribe to `document$` themselves, so only the
// diagrams.net viewer (used by the `.mxgraph` diagrams on the home page) needs
// to be wired up here.
document$.subscribe(function () {
  if (typeof GraphViewer === "undefined" || !GraphViewer.processElements) {
    return;
  }

  // Clear any previously rendered graph first; the source XML lives in the
  // `data-mxgraph` attribute, so re-processing rebuilds it without duplicating.
  document.querySelectorAll(".mxgraph").forEach(function (el) {
    el.innerHTML = "";
  });

  GraphViewer.processElements();
});
