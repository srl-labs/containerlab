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
