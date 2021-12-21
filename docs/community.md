Containerlab openness and focus on multivendor labs was a key to its success and adoption. With more than a dozen Network Operating Systems spread across several networking vendors and opensource teams, it is a tool that can answer the needs of a broad network engineers community.

## Discord Server
Growing the number of supported NOSes is a task that can't be done by a single person, and there the community role is adamant. To support and cherish the growing containerlab community and provide better feedback and discussions platform, we launched containerlab's own Discord server.

Everybody is welcome to join and chat with our community members about all things containerlab!

<center>[:fontawesome-brands-discord: Join Containerlab Discord Server](https://discord.gg/vAyddtaEV9){ .md-button .md-button--primary }</center>

## In The Media
We are always happy to showcase containerlab and demonstrate its powers. Luckily, the network engineering community has lots of events worldwide, and we participated in some. Below you will find recordings of containerlab talks in different formats and on various venues listed in reverse chronological order[^1].

### Packet Pushers Tech Bytes
<small>:material-podcast: [Containerlab Makes Container And VM Networking Labs Easy](https://packetpushers.net/podcast/tech-bytes-containerlab-makes-container-and-vm-networking-labs-easy-sponsored/) · :material-calendar: 2021-11-15</small>

A short, 14 minutes long introductory talk about Containerlab. If you wanted to know what containerlab is, but all you have is 15 minutes break - go check it out.

<div class="iframe-audio2-container">
<iframe width="320" height="30" src="https://packetpushers.net/?powerpress_embed=52038-podcast&amp;powerpress_player=mediaelement-audio" frameborder="0" scrolling="no"></iframe>
</div>

Participants: [:material-twitter:][rdodin-twitter][:material-linkedin:][rdodin-linkedin] Roman Dodin

### NANOG 83
<small>:material-youtube: [Containerlab - running networking labs with Docker UX](https://www.youtube.com/watch?v=qigCla1qY3k) · :material-calendar: 2021-11-03</small>

Our very first NANOG appearance and we went full-steam there. This talk is the most comprehensive containerlab tutorial captured to that date. It starts with the basics and escalates to the [advanced DC fabric deployment](https://youtu.be/qigCla1qY3k?t=2131) with HA telemetry cluster created. All driven by a single containerlab topology file.

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/qigCla1qY3k" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

Participants:

* [:material-twitter:][rdodin-twitter][:material-linkedin:][rdodin-linkedin] Roman Dodin
* [:material-linkedin:][karim-linkedin] Karim Radhouani

### Open Networking & Edge Summit 2021
<small>:material-youtube: [Containerlab - a Modern way to Deploy Networking Topologies for Labs, CI, and Testing](https://www.youtube.com/watch?v=snQTlFahY1c) · :material-calendar: 2021-10-11</small>

This 30mins screencast introduces containerlab by going through a multivendor lab example consisting of Nokia SR Linux, Arista cEOS, and GoBGP containers participating in a route reflection scenario.

The talk starts with the reasoning as to why containerlab development was warranted and what features of container-based labs we wanted to have. Then we start building the lab step by step, explaining how the containerlab topology file is structured.

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/snQTlFahY1c" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

Participants: [:material-twitter:][rdodin-twitter][:material-linkedin:][rdodin-linkedin] Roman Dodin

### NLNOG 2021
<small>:material-youtube: [Running networking labs with Docker User Experience](https://www.youtube.com/watch?v=n81Tc1g4W5U) · :material-calendar: 2021-09-05</small>

The first public talk around containerlab happened in the Netherlands at an in-person (sic!) networking event NLNOG 2021.

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/n81Tc1g4W5U" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

Participants: [:material-twitter:][rdodin-twitter][:material-linkedin:][rdodin-linkedin] Roman Dodin

### Modem Podcast s01e10
<small>:material-youtube: [Containerlab: Declarative network labbing](https://www.modem.show/post/s01e10/) · :material-calendar: 2021-06-06</small>

Building large-scale network labs can be tedious and error-prone — More importantly, they can be notoriously hard to spin up automatically. Containerlab is a new tool that promises to “redefine the way you run networking labs”, and I really think it hits that target. On this episode of the Modulate Demodulate podcast, Nick and Chris C. are joined by Roman Dodin, one of the brains behind Containerlab.

<div class="iframe-audio-container">
<iframe src="https://anchor.fm/modulate-demodulate/embed/episodes/Containerlab-Declarative-Network-Labbing-with-Roman-Dodin-e129cuc/a-a5q33b1" height="102px" width="400px" frameborder="0" scrolling="no"></iframe>
</div>

Participants: [:material-twitter:][rdodin-twitter][:material-linkedin:][rdodin-linkedin] Roman Dodin

## Blogs
The power of the community is in its members. We are delighted to have containerlab users who share their experience working with the tool, unveiling new use cases, and providing a personal touch to the workflows.

This section logs the most notable blogs, streams, and and demos delivered by containerlab users worldwide.

### Network Modeling: Automating Mikrotik RouterOS CHR Containerlab images
<small>:material-text: [Blog](https://stubarea51.net/2021/12/20/network-modeling-automating-mikrotik-routeros-chr-containerlab-images/) by [@nlgotz](https://twitter.com/nlgotz) · :material-calendar: 2021-12-20</small>

A [blogpost](https://stubarea51.net/2021/12/20/network-modeling-automating-mikrotik-routeros-chr-containerlab-images/) showing how to build containerlab images for the Mikrotik CHR, and how you can avoid having to build them yourself using Docker.

Discussions: [:material-twitter:](https://twitter.com/nlgotz/status/1472941962345033728)

### My Journey and Experience with Containerlab
<small>:material-text: [Blog](https://juliopdx.com/2021/12/10/my-journey-and-experience-with-containerlab/) by [@JulioPDX](https://twitter.com/Julio_PDX) · :material-calendar: 2021-12-10</small>

In [this blog](https://juliopdx.com/2021/12/10/my-journey-and-experience-with-containerlab/) Julio took containerlab for a spin and shares his experience with it. His lab consists of a few Arista cEOS nodes which he then provisions with Nornir, using [Ansible inventory](manual/inventory.md) generated by containerlab.

Discussions: [:material-twitter:](https://twitter.com/Julio_PDX/status/1469562531689631745) · [:material-linkedin:](https://www.linkedin.com/feed/update/urn:li:activity:6875328740948344832/)

### Network Modeling: Segmented Lab access with Containerlab and ZeroTier
<small>:material-text: [Blog](https://stubarea51.net/2021/11/23/network-modeling-segmented-lab-access-with-containerlab-and-zerotier/) by [@nlgotz](https://twitter.com/nlgotz) · :material-calendar: 2021-11-23</small>

When building out network labs, often multiple people will need access to the lab. The main way right now is to use something like EVE-NG or GNS3 to provide access.

There are 2 downsides to this method. The first is that your server is exposed to the internet and if your usernames/passwords aren’t strong enough, your server can become compromised. The second is that sometimes you may not want everyone to be able to add or edit to the lab topology.

The solution to this is using Containerlab and ZeroTier. This setup is great for things like testing new hires, training classes, or for providing lab access to others on a limited basis.

Discussions: [:material-twitter:](https://twitter.com/stubarea51/status/1463217901800935427)

### Building Your Own Data Center Fabric with Containerlab
<small>:material-text::material-youtube: [Blog](https://networkcloudandeverything.com/containerlab-post/) and a [screencast](https://youtu.be/d2f1SYRyj0I) by [Alperen Akpinar](https://www.linkedin.com/in/alperenakpinar/) · :material-calendar: 2021-08-24</small>

Alperen did a great job explaining how to build a DC fabric topology using containerlab. This was the first post in the series, making it an excellent intro to containerlab, especially when you follow the [screencast](https://youtu.be/d2f1SYRyj0I) and watch the topology buildup live.

In a [subsequent post](https://networkcloudandeverything.com/configuring-srlinux-nodes-in-a-3-tier-data-center/), Alperen explains how to configure the SR Linux fabric he just built.

[rdodin-twitter]: https://twitter.com/ntdvps
[rdodin-linkedin]: https://linkedin.com/in/rdodin
[karim-linkedin]: https://www.linkedin.com/in/karim-radhouani/
[^1]: most recent talks appear first.