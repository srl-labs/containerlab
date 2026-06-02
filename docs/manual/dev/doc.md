# Containerlab Documentation

Contributing documentation is as valuable as contributing code; it is also a great way to start contributing to the project.  
The containerlab documentation is part of the same repo that contains the code.

We tried to make it as easy as possible to contribute to the documentation. Starting from small edits that can be solely done in the browser, to more complex and thorough changes with a live dev server running on your machine to control the development process.

## Online editing

If you found a typo, or want to add a little piece of documentation you can do this all in your browser! On each documentation page you will find an "Edit this page" icon in the top right corner. Clicking on it will take you to the markdown file in the GitHub repository where you can make your changes. Once you are done, you can submit a pull request with your changes.

Sometimes you might want to have a change that spans more than one file, in that case you use GitHub's online VS Code experience and opening the repo in your browser by following the [**`github.dev/srl-labs/containerlab`**](https://github.dev/srl-labs/containerlab) link.

## Offline editing

While online editing makes it easy to make small changes, it doesn't offer you a preview of the changes you're making, and this might be a bit cumbersome for larger changes. For this reason, we recommend setting up a local development environment to preview your changes when you feel like your changes are more substantial than a typo fix.

To setup the dev environment you have to have Docker installed, which is a requirement for containerlab anyway. Once you have Docker installed, you can run the following command to start the development server:

1. Fork the [srl-labs/containerlab](https://github.com/srl-labs/containerlab) repo
2. Clone your fork of the repo locally
3. Change to the local repo top level directory - `cd containerlab`
4. Run `make serve-docs-full PUBLIC=yes`

You can access the local website content from your browser at http://localhost:8001

Look at the `nav` key in the [`mkdocs.yml`](https://github.com/srl-labs/containerlab/blob/main/mkdocs.yml) file to identify the markdown file that corresponds to the page you want to edit.

Any new content page should be added as a markdown file at a suitable location in the `docs` hierarchy and added under the `nav` key in `mkdocs.yml` to be reflected in the website documentation.

Consult the [mkdocs](https://www.mkdocs.org/) and [mkdocs-material](https://squidfunk.github.io/mkdocs-material/) for more information.

Once the documentation changes are complete, commit the changes and raise a pull request.

## Diagrams

We prefer scalable vector diagrams for crisp sharp images checked into the repository. But you probably already noticed that.

Containerlab drawings are made using the free online diagramming tool called [diagrams.net](https://diagrams.net). If you have multiple diagrams, create multiple pages in your diagram file - one per diagram.

Diagrams are stored in the [diagrams :octicons-link-external-16:](https://github.com/srl-labs/containerlab/tree/diagrams){:target="\_blank"} branch, not the `main` branch. Create a fork of the `diagrams` branch to commit your draw.io file and raise a pull request targeting the `srl-labs/containerlab:diagrams` branch.

Once your diagram PR is merged, you can embed it in any markdown document using the below markup -

```html
<div class='mxgraph'
  style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;'
  data-mxgraph='{"page":0,"zoom":1,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank",
      "url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/YOUR-DIAGRAM.drawio"}'>
</div>
```

Replace `YOUR-DIAGRAM` with the name of your diagram file in the above markup. If your file has multiple pages, you can specify the required page number in the above markup. Each diagram will need a markup like the above. You can add multiple markups for different diagrams in the same markdown file.

You MUST also add the below HTML markup at the bottom of your markdown file so that the diagrams are viewable -

```html
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
```
