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

try:
    from com.trilead.ssh2 import (Connection, SCPClient as JavaSCPClient,
                                  SFTPException, SFTPv3Client,
                                  SFTPv3DirectoryEntry, StreamGobbler)
except ImportError:
    raise ImportError(
        'Importing Trilead SSH library failed. '
        'Make sure you have the Trilead JAR distribution in your CLASSPATH.'
    )
import jarray
import os
from java.io import (BufferedReader, File, FileOutputStream, InputStreamReader,
                     IOException)

from .abstractclient import (AbstractShell, AbstractSSHClient,
                             AbstractSFTPClient, AbstractCommand,
                             SSHClientException, SFTPFileInfo)
try:
    from robot.api import logger
except ImportError:
    logger = None


class JavaSSHClientException(Exception):
    pass


def _wait_until_timeout(_shell, timeout):
    timeout_condition = 1
    rc = 32
    condition = _shell.waitForCondition(rc , int(timeout) * 1000)

    if condition & timeout_condition != 0:
        raise SSHClientException("Timed out in %s seconds" % int(timeout))

class JavaSSHClient(AbstractSSHClient):

    def _get_client(self):
        client = Connection(self.config.host, self.config.port)
        timeout = int(float(self.config.timeout)*1000)
        client.connect(None, timeout, timeout)
        return client

    @staticmethod
    def enable_logging(logfile):
        return False

    def _login(self, username, password, allow_agent='ignored', look_for_keys='ignored',
               proxy_cmd=None, jumphost_alias_or_index=None, read_config=False, keep_alive_interval=None):
        if allow_agent or look_for_keys or keep_alive_interval:
            raise JavaSSHClientException("Arguments 'allow_agent', 'look_for_keys', "
                                         "`jumphost_index_or_alias` and `keep_alive_interval`" 
                                         " do not work with Jython.")
        if not self.client.authenticateWithPassword(username, password):
            raise SSHClientException

    def _login_with_public_key(self, username, key_file, password,
                               allow_agent='ignored', look_for_keys='ignored',
                               proxy_cmd=None, jumphost_alias_or_index=None,
                               read_config=False, keep_alive_interval=None):
        if allow_agent or look_for_keys or keep_alive_interval:
            raise JavaSSHClientException("Arguments 'allow_agent', 'look_for_keys', "
                                         "`jumphost_index_or_alias` and `keep_alive_interval`"
                                         " do not work with Jython.")
        try:
            success = self.client.authenticateWithPublicKey(username,
                                                            File(key_file),
                                                            password)
            if not success:
                raise SSHClientException
        except IOError:
            # IOError is raised also when the keyfile is invalid
            raise SSHClientException

    def _start_command(self, command, sudo=False, sudo_password=None, invoke_subsystem=False, forward_agent=False):
        new_shell = self.client.openSession()
        if sudo:
            new_shell.requestDumbPTY()
        cmd = RemoteCommand(command, self.config.encoding)
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

    def create_local_ssh_tunnel(self, local_port, remote_host, remote_port, *args):
        self.client.createLocalPortForwarder(int(local_port), remote_host, int(remote_port))
        logger.info("Now forwarding port %s to %s:%s ..." % (local_port, remote_host, remote_port))


class Shell(AbstractShell):

    def __init__(self, client, term_type, term_width, term_height):
        shell = client.openSession()
        shell.requestPTY(term_type, term_width, term_height, 0, 0, None)
        shell.startShell()
        self.shell = shell
        self._stdout = shell.getStdout()
        self._stdin = shell.getStdin()

    def read(self):
        if self._output_available():
            read_bytes = jarray.zeros(self._output_available(), 'b')
            self._stdout.read(read_bytes)
            return ''.join(chr(b & 0xFF) for b in read_bytes)
        return ''

    def read_byte(self):
         if self._output_available():
             return chr(self._stdout.read())
         return ''

    @staticmethod
    def resize(width, height):
        logger.warn('Setting width or height is not supported with Jython.')

    def _output_available(self):
        return self._stdout.available()

    def write(self, text):
        self._stdin.write(text)
        self._stdin.flush()


