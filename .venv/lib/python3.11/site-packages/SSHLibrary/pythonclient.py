#  Copyright 2008-2015 Nokia Networks
#  Copyright 2016-     Robot Framework Foundation
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
import glob
import os
import ntpath
import time

try:
    import paramiko
except ImportError:
    raise ImportError(
        'Importing Paramiko library failed. '
        'Make sure you have Paramiko installed.'
    )

try:
    import scp
except ImportError:
    raise ImportError(
        'Importing SCP library failed. '
        'Make sure you have SCP installed.'
    )

from .abstractclient import (AbstractShell, AbstractSFTPClient,
                             AbstractSSHClient, AbstractCommand,
                             SSHClientException, SFTPFileInfo)
from .pythonforward import LocalPortForwarding
from .utils import is_bytes, is_list_like, is_unicode, is_truthy
from robot.api import logger


# There doesn't seem to be a simpler way to increase banner timeout
def _custom_start_client(self, *args, **kwargs):
    self.banner_timeout = 45
    self._orig_start_client(*args, **kwargs)


paramiko.transport.Transport._orig_start_client = \
    paramiko.transport.Transport.start_client
paramiko.transport.Transport.start_client = _custom_start_client

# See http://code.google.com/p/robotframework-sshlibrary/issues/detail?id=55
def _custom_log(self, level, msg, *args):
    escape = lambda s: s.replace('%', '%%')
    if is_list_like(msg):
        msg = [escape(m) for m in msg]
    else:
        msg = escape(msg)
    return self._orig_log(level, msg, *args)


paramiko.sftp_client.SFTPClient._orig_log = paramiko.sftp_client.SFTPClient._log
paramiko.sftp_client.SFTPClient._log = _custom_log


