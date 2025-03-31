# Share Lab Access

Lightweight, fast to deploy, and reproducible containerlabs are meant to be shared via Git. But sometimes, you can't quite share the entire lab... Maybe the recipient doesn't have access to the container images or has other limitations that prevent them from running a copy of you lab.

Yet, you might find yourself in need to share access of your lab with one or many users. Quite often you want someone else to have a look at your lab when you found something or got stuck. Or maybe you are a lecturer and want to broadcast your lab interaction to your students.

Here we will discuss different ways how you can share your lab with others in a secure and interactive way.

## SSHX

[SSHX](https://sshx.io) is a web-based terminal emulator that allows you to share access to a terminal sessions with others and have a collaborative terminal experience.

Containerlab users can leverage this handy free service[^1] to share lab access with read/write and read-only access by adding a simple container with `sshx` to their set of lab nodes:

```yaml
name: shared-lab
topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.3.1

    sshx:
      kind: linux
      image: ghcr.io/srl-labs/network-multitool
      exec:
        - >-
          ash -c "curl -sSf https://sshx.io/get | sh > /dev/null ;
          sshx -q --enable-readers > /tmp/sshx &
          while [ ! -s /tmp/sshx ]; do sleep 1; done && cat /tmp/sshx"
```

By adding the `sshx` node to this simple lab and deploying it, you will see an sshx link in the lab output:

```shell
# the rest of the deploy log is omitted for brevity
21:44:15 INFO Running postdeploy actions kind=nokia_srlinux node=srl
21:44:28 INFO Executed command node=sshx command="ash -c curl -sSf https://sshx.io/get | sh > /dev/null" stdout=""
21:44:28 INFO Executed command node=sshx command="ash -c sshx -q --enable-readers > /tmp/sshx &" stdout=""
21:44:28 INFO Executed command node=sshx command="cat /tmp/sshx"
  stdout=
  │ https://sshx.io/s/94eAaunO2L#CbTbaD6bqB90oU,mCIP5PtBmF41qL

21:44:28 INFO Adding host entries path=/etc/hosts
21:44:28 INFO Adding SSH config for nodes path=/etc/ssh/ssh_config.d/clab-shared-lab.conf
```

Take a look at the stdout line that contains the sshx link:

```
https://sshx.io/s/94eAaunO2L#CbTbaD6bqB90oU,mCIP5PtBmF41qL
```

By pasting this link in a browser you will get a collaborative terminal session in the browser where you can open many shell windows that will belong to the `sshx` container that runs from the `ghcr.io/srl-labs/network-multitool` with lots of networking tools installed.

The great part is that this `sshx` container has access to all other lab nodes and you can refer to them by name, as they appear in your topology file. For example, we can ssh to the SR Linux container that is part of our lab:

![img](https://gitlab.com/rdodin/pics/-/wikis/uploads/38073aeb55006b57f4b5e3db1d6a230f/CleanShot_2025-03-30_at_21.46.35_2x.png)

Attentive reader also noticed that sshx link has two parts separated by a comma. If you copy the link from the beginning to a comma, you get a link that has **read-only** access. Share this link with someone and they will be able to see by can not touch...

The full link gives **read/write** access, and users with a link would be able to open terminals and execute commands in the shell.

You can also provide the sshx-based access to a lab on demand, even if your lab file did not feature the sshx node from the beginning.  
To do so, run the adhoc command to create a container in the docker network that your lab uses (`clab` by default):

```shell
docker rm -f sshx-adhoc
docker run --network clab --rm -i -t \
  --name sshx-adhoc --entrypoint '' -d \
  ghcr.io/srl-labs/network-multitool \
  ash -c 'ash -c "curl -sSf https://sshx.io/get | sh > /dev/null ;
  sshx -q --enable-readers"'
docker logs -f sshx-adhoc
```

You will get to see the same sshx link as if you had sshx container node in your lab topology.

[^1]: Feel free to access the security of it by googling other researches having a go at it. For a self-hosted alternative you may consider [frp](https://github.com/fatedier/frp).