class SFTPClient(AbstractSFTPClient):

    def __init__(self, ssh_client, encoding):
        self._client = SFTPv3Client(ssh_client)
        self._client.setCharset(encoding)
        super(SFTPClient, self).__init__(encoding)

    def _list(self, path):
        for item in self._client.ls(path):
            if item.filename not in ('.', '..'):
                yield SFTPFileInfo(item.filename, item.attributes.permissions)

    def _stat(self, path):
        attributes = self._client.stat(path)
        return SFTPFileInfo('', attributes.permissions)

    def _create_remote_file(self, destination, mode):
        remote_file = self._client.createFile(destination)
        try:
            file_stat = self._client.fstat(remote_file)
            file_stat.permissions = mode
            self._client.fsetstat(remote_file, file_stat)
        except SFTPException:
            pass
        return remote_file

    def _write_to_remote_file(self, remote_file, data, position):
        self._client.write(remote_file, position, data, 0, len(data))

    def _close_remote_file(self, remote_file):
        self._client.closeFile(remote_file)

    def _get_file(self, remote_path, local_path, scp_preserve_times):
        local_file = FileOutputStream(local_path)
        remote_file_size = self._client.stat(remote_path).size
        remote_file = self._client.openFileRO(remote_path)
        array_size_bytes = 4096
        data = jarray.zeros(array_size_bytes, 'b')
        offset = 0
        while True:
            read_bytes = self._client.read(remote_file, offset, data, 0,
                                           array_size_bytes)
            data_length = len(data)
            if read_bytes == -1:
                break
            if remote_file_size - offset < array_size_bytes:
                data_length = remote_file_size - offset
            local_file.write(data, 0, data_length)
            offset += data_length
        self._client.closeFile(remote_file)
        local_file.flush()
        local_file.close()

    def _absolute_path(self, path):
        return self._client.canonicalPath(path)

    def _readlink(self, path):
        return self._client.readLink(path)


class SCPClient(object):
    def __init__(self, ssh_client):
        self._scp_client = JavaSCPClient(ssh_client)

    def put_file(self, source, destination, *args):
        self._scp_client.put(source, destination)

    def get_file(self, source, destination, *args):
        self._scp_client.get(source, destination)

    def put_directory(self, source, destination, *args):
        raise JavaSSHClientException('`Put Directory` not available with `scp=ALL` option. Try again with '
                                     '`scp=TRANSFER` or `scp=OFF`.')

    def get_directory(self, source, destination, *args):
        raise JavaSSHClientException('`Get Directory` not available with `scp=ALL` option. Try again with '
                                     '`scp=TRANSFER` or `scp=OFF`.')


class SCPTransferClient(SFTPClient):

    def __init__(self, ssh_client, encoding):
        self._scp_client = JavaSCPClient(ssh_client)
        super(SCPTransferClient, self).__init__(ssh_client, encoding)

    def _put_file(self, source, destination, mode, newline, path_separator, scp_preserve_times):
        self._create_remote_file(destination, mode)
        self._scp_client.put(source, destination.rsplit(path_separator, 1)[0])

    def _get_file(self, remote_path, local_path, scp_preserve_times):
        self._scp_client.get(remote_path, local_path.rsplit(os.sep, 1)[0])


class RemoteCommand(AbstractCommand):

    def read_outputs(self, timeout=None, *args):
        if timeout:
            _wait_until_timeout(self._shell, timeout)
        stdout = self._read_from_stream(self._shell.getStdout())
        stderr = self._read_from_stream(self._shell.getStderr())
        rc = self._shell.getExitStatus() or 0
        self._shell.close()
        return stdout, stderr, rc

    def _read_from_stream(self, stream):
        reader = BufferedReader(InputStreamReader(StreamGobbler(stream),
                                                  self._encoding))
        result = ''
        line = reader.readLine()
        while line is not None:
            result += line + '\n'
            line = reader.readLine()
        return result

    def _execute(self):
        command = self._command.decode(self._encoding)
        self._shell.execCommand(command)

    def _execute_with_sudo(self, sudo_password=None):
        command = self._command.decode(self._encoding)
        if sudo_password is None:
            self._shell.execCommand('sudo ' + command)
        else:
            self._shell.execCommand('sudo --stdin --prompt "" %s' % (command))
            self._shell.write('\n\n' + sudo_password + '\n')

    def _invoke(self):
        command = self._command.decode(self._encoding)
        self._shell.startSubSystem(command)

