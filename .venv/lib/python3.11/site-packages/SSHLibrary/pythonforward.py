import select
import socket
import threading
from robot.utils import platform
from .logger import logger
if platform.PY2 and platform.WINDOWS:
    import win_inet_pton
try:
    import SocketServer
except ImportError:
    import socketserver as SocketServer


def check_if_ipv6(ip):
    try:
        socket.inet_pton(socket.AF_INET6, ip)
        return True
    except socket.error:
        return False


class LocalPortForwarding:
    def __init__(self, port, host, transport, bind_address):
        self.server = None
        self.port = port
        self.host = host
        self.transport = transport
        self.bind_address = bind_address

    def forward(self, local_port):
        class SubHandler(LocalPortForwardingHandler):
            port = self.port
            host = self.host
            ssh_transport = self.transport

        self.server = ForwardServer((self.bind_address or '', local_port), SubHandler, ipv6=check_if_ipv6(self.host))
        t = threading.Thread(target=self.server.serve_forever)
        t.setDaemon(True)
        t.start()
        logger.info("Now forwarding port %d to %s:%d ..." % (local_port, self.host, self.port))

    def close(self):
        if self.server:
            self.server.shutdown()
            try:
                logger.log_background_messages()
            except AttributeError:
                pass


class ForwardServer(SocketServer.ThreadingTCPServer):
    daemon_threads = True
    allow_reuse_address = True

    def __init__(self, server_address, RequestHandlerClass, ipv6=False):
        if ipv6:
            ForwardServer.address_family = socket.AF_INET6
        SocketServer.ThreadingTCPServer.__init__(self, server_address, RequestHandlerClass, bind_and_activate=True)


class LocalPortForwardingHandler(SocketServer.BaseRequestHandler):
    host, port, ssh_transport = None, None, None

    def handle(self):
        try:
            chan = self.ssh_transport.open_channel('direct-tcpip', (self.host, self.port),
                                                   self.request.getpeername())
        except Exception as e:
            logger.info("Incoming request to %s:%d failed: %s" % (self.host, self.port, repr(e)))
            return
        if chan is None:
            logger.info("Incoming request to %s:%d was rejected by the SSH server." % (self.host, self.port))
            return
        logger.info("Connected! Tunnel open %r -> %r -> %r" % (self.request.getpeername(),
                                                               chan.getpeername(),
                                                               (self.host, self.port)))
        while True:
            r, w, x = select.select([self.request, chan], [], [])
            if self.request in r:
                data = self.request.recv(1024)
                if len(data) == 0:
                    break
                chan.send(data)
            if chan in r:
                data = chan.recv(1024)
                if len(data) == 0:
                    break
                self.request.send(data)
        peername = self.request.getpeername()
        chan.close()
        self.request.close()
        logger.info("Tunnel closed from %r" % (peername,))