class PythonSSHClient(AbstractSSHClient):
    tunnel = None

    def _get_client(self):
        client = paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        return client

    @staticmethod
    def enable_logging(path):
        paramiko.util.log_to_file(path)
        return True

    @staticmethod
    def _read_login_ssh_config(host, username, port_number, proxy_cmd):
        ssh_config_file = os.path.expanduser("~/.ssh/config")
        if os.path.exists(ssh_config_file):
            conf = paramiko.SSHConfig()
            with open(ssh_config_file) as f:
                conf.parse(f)
            port = int(PythonSSHClient._get_ssh_config_port(conf, host, port_number))
            user = PythonSSHClient._get_ssh_config_user(conf, host, username)
            proxy_command = PythonSSHClient._get_ssh_config_proxy_cmd(conf, host, proxy_cmd)
            host = PythonSSHClient._get_ssh_config_host(conf, host)
            return host, user, port, proxy_command
        return host, username, port_number, proxy_cmd

    @staticmethod
    def _read_public_key_ssh_config(host, username, port_number, proxy_cmd, identity_file):
        ssh_config_file = os.path.expanduser("~/.ssh/config")
        if os.path.exists(ssh_config_file):
            conf = paramiko.SSHConfig()
            with open(ssh_config_file) as f:
                conf.parse(f)
            port = int(PythonSSHClient._get_ssh_config_port(conf, host, port_number))
            id_file = PythonSSHClient._get_ssh_config_identity_file(conf, host, identity_file)
            user = PythonSSHClient._get_ssh_config_user(conf, host, username)
            proxy_command = PythonSSHClient._get_ssh_config_proxy_cmd(conf, host, proxy_cmd)
            host = PythonSSHClient._get_ssh_config_host(conf, host)
            return host, user, port, id_file, proxy_command
        return host, username, port_number, identity_file, proxy_cmd

    @staticmethod
    def _get_ssh_config_user(conf, host, user):
        try:
            return conf.lookup(host)['user'] if not None else user
        except KeyError:
            return None

    @staticmethod
    def _get_ssh_config_proxy_cmd(conf, host, proxy_cmd):
        try:
            return conf.lookup(host)['proxycommand'] if not None else proxy_cmd
        except KeyError:
            return proxy_cmd

    @staticmethod
    def _get_ssh_config_identity_file(conf, host, id_file):
        try:
            return conf.lookup(host)['identityfile'][0] if not None else id_file
        except KeyError:
            return id_file

    @staticmethod
    def _get_ssh_config_port(conf, host, port_number):
        try:
            return conf.lookup(host)['port'] if not None else port_number
        except KeyError:
            return port_number

    @staticmethod
    def _get_ssh_config_host(conf, host):
        try:
            return conf.lookup(host)['hostname'] if not None else host
        except KeyError:
            return host

    def _get_jumphost_tunnel(self, jumphost_connection):
        dest_addr = (self.config.host, self.config.port)
        jump_addr = (jumphost_connection.config.host, jumphost_connection.config.port)
        jumphost_transport = jumphost_connection.client.get_transport()
        if not jumphost_transport:
            raise RuntimeError("Could not get transport for {}:{}. Have you logged in?".format(*jump_addr))
        return jumphost_transport.open_channel("direct-tcpip", dest_addr, jump_addr)

    def _login(self, username, password, allow_agent=False, look_for_keys=False, proxy_cmd=None,
               read_config=False, jumphost_connection=None, keep_alive_interval=None):
        if read_config:
            hostname = self.config.host
            self.config.host, username, self.config.port, proxy_cmd = \
                self._read_login_ssh_config(hostname, username, self.config.port, proxy_cmd)

        sock_tunnel = None

        if proxy_cmd and jumphost_connection:
            raise ValueError("`proxy_cmd` and `jumphost_connection` are mutually exclusive SSH features.")
        elif proxy_cmd:
            sock_tunnel = paramiko.ProxyCommand(proxy_cmd)
        elif jumphost_connection:
            sock_tunnel = self._get_jumphost_tunnel(jumphost_connection)
        try:
            if not password and not allow_agent:
                # If no password is given, try login without authentication
                try:
                    self.client.connect(self.config.host, self.config.port, username,
                                        password, look_for_keys=look_for_keys,
                                        allow_agent=allow_agent,
                                        timeout=float(self.config.timeout), sock=sock_tunnel)
                except paramiko.SSHException:
                    pass
                transport = self.client.get_transport()
                transport.set_keepalive(keep_alive_interval)
                transport.auth_none(username)
            else:
                try:
                    self.client.connect(self.config.host, self.config.port, username,
                                        password, look_for_keys=look_for_keys,
                                        allow_agent=allow_agent,
                                        timeout=float(self.config.timeout), sock=sock_tunnel)
                    transport = self.client.get_transport()
                    transport.set_keepalive(keep_alive_interval)
                except paramiko.AuthenticationException:
                    try:
                        transport = self.client.get_transport()
                        transport.set_keepalive(keep_alive_interval)
                        try:
                            transport.auth_none(username)
                        except:
                            pass
                        transport.auth_password(username, password)
                    except:
                        raise SSHClientException
        except paramiko.AuthenticationException:
            raise SSHClientException

    def _login_with_public_key(self, username, key_file, password, allow_agent, look_for_keys, proxy_cmd=None,
                               jumphost_connection=None, read_config=False, keep_alive_interval=None):
        if read_config:
            hostname = self.config.host
            self.config.host, username, self.config.port, key_file, proxy_cmd = \
                self._read_public_key_ssh_config(hostname, username, self.config.port, proxy_cmd, key_file)

        sock_tunnel = None
        if key_file is not None:
            if not os.path.exists(key_file):
                raise SSHClientException("Given key file '%s' does not exist." %
                                         key_file)
            try:
                open(key_file).close()
            except IOError:
                raise SSHClientException("Could not read key file '%s'." % key_file)
        else:
            raise RuntimeError("Keyfile must be specified as keyword argument or in config file.")
        if proxy_cmd and jumphost_connection:
            raise ValueError("`proxy_cmd` and `jumphost_connection` are mutually exclusive SSH features.")
        elif proxy_cmd:
            sock_tunnel = paramiko.ProxyCommand(proxy_cmd)
        elif jumphost_connection:
            sock_tunnel = self._get_jumphost_tunnel(jumphost_connection)

        try:
            self.client.connect(self.config.host, self.config.port, username,
                                password, key_filename=key_file,
                                allow_agent=allow_agent,
                                look_for_keys=look_for_keys,
                                timeout=float(self.config.timeout),
                                sock=sock_tunnel)
            transport = self.client.get_transport()
            transport.set_keepalive(keep_alive_interval)
        except paramiko.AuthenticationException:
            try:
                transport = self.client.get_transport()
                transport.set_keepalive(keep_alive_interval)
                try:
                    transport.auth_none(username)
                except:
                    pass
                transport.auth_publickey(username,None)
            except Exception as err:
                raise SSHClientException

    def get_banner(self):
        return self.client.get_transport().get_banner()

    @staticmethod
    def get_banner_without_login(host, port=22):
        client = paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        try:
            client.connect(str(host), int(port), username="bad-username")
        except paramiko.AuthenticationException:
            return client.get_transport().get_banner()
        except Exception:
            raise SSHClientException('Unable to connect to port {} on {}'.format(port, host))

    def _start_command(self, command, sudo=False,  sudo_password=None, invoke_subsystem=False, forward_agent=False):
        cmd = RemoteCommand(command, self.config.encoding)
        transport = self.client.get_transport()
        if not transport:
            raise AssertionError("Connection not open")
        new_shell = transport.open_session(timeout=float(self.config.timeout))

        if forward_agent:
            paramiko.agent.AgentRequestHandler(new_shell)
            
        cmd.run_in(new_shell, sudo, sudo_password, invoke_subsystem)
        return cmd

    def _create_sftp_client(self):
        return SFTPClient(self.client, self.config.encoding)

    def _create_scp_transfer_client(self):
        return SCPTransferClient(self.client, self.config.encoding)

    def _create_scp_all_client(self):
        return SCPClient(self.client)

    def _create_shell(self):
        return Shell(self.client, self.config.term_type,
                     self.config.width, self.config.height)

    def create_local_ssh_tunnel(self, local_port, remote_host, remote_port, bind_address):
        self._create_local_port_forwarder(local_port, remote_host, remote_port, bind_address)

    def _create_local_port_forwarder(self, local_port, remote_host, remote_port, bind_address):
        transport = self.client.get_transport()
        if not transport:
            raise AssertionError("Connection not open")
        self.tunnel = LocalPortForwarding(int(remote_port), remote_host, transport, bind_address)
        self.tunnel.forward(int(local_port))

    def close(self):
        if self.tunnel:
            self.tunnel.close()
        return super(PythonSSHClient, self).close()


