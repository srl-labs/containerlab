(function (nx) {

    data = JSON.parse(data)
    var activeLayout = ''
    var defaultIconType = 'router'

    // when group property is not set in containerlab
    // we set it to N/A to indicate a missing group assignment
    for (var key in data.nodes) {
        if (!("group" in data.nodes[key])) {
            data.nodes[key]["group"] = 'N/A';
        }
    }
    // for non container nodes, no image filed is set
    // we set it to N/A to indicate no image property
    // without this, the nx graph node tooltip image value 
    // will be skipped, cause the filed name and value mismatch
    for (var key in data.nodes) {
        if (!("image" in data.nodes[key])) {
            data.nodes[key]["image"] = 'N/A';
        }
    }

    nx.define('CustomLinkLabel', nx.graphic.Topology.Link, {
        properties: {
            sourcelabel: 'null',
            targetlabel: 'null',
        },
        view: function (view) {
            view.content.push({
                name: 'sourceBadge',
                type: 'nx.graphic.Group',
                content: [
                    {
                        name: 'sourceBg',
                        type: 'nx.graphic.Rect',
                        props: {
                            'class': 'link-set-circle',
                            height: 1
                        }
                    },
                    {
                        name: 'sourceText',
                        type: 'nx.graphic.Text',
                        props: {
                            'class': 'link-set-text',
                            y: 1
                        }
                    }
                ],
                props: {
                    'alignment-baseline': 'after-edge',
                }
            },
                {
                    name: 'targetBadge',
                    type: 'nx.graphic.Group',
                    content: [
                        {
                            name: 'targetBg',
                            type: 'nx.graphic.Rect',
                            props: {
                                'class': 'link-set-circle',
                                height: 1
                            }
                        },
                        {
                            name: 'targetText',
                            type: 'nx.graphic.Text',
                            props: {
                                'class': 'link-set-text',
                                y: 1
                            }
                        }
                    ],
                    props: {
                        'alignment-baseline': 'after-edge',
                    }
                }

            );
            return view;
        },
        methods: {
            init: function (args) {
                this.inherited(args);
                this.topology().fit();
            },
            'setModel': function (model) {
                this.inherited(model);
            },
            update: function () {
                this.inherited();
                var line = this.line();
                var angle = line.angle();
                var stageScale = this.stageScale();
                line = line.pad(50 * stageScale, 50 * stageScale);
                if (this.sourcelabel()) {
                    var sourceBadge = this.view('sourceBadge');
                    var sourceText = this.view('sourceText');
                    var sourceBg = this.view('sourceBg');
                    var point;
                    sourceText.sets({
                        text: this.sourcelabel(),
                    });
                    var sourceTextBound = sourceText.getBound()
                    sourceBg.sets({ width: sourceTextBound.width, visible: true });
                    sourceBg.setTransform(sourceTextBound.width / -2);
                    point = line.start;
                    if (stageScale) {
                        sourceBadge.set('transform', 'translate(' + point.x + ',' + point.y + ') ' + 'scale (' + stageScale + ') ');
                    } else {
                        sourceBadge.set('transform', 'translate(' + point.x + ',' + point.y + ') ');
                    }
                }
                if (this.targetlabel()) {
                    var targetBadge = this.view('targetBadge');
                    var targetText = this.view('targetText');
                    var targetBg = this.view('targetBg');
                    var point;
                    targetText.sets({
                        text: this.targetlabel(),
                    });
                    var targetTextBound = targetText.getBound()
                    targetBg.sets({ width: targetTextBound.width, visible: true });
                    targetBg.setTransform(targetTextBound.width / -2);
                    point = line.end;
                    if (stageScale) {
                        targetBadge.set('transform', 'translate(' + point.x + ',' + point.y + ') ' + 'scale (' + stageScale + ') ');
                    } else {
                        targetBadge.set('transform', 'translate(' + point.x + ',' + point.y + ') ');
                    }
                }
                this.view("sourceBadge").visible(true);
                this.view("sourceBg").visible(true);
                this.view("sourceText").visible(true);
                this.view("targetBadge").visible(true);
                this.view("targetBg").visible(true);
                this.view("targetText").visible(true);
            }
        }
    });

    nx.define('CustomNodeTooltip', nx.ui.Component, {
        properties: {
            node: {},
            topology: {}
        },
        view: {
            tag: 'div',
            content: [
                {
                    tag: 'div',
                    content: '{#node.model.name}',
                    props: { "class": "font-bold text-black text-center uppercase border-b pb-2" },
                },
                {
                    tag: 'div',
                    content: [
                        {
                            tag: 'div',
                            content: [
                                {
                                    tag: 'label',
                                    content: 'Image: ',
                                    props: { "class": "font-semibold text-black pt-2" },
                                },
                                {
                                    tag: 'label',
                                    content: 'Kind: ',
                                    props: { "class": "font-semibold text-black" },
                                },
                                {
                                    tag: 'label',
                                    content: 'Group: ',
                                    props: { "class": "font-semibold text-black" },
                                },
                                {
                                    tag: 'label',
                                    content: 'State: ',
                                    props: { "class": "font-semibold text-black" },
                                },
                                {
                                    tag: 'label',
                                    content: 'IPv4: ',
                                    props: { "class": "font-semibold text-black" },
                                },
                                {
                                    tag: 'label',
                                    content: 'IPv6: ',
                                    props: { "class": "font-semibold text-black" },
                                },
                            ],
                            props: { "class": "flex flex-col pr-3" },
                        },
                        {
                            tag: 'div',
                            content: [
                                {
                                    tag: 'span',
                                    content: '{#node.model.image}',
                                    props: { "class": "font-normal text-black pt-2 inline-table" },
                                },
                                {
                                    tag: 'span',
                                    content: '{#node.model.kind}',
                                    props: { "class": "font-normal text-black" },
                                },
                                {
                                    tag: 'span',
                                    content: '{#node.model.group}',
                                    props: { "class": "font-normal text-black" },
                                },
                                {
                                    tag: 'span',
                                    content: '{#node.model.state}',
                                    props: { "class": "font-normal text-black" },
                                },
                                {
                                    tag: 'span',
                                    content: '{#node.model.ipv4_address}',
                                    props: { "class": "font-normal text-black" },
                                },
                                {
                                    tag: 'span',
                                    content: '{#node.model.ipv6_address}',
                                    props: { "class": "font-normal text-black" },
                                },
                            ],
                            props: { "class": "flex flex-col" },
                        },
                    ],
                    props: { "class": "inline-flex" },
                },
            ],
            props: { "class": "bg-white text-sm whitespace-nowrap" }
        },
    });

    var topo = new nx.graphic.Topology({
        showIcon: true,
        adaptive: true,
        nodeConfig: {
            label: 'model.name',
            iconType: function (model) {
                if (model._data.group === 'N/A') {
                    return defaultIconType
                }
                else {
                    return model._data.group
                }
            },
        },
        linkConfig: {
            linkType: 'curve',
            width: 2,
            sourcelabel: 'model.source_endpoint',
            targetlabel: 'model.target_endpoint',
            color: '#DBEAFE',
        },
        identityKey: 'name',
        dataProcessor: 'force',
        // sort order consists of typical Clos hierarchy levels mixed with numerical values to help achieve auto sorting on arbitrary topologies
        layoutConfig: {
            sortOrder: ['10', '9', 'superspine', '8', 'dc-gw', '7', '6', 'spine', '5', '4', 'leaf', 'border-leaf', '3', 'server', '2', '1'],
        },
        enableSmartLabel: true,
        enableSmartNode: true,
        enableGradualScaling: true,
        autoLayout: true,
        linkInstanceClass: 'CustomLinkLabel',
        supportMultipleLink: true,
        tooltipManagerConfig: {
            nodeTooltipContentClass: 'CustomNodeTooltip',
        },
    });

    topo.on('ready', function () {
        topo.data(data);
    });

    adaptToContainer = function () {
        topo.adaptToContainer();
    };

    horizontal = function () {
        if (activeLayout === 'horizontal') {
            return;
        };
        document.getElementById("v-btn").classList.remove("text-white", "bg-[#0386d2]");
        document.getElementById("h-btn").classList.add("text-white", "bg-[#0386d2]");
        activeLayout = 'horizontal';
        var layout = topo.getLayout('hierarchicalLayout');
        layout.direction('horizontal');
        layout.levelBy(function (node, model) {
            return model.get('group');
        });
        topo.activateLayout('hierarchicalLayout');
    }

    vertical = function () {
        if (activeLayout === 'vertical') {
            return;
        };
        document.getElementById("h-btn").classList.remove("text-white", "bg-[#0386d2]");
        document.getElementById("v-btn").classList.add("text-white", "bg-[#0386d2]");
        activeLayout = 'vertical';
        var layout = topo.getLayout('hierarchicalLayout');
        layout.direction('vertical');
        layout.levelBy(function (node, model) {
            return model.get('group');
        });
        topo.activateLayout('hierarchicalLayout');
    }

    window.onresize = adaptToContainer;
    var app = new nx.ui.Application();
    app.container(document.getElementById('clab-topology'));
    topo.attach(app);

})(nx);
