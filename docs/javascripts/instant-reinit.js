// Render diagrams.net (`.mxgraph`) embeds and keep them working across
// Material's instant navigation.
//
// The diagrams.net viewer renders each `.mxgraph` element asynchronously: it
// fetches the diagram source from the `url` in `data-mxgraph` via XHR and
// inserts the SVG in the fetch callback. The viewer also auto-renders every
// `.mxgraph` once when its script finishes loading. With `navigation.instant`
// the page content is swapped via XHR without reloading scripts, so we must
// re-render on every navigation (`document$` emits on first load and after each
// instant navigation).
//
// If both the viewer's built-in auto-init and our own render pass run, each
// clears the container synchronously but appends its SVG later, producing a
// duplicated, vertically-stacked diagram. To avoid that we:
//   1. suppress the viewer's built-in auto-init by defining
//      `window.onDrawioViewerLoad` before the viewer script runs, so all
//      rendering goes through us, and
//   2. render each element at most once per page instance via a marker, so
//      overlapping triggers (first load + first `document$` emission) can't
//      double up.

function renderClabDiagrams() {
  if (typeof GraphViewer === "undefined" || !GraphViewer.createViewerForElement) {
    return;
  }

  document.querySelectorAll(".mxgraph").forEach(function (el) {
    if (el.dataset.clabRendered === "1") {
      return;
    }
    el.dataset.clabRendered = "1";
    el.innerHTML = "";
    GraphViewer.createViewerForElement(el);
  });
}

// Replace the viewer's built-in auto-init so it does not call
// `GraphViewer.processElements()` itself and race with us. This requires
// instant-reinit.js to load before viewer-static.min.js (see mkdocs.yml).
window.onDrawioViewerLoad = renderClabDiagrams;

// Render on first load and re-render after every instant navigation. Fresh DOM
// from a navigation has no `clabRendered` marker, so it renders again; repeated
// triggers for the same DOM are ignored.
document$.subscribe(renderClabDiagrams);
