# Containerlab Documentation

The containerlab website and documentation is part of the same monorepo as the code.

To edit or add to the Containerlab website and manual make sure you have docker installed and then follow these steps -

1. Fork the [srl-labs/containerlab](https://github.com/srl-labs/containerlab) repo
2. Clone your fork of the repo locally
3. Change to the local repo top level directory - `cd containerlab`
4. Run `make serve-docs-full PUBLIC=yes`

You can access the local website content from your browser at http://127.0.0.1:8001

Look at the `nav` key in the `mkdocs.yml` file  to identify the markdown file that corresponds to the page you want to edit.

Any new content page should be added as a markdown file at a suitable location in the `docs` hierarchy and added under the `nav` key in `mkdocs.yml` to be reflected in the website documentation.

Consult the [mkdocs](https://www.mkdocs.org/) and [mkdocs-material](https://squidfunk.github.io/mkdocs-material/) for more information.

Once the documentation changes are complete, commit the changes and raise a pull request.

## Diagrams

We prefer scalable vector diagrams for crisp sharp images.

Use [diagrams.net](https://diagrams.net) to create a draw.io vector diagram. If you have multiple diagrams, create multiple pages in your diagram file - one per diagram.

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
