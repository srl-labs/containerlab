# graph command

### Description

The `graph` command generates graphical representations of a topology.

Two graphing options are available:

* an HTML page served by `containerlab` web-server based on a user-provided HTML template and static files.
* a [graph description file in dot format](https://en.wikipedia.org/wiki/DOT_(graph_description_language)) that can be rendered using [Graphviz](https://graphviz.org/) or viewed [online](https://dreampuf.github.io/GraphvizOnline/).[^1]

#### HTML

The HTML-based graph representation is the default graphing option. The topology will be graphed and served online using the embedded web server.

The default graph template is based on the [NeXt UI](https://developer.cisco.com/site/neXt/) framework[^2].

![animation](https://user-images.githubusercontent.com/11521160/155654224-d46b346d-7051-49f8-ba93-6dee6d22a39f.gif)

To render a topology using this default graph engine:

```
containerlab graph -t <path/to/topo.clab.yml>
```

##### NeXt UI

Topology graph created with NeXt UI has some control elements that allow you to choose the color theme of the web view, scaling and panning. Besides these generic controls it is possible to enable auto-layout of the components using buttons at the top of the screen.

###### Layout and sorting

The graph engine can automatically pan and sort elements in your topology based on their _role_. We encode the role via `group` property of a node.

Today we have the following sort orders available to users:

```yaml
sortOrder: ['10', '9', 'superspine', '8', 'dc-gw', '7', '6', 'spine', '5', '4', 'leaf', 'border-leaf', '3', 'server', '2', '1'],
```

The values are sorted so that `10` is placed higher in the hierarchy than `9` and so on.

Consider the following snippet:

```yaml
topology:
  nodes:
    ### SPINES ###
    spine1:
      group: spine
    
    ### LEAFS ###
    leaf1:
      group: leaf

    ### CLIENTS ###
    client1:
      kind: linux
      group: server
```

The `group` property set to the predefined value will automatically auto-align the elements based on their role.

#### Graphviz

When `graph` command is called without the `--srv` flag, containerlab will generate a [graph description file in dot format](https://en.wikipedia.org/wiki/DOT_(graph_description_language)).

The dot file can be used to view the graphical representation of the topology either by rendering the dot file into a PNG file or using [online dot viewer](https://dreampuf.github.io/GraphvizOnline/).


#### Mermaid

When `graph` command is called with the `--mermaid` flag, containerlab will generate a graph description file in [Mermaid graph format](https://mermaid.js.org/syntax/flowchart.html). This is useful for embedding generated graph text to Markdown. Some Markdown renderer like GitHub or Notion supports rendering the Mermaid graph in the code block. When you are not satisfying the rendering result, you can import the generated text into [draw.io](https://draw.io) and edit it.

### Online vs offline graphing

When HTML graph option is used, containerlab will try to build the topology graph by inspecting the running containers which are part of the lab. This essentially means, that the lab must be running. Although this method provides some additional details (like IP addresses), it is not always convenient to run a lab to see its graph.

The other option is to use the topology file solely to build the graph. This is done by adding `--offline` flag.

If `--offline` flag was not provided and no containers were found matching the lab name, containerlab will use the topo file only (as if offline mode was set).

### Usage

`containerlab [global-flags] graph [local-flags]`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### srv

The `--srv` flag allows a user to customize the HTTP address and port for the web server. Default value is `:50080`.

A single path `/` is served, where the graph is generated based on either a default template or on the template supplied using `--template`.

#### template

The `--template` flag allows to customize the HTML based graph by supplying a user defined template that will be rendered and exposed on the address specified by `--srv`.

#### static-dir

The `--static-dir` flag enables the embedded HTML web-server to serve static files from the specified directory. Must be used together with the `--template` flag.

With this flag, it is possible to link to local files (JS, CSS, fonts, etc.) from the custom HTML template.

#### dot

With `--dot` flag provided containerlab will generate the `dot` file instead of serving the topology with embedded HTTP server.

#### mermaid

With `--mermaid` flag provided containerlab will generate the `mermaid` file instead of serving the topology with embedded HTTP server.

#### mermaid-direction

With `--mermaid-direction` flag provided with `--mermaid` flag, containerlab adjusts [direction](https://mermaid.js.org/syntax/flowchart.html#direction) of the generated graph. Accepted values are TB, TD, BT, RL, and LR.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `graph` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only graph selected nodes and their links.

### Examples

#### Render graph of topology on HTML server

This will render the running lab if the lab is running and the topology file if it isn't. Default options will be used (HTML server running on port `50080`).

```bash
containerlab graph -t /path/to/topo1.clab.yml
```

#### Render graph on specified http server port

```bash
containerlab graph --topo /path/to/topo1.clab.yml --srv ":3002"
```

#### Render graph using a custom html template

```bash
containerlab graph --topo /path/to/topo1.clab.yml --template my_template.html
```

#### Render graph using a custom template that links to local files

The HTML server will use a custom template that links to local files located at /path/to/static_files directory

```bash
containerlab graph --topo /path/to/topo1.clab.yml --template my_template.html --static-dir /path/to/static_files
```

[^1]: This method is prone to errors when node names contain dashes and special symbols. Use with caution, and prefer the HTML server alternative.
[^2]: NeXt UI css/js files can be found at `/etc/containerlab/templates/graph/nextui` directory