class Shell(AbstractShell):

    def __init__(self, client, term_type, term_width, term_height):
        try:
            self._shell = client.invoke_shell(term_type, term_width, term_height)
        except AttributeError:
            raise RuntimeError('Cannot open session, you need to establish a connection first.')

    def read(self):
        data = b''
        while self._output_available():
            data += self._shell.recv(4096)
        return data

    def read_byte(self):
         if self._output_available():
            return self._shell.recv(1)
         return b''

    def resize(self, width, height):
        self._shell.resize_pty(width=width, height=height)

    def _output_available(self):
        return self._shell.recv_ready()

    def write(self, text):
        self._shell.sendall(text)


class SFTPClient(AbstractSFTPClient):

    def __init__(self, ssh_client, encoding):
        self.ssh_client = ssh_client
        self._client = ssh_client.open_sftp()
        super(SFTPClient, self).__init__(encoding)

    def _list(self, path):
        path = path.encode(self._encoding)
        for item in self._client.listdir_attr(path):
            filename = item.filename
            if is_bytes(filename):
                filename = filename.decode(self._encoding)
            yield SFTPFileInfo(filename, item.st_mode)

    def _stat(self, path):
        path = path.encode(self._encoding)
        attributes = self._client.stat(path)
        return SFTPFileInfo('', attributes.st_mode)

    def _create_missing_remote_path(self, path, mode):
        if is_unicode(path):
            path = path.encode(self._encoding)
        return super(SFTPClient, self)._create_missing_remote_path(path, mode)

    def _create_remote_file(self, destination, mode):
        file_exists = self.is_file(destination)
        destination = destination.encode(self._encoding)
        remote_file = self._client.file(destination, 'wb')
        remote_file.set_pipelined(True)
        if not file_exists and mode:
            self._client.chmod(destination, mode)
        return remote_file

    def _write_to_remote_file(self, remote_file, data, position):
        remote_file.write(data)

    def _close_remote_file(self, remote_file):
        remote_file.close()

    def _get_file(self, remote_path, local_path, scp_preserve_times):
        remote_path = remote_path.encode(self._encoding)
        self._client.get(remote_path, local_path)

    def _absolute_path(self, path):
        if not self._is_windows_path(path):
            path = self._client.normalize(path)
        if is_bytes(path):
            path = path.decode(self._encoding)
        return path

    def _is_windows_path(self, path):
        return bool(ntpath.splitdrive(path)[0])

    def _readlink(self, path):
        return self._client.readlink(path)


class SCPClient(object):
    def __init__(self, ssh_client):
        self._scp_client = scp.SCPClient(ssh_client.get_transport())

    def put_file(self, source, destination, scp_preserve_times, *args):
        sources = self._get_put_file_sources(source)
        self._scp_client.put(sources, destination, preserve_times=is_truthy(scp_preserve_times))

    def get_file(self, source, destination, scp_preserve_times, *args):
        self._scp_client.get(source, destination, preserve_times=is_truthy(scp_preserve_times))

    def put_directory(self, source, destination, scp_preserve_times, *args):
        self._scp_client.put(source, destination, True, preserve_times=is_truthy(scp_preserve_times))

    def get_directory(self, source, destination, scp_preserve_times, *args):
        self._scp_client.get(source, destination, True, preserve_times=is_truthy(scp_preserve_times))

    def _get_put_file_sources(self, source):
        source = source.replace('/', os.sep)
        if not os.path.exists(source):
            sources = [f for f in glob.glob(source)]
        else:
            sources = [f for f in [source]]
        if not sources:
            msg = "There are no source files matching '%s'." % source
            raise SSHClientException(msg)
        return sources


class SCPTransferClient(SFTPClient):

    def __init__(self, ssh_client, encoding):
        self._scp_client = scp.SCPClient(ssh_client.get_transport())
        super(SCPTransferClient, self).__init__(ssh_client, encoding)

    def _put_file(self, source, destination, mode, newline, path_separator, scp_preserve_times=False):
        self._create_remote_file(destination, mode)
        self._scp_client.put(source, destination, preserve_times=is_truthy(scp_preserve_times))

    def _get_file(self, remote_path, local_path, scp_preserve_times=False):
        self._scp_client.get(remote_path, local_path, preserve_times=is_truthy(scp_preserve_times))


class RemoteCommand(AbstractCommand):

    def read_outputs(self, timeout=None, output_during_execution=False, output_if_timeout=False):
        stderr, stdout = self._receive_stdout_and_stderr(timeout, output_during_execution, output_if_timeout)
        rc = self._shell.recv_exit_status()
        self._shell.close()
        return stdout, stderr, rc

    def _receive_stdout_and_stderr(self, timeout=None, output_during_execution=False, output_if_timeout=False):
        stdout_filebuffer = self._shell.makefile('rb', -1)
        stderr_filebuffer = self._shell.makefile_stderr('rb', -1)
        stdouts = []
        stderrs = []
        while self._shell_open():
            self._flush_stdout_and_stderr(stderr_filebuffer, stderrs, stdout_filebuffer, stdouts, timeout,
                                          output_during_execution, output_if_timeout)
            time.sleep(0.01)  # lets not be so busy
        stdout = (b''.join(stdouts) + stdout_filebuffer.read()).decode(self._encoding)
        stderr = (b''.join(stderrs) + stderr_filebuffer.read()).decode(self._encoding)
        return stderr, stdout

    def _flush_stdout_and_stderr(self, stderr_filebuffer, stderrs, stdout_filebuffer, stdouts, timeout=None,
                                 output_during_execution=False, output_if_timeout=False):
        if timeout:
            end_time = time.time() + timeout
            while time.time() < end_time:
                if self._shell.status_event.wait(0):
                    break
                self._output_logging(stderr_filebuffer, stderrs, stdout_filebuffer, stdouts, output_during_execution)
            if not self._shell.status_event.isSet():
                if is_truthy(output_if_timeout):
                    logger.info(stdouts)
                    logger.info(stderrs)
                raise SSHClientException('Timed out in %s seconds' % int(timeout))
        else:
            self._output_logging(stderr_filebuffer, stderrs, stdout_filebuffer, stdouts, output_during_execution)

    def _output_logging(self, stderr_filebuffer, stderrs, stdout_filebuffer, stdouts, output_during_execution=False):
        if self._shell.recv_ready():
            stdout_output = stdout_filebuffer.read(len(self._shell.in_buffer))
            if is_truthy(output_during_execution):
                logger.console(stdout_output)
            stdouts.append(stdout_output)
        if self._shell.recv_stderr_ready():
            stderr_output = stderr_filebuffer.read(len(self._shell.in_stderr_buffer))
            if is_truthy(output_during_execution):
                logger.console(stderr_output)
            stderrs.append(stderr_output)

    def _shell_open(self):
        return not (self._shell.closed or
                    self._shell.eof_received or
                    self._shell.eof_sent or
                    not self._shell.active)

    def _execute(self):
        self._shell.exec_command(self._command)

    def _execute_with_sudo(self, sudo_password=None):
        command = self._command.decode(self._encoding)
        if sudo_password is None:
            self._shell.exec_command('sudo ' + command)
        else:
            self._shell.exec_command('sudo --stdin --prompt "" %s' % (command))
            self._shell.sendall('\n\n' + sudo_password + '\n')

    def _invoke(self):
        self._shell.invoke_subsystem(self._command)
