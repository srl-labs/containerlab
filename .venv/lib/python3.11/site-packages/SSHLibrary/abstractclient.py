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

from fnmatch import fnmatchcase
import os
import re
import stat
import time
import glob
import posixpath
import ntpath
import fnmatch

from .config import (Configuration, IntegerEntry, NewlineEntry, StringEntry,
                     TimeEntry)
from .logger import logger
from .utils import is_bytes, is_string, unicode, is_list_like


class SSHClientException(RuntimeError):
    pass


class _ClientConfiguration(Configuration):

    def __init__(self, host, alias, port, timeout, newline, prompt, term_type,
                 width, height, path_separator, encoding, escape_ansi, encoding_errors):
        super(_ClientConfiguration, self).__init__(
            index=IntegerEntry(None),
            host=StringEntry(host),
            alias=StringEntry(alias),
            port=IntegerEntry(port),
            timeout=TimeEntry(timeout),
            newline=NewlineEntry(newline),
            prompt=StringEntry(prompt),
            term_type=StringEntry(term_type),
            width=IntegerEntry(width),
            height=IntegerEntry(height),
            path_separator=StringEntry(path_separator),
            encoding=StringEntry(encoding),
            escape_ansi=StringEntry(escape_ansi),
            encoding_errors=StringEntry(encoding_errors)
        )


class AbstractSSHClient(object):
    """Base class for the SSH client implementation.

    This class defines the public API. Subclasses (:py:class:`pythonclient.
    PythonSSHClient` and :py:class:`javaclient.JavaSSHClient`) provide the
    language specific concrete implementations.
    """
    def __init__(self, host, alias=None, port=22, timeout=3, newline='LF',
                 prompt=None, term_type='vt100', width=80, height=24,
                 path_separator='/', encoding='utf8', escape_ansi=False, encoding_errors='strict'):
        self.config = _ClientConfiguration(host, alias, port, timeout, newline,
                                           prompt, term_type, width, height,
                                           path_separator, encoding, escape_ansi, encoding_errors)
        self._sftp_client = None
        self._scp_transfer_client = None
        self._scp_all_client = None
        self._shell = None
        self._started_commands = []
        self.client = self._get_client()
        self.width = width
        self.height = height

    def _get_client(self):
        raise NotImplementedError('This should be implemented in the subclass.')

    @staticmethod
    def enable_logging(path):
        """Enables logging of SSH events to a file.

        :param str path: Path to the file the log is written to.

        :returns: `True`, if logging was successfully enabled. False otherwise.
        """
        raise NotImplementedError

    @property
    def sftp_client(self):
        """Gets the SFTP client for the connection.

        :returns: An object of the class that inherits from
            :py:class:`AbstractSFTPClient`.
        """
        if not self._sftp_client:
            self._sftp_client = self._create_sftp_client()
        return self._sftp_client

    @property
    def scp_transfer_client(self):
        """Gets the SCP client for the file transfer.

        :returns: An object of the class that inherits from
            :py:class:`SFTPClient`.
        """
        if not self._scp_transfer_client:
            self._scp_transfer_client = self._create_scp_transfer_client()
        return self._scp_transfer_client

    @property
    def scp_all_client(self):
        """Gets the SCP client for the file transfer.

        :returns: An object of the class type
            :py:class:`SCPClient`.
        """
        if not self._scp_all_client:
            self._scp_all_client = self._create_scp_all_client()
        return self._scp_all_client

    @property
    def shell(self):
        """Gets the shell for the connection.

        :returns: An object of the class that inherits from
            :py:class:`AbstractShell`.
        """
        if not self._shell:
            self._shell = self._create_shell()
        if self.width != self.config.width or self.height != self.config.height:
            self._shell.resize(self.config.width, self.config.height)
            self.width, self.height = self.config.width, self.config.height
        return self._shell

    def _create_sftp_client(self):
        raise NotImplementedError

    def _create_scp_transfer_client(self):
        raise NotImplementedError

    def _create_scp_all_client(self):
        raise NotImplementedError

    def _create_shell(self):
        raise NotImplementedError

    def close(self):
        """Closes the connection."""
        self._sftp_client = None
        self._scp_transfer_client = None
        self._scp_all_client = None
        self._shell = None
        self.client.close()
        try:
            logger.log_background_messages()
        except AttributeError:
            pass

    def login(self, username=None, password=None, allow_agent=False, look_for_keys=False, delay=None, proxy_cmd=None,
              read_config=False, jumphost_connection=None, keep_alive_interval=None):
        """Logs into the remote host using password authentication.

        This method reads the output from the remote host after logging in,
        thus clearing the output. If prompt is set, everything until the prompt
        is read (using :py:meth:`read_until_prompt` internally).
        Otherwise everything on the output is read with the specified `delay`
        (using :py:meth:`read` internally).

        :param keep_alive_interval: Set the transport keepalive interval.

        :param str username: Username to log in with.

        :param str password: Password for the `username`.

        :param bool allow_agent: enables the connection to the SSH agent.
            This option does not work when using Jython.

        :param bool look_for_keys: Whether the login method should look for
            available public keys for login. This will also enable ssh agent.
            This option is ignored when using Jython.

        :param str proxy_cmd: Proxy command
        :param str delay: The `delay` passed to :py:meth:`read` for reading
            the output after logging in. The delay is only effective if
            the prompt is not set.

        :param read_config: reads or ignores host entries from ``~/.ssh/config`` file. This parameter will read the hostname,
        port number, username and proxy command.

        :param PythonSSHClient jumphost_connection : An instance of
            PythonSSHClient that will be used as an intermediary jump-host
            for the SSH connection being attempted.

        :raises SSHClientException: If logging in failed.

        :returns: The read output from the server.
        """
        keep_alive_interval = int(TimeEntry(keep_alive_interval).value)
        username = self._encode(username)
        if not password and not allow_agent:
            password = self._encode(password)
        try:
            self._login(username, password, allow_agent, look_for_keys, proxy_cmd, read_config,
                        jumphost_connection, keep_alive_interval)
        except SSHClientException:
            self.client.close()
            raise SSHClientException("Authentication failed for user '%s'."
                                     % self._decode(username))
        return self._read_login_output(delay)

    def _encode(self, text):
        if is_bytes(text):
            return text
        if not is_string(text):
            text = unicode(text)
        return text.encode(self.config.encoding, self.config.encoding_errors)

    def _decode(self, bytes):
        return bytes.decode(self.config.encoding, self.config.encoding_errors)

    def _login(self, username, password, allow_agent, look_for_keys, proxy_cmd, read_config,
               jumphost_connection, keep_alive_interval):
        raise NotImplementedError

    def _read_login_output(self, delay):
        if not self.config.prompt:
            return self.read(delay)
        elif self.config.prompt.startswith('REGEXP:'):
            return self.read_until_regexp(self.config.prompt[7:])
        return self.read_until_prompt()

    def login_with_public_key(self, username, keyfile, password, allow_agent=False,
                              look_for_keys=False, delay=None, proxy_cmd=None,
                              jumphost_connection=None, read_config=False, keep_alive_interval=None):
        """Logs into the remote host using the public key authentication.

        This method reads the output from the remote host after logging in,
        thus clearing the output. If prompt is set, everything until the prompt
        is read (using :py:meth:`read_until_prompt` internally).
        Otherwise everything on the output is read with the specified `delay`
        (using :py:meth:`read` internally).

        :param str username: Username to log in with.

        :param str keyfile: Path to the valid OpenSSH private key file.

        :param str password: Password (if needed) for unlocking the `keyfile`.

        :param boolean allow_agent: enables the connection to the SSH agent.
            This option does not work when using Jython.

        :param boolean look_for_keys: enables the searching for discoverable
            private key files in ~/.ssh/. This option also does not work when
            using Jython.

        :param str delay: The `delay` passed to :py:meth:`read` for reading
            the output after logging in. The delay is only effective if
            the prompt is not set.

        :param str proxy_cmd : Proxy command

        :param PythonSSHClient jumphost_connection : An instance of
            PythonSSHClient that is will be used as an intermediary jump-host
            for the SSH connection being attempted.

        :param read_config: reads or ignores entries from ``~/.ssh/config`` file. This parameter will read the hostname,
        port number, username, identity file and proxy command.

        :raises SSHClientException: If logging in failed.

        :returns: The read output from the server.
        """
        if username:
            username = self._encode(username)
        if keyfile:
            self._verify_key_file(keyfile)
        keep_alive_interval = int(TimeEntry(keep_alive_interval).value)
        try:
            self._login_with_public_key(username, keyfile, password,
                                        allow_agent, look_for_keys,
                                        proxy_cmd, jumphost_connection,
                                        read_config, keep_alive_interval)
        except SSHClientException:
            self.client.close()
            raise SSHClientException("Login with public key failed for user "
                                     "'%s'." % self._decode(username))
        return self._read_login_output(delay)

    def _verify_key_file(self, keyfile):
        if not os.path.exists(keyfile):
            raise SSHClientException("Given key file '%s' does not exist." %
                                     keyfile)
        try:
            open(keyfile).close()
        except IOError:
            raise SSHClientException("Could not read key file '%s'." % keyfile)

    def _login_with_public_key(self, username, keyfile, password,
                               allow_agent, look_for_keys, proxy_cmd,
                               jumphost_index_or_alias, read_config, keep_alive_interval):
        raise NotImplementedError

    @staticmethod
    def get_banner_without_login(host, port=22):
        raise NotImplementedError('Not supported on this Python interpreter.')

    def get_banner(self):
        raise NotImplementedError('Not supported on this Python interpreter.')

    def execute_command(self, command, sudo=False,  sudo_password=None, timeout=None, output_during_execution=False,
                        output_if_timeout=False, invoke_subsystem=False, forward_agent=False):
        """Executes the `command` on the remote host.

        This method waits until the output triggered by the execution of the
        `command` is available and then returns it.

        The `command` is always executed in a new shell, meaning that changes to
        the environment are not visible to the subsequent calls of this method.

        :param str command: The command to be executed on the remote host.

        :param sudo
         and
        :param sudo_password are used for executing commands within a sudo session.

        :param invoke_subsystem will request a subsystem on the server.

        :returns: A 3-tuple (stdout, stderr, return_code) with values
            `stdout` and `stderr` as strings and `return_code` as an integer.
        """
        self.start_command(command, sudo, sudo_password, invoke_subsystem, forward_agent)
        return self.read_command_output(timeout=timeout, output_during_execution=output_during_execution,
                                        output_if_timeout=output_if_timeout)

    def start_command(self, command, sudo=False,  sudo_password=None, invoke_subsystem=False, forward_agent=False):
        """Starts the execution of the `command` on the remote host.

        The started `command` is pushed into an internal stack. This stack
        always has the latest started `command` on top of it.

        The `command` is always started in a new shell, meaning that changes to
        the environment are not visible to the subsequent calls of this method.

        This method does not return anything. Use :py:meth:`read_command_output`
        to get the output of the previous started command.

        :param str command: The command to be started on the remote host.

        :param sudo
         and
        :param sudo_password are used for executing commands within a sudo session.

        :param invoke_subsystem will request a subsystem on the server.
        """
        command = self._encode(command)

        self._started_commands.append(self._start_command(command, sudo, sudo_password, invoke_subsystem, forward_agent))

    def _start_command(self, command, sudo=False, sudo_password=None, invoke_subsystem=False, forward_agent=False):
        raise NotImplementedError

    def read_command_output(self, timeout=None, output_during_execution=False, output_if_timeout=False):
        """Reads the output of the previous started command.

        The previous started command, started with :py:meth:`start_command`,
        is popped out of the stack and its outputs (stdout, stderr and the
        return code) are read and returned.

        :raises SSHClientException: If there are no started commands to read
            output from.

        :returns: A 3-tuple (stdout, stderr, return_code) with values
            `stdout` and `stderr` as strings and `return_code` as an integer.
        """
        if timeout:
            timeout = float(TimeEntry(timeout).value)
        try:
            return self._started_commands.pop().read_outputs(timeout, output_during_execution, output_if_timeout)
        except IndexError:
            raise SSHClientException('No started commands to read output from.')

    def write(self, text, add_newline=False):
        """Writes `text` in the current shell.

        :param str text: The text to be written.

        :param bool add_newline: If `True`, the configured newline will be
            appended to the `text` before writing it on the remote host.
            The newline is set when calling :py:meth:`open_connection`
        """
        text = self._encode(text)
        if add_newline:
            text += self._encode(self.config.newline)
        self.shell.write(text)

    def read(self, delay=None):
        """Reads all output available in the current shell.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :param str delay: If given, this method reads again after the delay
            to see if there is more output is available. This wait-read cycle is
            repeated as long as further reads return more output or the
            configured timeout expires. The timeout is set when calling
            :py:meth:`open_connection`. The delay can be given as an integer
            (the number of seconds) or in Robot Framework's time format, e.g.
            `4.5s`, `3 minutes`, `2 min 3 sec`.

        :returns: The read output from the remote host.
        """
        output = self.shell.read()
        if delay:
            output += self._delayed_read(delay)
        return self._decode(output)

    def _delayed_read(self, delay):
        delay = TimeEntry(delay).value
        max_time = time.time() + self.config.get('timeout').value
        output = b''
        while time.time() < max_time:
            time.sleep(delay)
            read = self.shell.read()
            if not read:
                break
            output += read
        return output

    def read_char(self):
        """Reads a single Unicode character from the current shell.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :returns: A single char read from the output.
        """
        server_output = b''
        while True:
            try:
                server_output += self.shell.read_byte()
                return self._decode(server_output)
            except UnicodeDecodeError as e:
                if e.reason == 'unexpected end of data':
                    pass
                else:
                    raise

    def read_until(self, expected):
        """Reads output from the current shell until the `expected` text is
        encountered or the timeout expires.

        The timeout is set when calling :py:meth:`open_connection`.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :param str expected: The text to look for in the output.

        :raises SSHClientException: If `expected` is not found in the output
            when the timeout expires.

        :returns: The read output, including the encountered `expected` text.
        """
        return self._read_until(lambda s: expected in s, expected)

    def _read_until(self, matcher, expected, timeout=None):
        output = ''
        timeout = TimeEntry(timeout) if timeout else self.config.get('timeout')
        max_time = time.time() + timeout.value
        while time.time() < max_time:
            char = self.read_char()
            if not char:
                time.sleep(.00001)  # Release GIL so paramiko I/O thread can run
            output += char
            if matcher(output):
                return output
        raise SSHClientException("No match found for '%s' in %s\nOutput:\n%s."
                                 % (expected, timeout, output))

    def read_until_newline(self):
        """Reads output from the current shell until a newline character is
        encountered or the timeout expires.

        The newline character and the timeout are set when calling
        :py:meth:`open_connection`.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :raises SSHClientException: If the newline character is not found in the
            output when the timeout expires.

        :returns: The read output, including the encountered newline character.
        """
        return self.read_until(self.config.newline)

    def read_until_prompt(self, strip_prompt=False):
        """Reads output from the current shell until the prompt is encountered
        or the timeout expires.

        The prompt and timeout are set when calling :py:meth:`open_connection`.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :param bool strip_prompt: If 'True' then the prompt is removed from
            the resulting output

        :raises SSHClientException: If prompt is not set or is not found
            in the output when the timeout expires.

        :returns: The read output, including the encountered prompt.
        """
        if not self.config.prompt:
            raise SSHClientException('Prompt is not set.')

        if self.config.prompt.startswith('REGEXP:'):
            output = self.read_until_regexp(self.config.prompt[7:])
        else:
            output = self.read_until(self.config.prompt)
        if strip_prompt:
            output = self._strip_prompt(output)
        return output

    def _strip_prompt(self, output):
        if self.config.prompt.startswith('REGEXP:'):
            pattern = re.compile(self.config.prompt[7:])
            match = pattern.search(output)
            length = match.end() - match.start()
        else:
            length = len(self.config.prompt)
        return output[:-length]

    def read_until_regexp(self, regexp):
        """Reads output from the current shell until the `regexp` matches or
        the timeout expires.

        The timeout is set when calling :py:meth:`open_connection`.

        Reading always consumes the output, meaning that after being read,
        the read content is no longer present in the output.

        :param regexp: Either the regular expression as a string or a compiled
            Regex object.

        :raises SSHClientException: If no match against `regexp` is found when
            the timeout expires.

        :returns: The read output up and until the `regexp` matches.
        """
        if is_string(regexp):
            regexp = re.compile(regexp)
        return self._read_until(lambda s: regexp.search(s), regexp.pattern)

    def read_until_regexp_with_prefix(self, regexp, prefix):
        """
        Read and return from output until regexp matches prefix + output.

        :param regexp: a pattern or a compiled regexp object used for matching
        :raises SSHClientException: if match is not found in prefix+output when
            timeout expires.

        timeout is defined with :py:meth:`open_connection()`
        """
        if is_string(regexp):
            regexp = re.compile(regexp)
        matcher = regexp.search
        expected = regexp.pattern
        ret = ""
        timeout = self.config.get('timeout')
        start_time = time.time()
        while time.time() < float(timeout.value) + start_time:
            ret += self.read_char()
            if matcher(prefix + self._encode(ret)):
                return ret
        raise SSHClientException(
            "No match found for '%s' in %s\nOutput:\n%s"
            % (expected, timeout, ret))

    def write_until_expected(self, text, expected, timeout, interval):
        """Writes `text` repeatedly in the current shell until the `expected`
        appears in the output or the `timeout` expires.

        :param str text: Text to be written. Uses :py:meth:`write_bare`
            internally so no newline character is appended to the written text.

        :param str expected: Text to look for in the output.

        :param int timeout: The timeout during which `expected` must appear
            in the output. Can be given as an integer (the number of seconds)
            or in Robot Framework's time format, e.g. `4.5s`, `3 minutes`,
            `2 min 3 sec`.

        :param int interval: Time to wait between the repeated writings of
            `text`.

        :raises SSHClientException: If `expected` is not found in the output
            before the `timeout` expires.

        :returns: The read output, including the encountered `expected` text.
        """
        expected = self._encode(expected)
        interval = TimeEntry(interval)
        timeout = TimeEntry(timeout)
        max_time = time.time() + timeout.value
        while time.time() < max_time:
            self.write(text)
            try:
                return self._read_until(lambda s: expected in self._encode(s), expected,
                                        timeout=interval.value)
            except SSHClientException:
                pass
        raise SSHClientException("No match found for '%s' in %s."
                                 % (self._decode(expected), timeout))

    def put_file(self, source, destination='.', mode='0o744', newline='',
                 scp='OFF', scp_preserve_times=False):
        """Calls :py:meth:`AbstractS`FTPClient.put_file` with the given
        arguments.

        See :py:meth:`AbstractSFTPClient.put_file` for more documentation.
        """
        client = self._create_client(scp)
        return client.put_file(source, destination, scp_preserve_times, mode, newline,
                               self.config.path_separator)

    def put_directory(self, source, destination='.', mode='0o744', newline='',
                      recursive=False, scp='OFF', scp_preserve_times=False):
        """Calls :py:meth:`AbstractSFTPClient.put_directory` with the given
        arguments and the connection specific path separator.

        The connection specific path separator is set when calling
        :py:meth:`open_connection`.

        See :py:meth:`AbstractSFTPClient.put_directory` for more documentation.
        """
        client = self._create_client(scp)
        return client.put_directory(source, destination, scp_preserve_times, mode,
                                    newline, self.config.path_separator, recursive)

    def get_file(self, source, destination='.', scp='OFF', scp_preserve_times=False):
        """Calls :py:meth:`AbstractSFTPClient.get_file` with the given
        arguments.

        See :py:meth:`AbstractSFTPClient.get_file` for more documentation.
        """
        client = self._create_client(scp)
        if scp == 'ALL':
            sources = self._get_files_for_scp_all(source)
            return client.get_file(sources, destination, scp_preserve_times, self.config.path_separator)
        return client.get_file(source, destination, scp_preserve_times, self.config.path_separator)

    def _get_files_for_scp_all(self, source):
        sources = self.execute_command('printf "%%s\\n" %s' % source)
        result = sources[0].split('\n')
        result[:] = [x for x in result if x]  # remove empty entries
        return result

    def get_directory(self, source, destination='.', recursive=False,
                      scp='OFF', scp_preserve_times=False):
        """Calls :py:meth:`AbstractSFTPClient.get_directory` with the given
        arguments and the connection specific path separator.

        The connection specific path separator is set when calling
        :py:meth:`open_connection`.

        See :py:meth:`AbstractSFTPClient.get_directory` for more documentation.
        """
        client = self._create_client(scp)
        return client.get_directory(source, destination, scp_preserve_times,
                                    self.config.path_separator,
                                    recursive)

    def list_dir(self, path, pattern=None, absolute=False):
        """Calls :py:meth:`.AbstractSFTPClient.list_dir` with the given
        arguments.

        See :py:meth:`AbstractSFTPClient.list_dir` for more documentation.

        :returns: A sorted list of items returned by
            :py:meth:`AbstractSFTPClient.list_dir`.
        """
        items = self.sftp_client.list_dir(path, pattern, absolute)
        return sorted(items)

    def list_files_in_dir(self, path, pattern=None, absolute=False):
        """Calls :py:meth:`AbstractSFTPClient.list_files_in_dir` with the given
        arguments.

        See :py:meth:`AbstractSFTPClient.list_files_in_dir` for more documentation.

        :returns: A sorted list of items returned by
            :py:meth:`AbstractSFTPClient.list_files_in_dir`.
        """
        files = self.sftp_client.list_files_in_dir(path, pattern, absolute)
        return sorted(files)

    def list_dirs_in_dir(self, path, pattern=None, absolute=False):
        """Calls :py:meth:`AbstractSFTPClient.list_dirs_in_dir` with the given
        arguments.

        See :py:meth:`AbstractSFTPClient.list_dirs_in_dir` for more documentation.

        :returns: A sorted list of items returned by
            :py:meth:`AbstractSFTPClient.list_dirs_in_dir`.
        """
        dirs = self.sftp_client.list_dirs_in_dir(path, pattern, absolute)
        return sorted(dirs)

    def is_dir(self, path):
        """Calls :py:meth:`AbstractSFTPClient.is_dir` with the given `path`.

        :param str path: Path to check for directory. Supports GLOB Patterns.

        :returns: Boolean indicating is the directory is present or not.

        :rtype: bool

        See :py:meth:`AbstractSFTPClient.is_dir` for more documentation.
        """
        has_glob = bool([ops for ops in '*?![' if(ops in path)])
        if has_glob:
            dir_dir = path[:(-len(path.split(self.config.path_separator)[-1]))]
            dirs = self.sftp_client.list_dirs_in_dir(dir_dir)
            for dirname in dirs:
                if fnmatch.fnmatch(dirname, path.split(self.config.path_separator)[-1]):
                    return self.sftp_client.is_dir(dir_dir + dirname)
        return self.sftp_client.is_dir(path)

    def is_file(self, path):
        """Calls :py:meth:`AbstractSFTPClient.is_file` with the given `path`.

        :param str path: Path to check for file. Supports GLOB Patterns.

        :returns: Boolean indicating is the file is present or not.

        :rtype: bool

        See :py:meth:`AbstractSFTPClient.is_file` for more documentation.
        """
        if bool([ops for ops in '*?![' if(ops in path)]):
            file_dir = path[:(-len(path.split(self.config.path_separator)[-1]))]
            if file_dir == '':
                return self.sftp_client.is_file(path)
            files = self.sftp_client.list_files_in_dir(file_dir)
            for filename in files:
                if fnmatch.fnmatch(filename, path.split(self.config.path_separator)[-1]):
                    return self.sftp_client.is_file(file_dir + filename)
        return self.sftp_client.is_file(path)

    def _create_client(self, scp):
        if scp.upper() == 'ALL':
            return self.scp_all_client
        elif scp.upper() == 'TRANSFER':
            return self.scp_transfer_client
        else:
            return self.sftp_client


class AbstractShell(object):
    """Base class for the shell implementation.

    Classes derived from this class (i.e. :py:class:`pythonclient.Shell`
    and :py:class:`javaclient.Shell`) provide the concrete and the language
    specific implementations for reading and writing in a shell session.
    """

    def read(self):
        """Reads all the output from the shell.

        :returns: The read output.
        """
        raise NotImplementedError

    def read_byte(self):
        """Reads a single byte from the shell.

        :returns: The read byte.
        """
        raise NotImplementedError

    def write(self, text):
        """Writes the `text` in the current shell.

        :param str text: The text to be written. No newline characters are
            be appended automatically to the written text by this method.
        """
        raise NotImplementedError


class AbstractSFTPClient(object):
    """Base class for the SFTP implementation.

    Classes derived from this class (i.e. :py:class:`pythonclient.SFTPClient`
    and :py:class:`javaclient.SFTPClient`) provide the concrete and the language
    specific implementations for getting, putting and listing files and
    directories.
    """

    def __init__(self, encoding):
        self._encoding = encoding
        self._homedir = self._absolute_path(b'.')

    def _absolute_path(self, path):
        raise NotImplementedError

    def is_file(self, path):
        """Checks if the `path` points to a regular file on the remote host.

        If the `path` is a symlink, its destination is checked instead.

        :param str path: The path to check.

        :returns: `True`, if the `path` is points to an existing regular file.
            False otherwise.
        """
        try:
            item = self._stat(path)
        except IOError:
            return False
        return item.is_regular()

    def _stat(self, path):
        raise NotImplementedError

    def is_dir(self, path):
        """Checks if the `path` points to a directory on the remote host.

        If the `path` is a symlink, its destination is checked instead.

        :param str path: The path to check.

        :returns: `True`, if the `path` is points to an existing directory.
            False otherwise.
        """
        try:
            item = self._stat(path)
        except IOError:
            return False
        return item.is_directory()

    def list_dir(self, path, pattern=None, absolute=False):
        """Gets the item names, or optionally the absolute paths, on the given
        `path` on the remote host.

        This includes regular files, directories as well as other file types,
        e.g. device files.

        :param str path: The path on the remote host to list.

        :param str pattern: If given, only the item names that match
            the given pattern are returned. Please do note, that the `pattern`
            is never matched against the full path, even if `absolute` is set
            `True`.

        :param bool absolute: If `True`, the absolute paths of the items are
            returned instead of the item names.

        :returns: A list containing either the item names or the absolute
            paths. In both cases, the List is first filtered by the `pattern`
            if it is given.
        """
        return self._list_filtered(path, self._get_item_names, pattern,
                                   absolute)

    def _list_filtered(self, path, filter_method, pattern=None, absolute=False):
        self._verify_remote_dir_exists(path)
        items = filter_method(path)
        if pattern:
            items = self._filter_by_pattern(items, pattern)
        if absolute:
            items = self._include_absolute_path(items, path)
        return items

    def _verify_remote_dir_exists(self, path):
        if not self.is_dir(path):
            raise SSHClientException("There was no directory matching '%s'." %
                                     path)

    def _get_item_names(self, path):
        return [item.name for item in self._list(path)]

    def _list(self, path):
        raise NotImplementedError

    def _filter_by_pattern(self, items, pattern):
        return [name for name in items if fnmatchcase(name, pattern)]

    def _include_absolute_path(self, items, path):
        absolute_path = self._absolute_path(path)
        if absolute_path[1:3] == ':\\':
            absolute_path += '\\'
        else:
            absolute_path += '/'
        return [absolute_path + name for name in items]

    def list_files_in_dir(self, path, pattern=None, absolute=False):
        """Gets the file names, or optionally the absolute paths, of the regular
        files on the given `path` on the remote host.
.
        :param str path: The path on the remote host to list.

        :param str pattern: If given, only the file names that match
            the given pattern are returned. Please do note, that the `pattern`
            is never matched against the full path, even if `absolute` is set
            `True`.

        :param bool absolute: If `True`, the absolute paths of the regular files
            are returned instead of the file names.

        :returns: A list containing either the regular file names or the absolute
            paths. In both cases, the List is first filtered by the `pattern`
            if it is given.
        """
        return self._list_filtered(path, self._get_file_names, pattern,
                                   absolute)

    def _get_file_names(self, path):
        return [item.name
                for item in self._list(path)
                if item.is_regular()
                or (item.is_link()
                    and not self._is_dir_symlink(path, item.name))]

    def _is_dir_symlink(self, path, item):
        resolved_link = self._readlink('%s/%s' % (path, item))
        return self.is_dir('%s/%s' % (path, resolved_link))

    def list_dirs_in_dir(self, path, pattern=None, absolute=False):
        """Gets the directory names, or optionally the absolute paths, on the
        given `path` on the remote host.

        :param str path: The path on the remote host to list.

        :param str pattern: If given, only the directory names that match
            the given pattern are returned. Please do note, that the `pattern`
            is never matched against the full path, even if `absolute` is set
            `True`.

        :param bool absolute: If `True`, the absolute paths of the directories
            are returned instead of the directory names.

        :returns: A list containing either the directory names or the absolute
            paths. In both cases, the List is first filtered by the `pattern`
            if it is given.
        """
        return self._list_filtered(path, self._get_directory_names, pattern,
                                   absolute)

    def _get_directory_names(self, path):
        return [item.name for item in self._list(path) if item.is_directory()]

    def get_directory(self, source, destination, scp_preserve_time, path_separator='/',
                      recursive=False):
        destination = self.build_destination(source, destination, path_separator)
        return self._get_directory(source, destination, path_separator, recursive, scp_preserve_time)

    def _get_directory(self, source, destination, path_separator='/',
                       recursive=False, scp_preserve_times=False):
        r"""Downloads directory(-ies) from the remote host to the local machine,
        optionally with subdirectories included.

        :param str source: The path to the directory on the remote machine.

        :param str destination: The target path on the local machine.
            The destination defaults to the current local working directory.

        :param bool scp_preserve_times: preserve modification time and access time
        of transferred files and directories.

        :param str path_separator: The path separator used for joining the
            paths on the remote host. On Windows, this must be set as `\`.
            The default is `/`, which is also the default on Linux-like systems.

        :param bool recursive: If `True`, the subdirectories in the `source`
            path are downloaded as well.

        :returns: A list of 2-tuples for all the downloaded files. These tuples
            contain the remote path as the first value and the local target
            path as the second.
        """
        source = self._remove_ending_path_separator(path_separator, source)
        self._verify_remote_dir_exists(source)
        files = []
        items = self.list_dir(source)
        if items:
            for item in items:
                remote = source + path_separator + item
                local = os.path.join(destination, item)
                if self.is_file(remote):
                    files += self.get_file(remote, local, scp_preserve_times)
                elif recursive:
                    files += self.get_directory(remote, local, scp_preserve_times, path_separator,
                                                recursive)
        else:
            if not os.path.exists(destination):
                os.makedirs(destination)
            files.append((source, destination))
        return files

    def build_destination(self, source, destination, path_separator):
        """Add parent directory from source to destination path if destination is '.'
        or if destination already exists.
        Otherwise the missing intermediate directories are created.

        :return: A new destination path.
        """
        if os.path.exists(destination) or destination == '.':
            fullpath_destination = os.path.join(destination, self.get_parent_folder(source, path_separator))
            if not os.path.exists(fullpath_destination):
                os.makedirs(fullpath_destination)
            return fullpath_destination
        else:
            return destination

    def get_parent_folder(self, source, path_separator):
        if source.endswith(path_separator):
            return (source[:-len(path_separator)]).split(path_separator)[-1]
        else:
            return source.split(path_separator)[-1]

    def _remove_ending_path_separator(self, path_separator, source):
        if source.endswith(path_separator):
            source = source[:-len(path_separator)]
        return source

    def get_file(self, source, destination, scp_preserve_times, path_separator='/'):
        r"""Downloads file(s) from the remote host to the local machine.

        :param str source: Must be the path to an existing file on the remote
            machine or a glob pattern.
            Glob patterns, like '*' and '?', can be used in the source, in
            which case all the matching files are downloaded.

        :param str destination: The target path on the local machine.
            If many files are downloaded, e.g. patterns are used in the
            `source`, then this must be a path to an existing directory.
            The destination defaults to the current local working directory.

        :param bool scp_preserve_times: preserve modification time and access time
        of transferred files and directories.

        :param str path_separator: The path separator used for joining the
            paths on the remote host. On Windows, this must be set as `\`.
            The default is `/`, which is also the default on Linux-like systems.

        :returns: A list of 2-tuples for all the downloaded files. These tuples
            contain the remote path as the first value and the local target
            path as the second.
        """
        remote_files = self._get_get_file_sources(source, path_separator)
        if not remote_files:
            msg = "There were no source files matching '%s'." % source
            raise SSHClientException(msg)
        local_files = self._get_get_file_destinations(remote_files, destination)
        files = list(zip(remote_files, local_files))
        for src, dst in files:
            self._get_file(src, dst, scp_preserve_times)
        return files

    def _get_get_file_sources(self, source, path_separator):
        if path_separator in source:
            path, pattern = source.rsplit(path_separator, 1)
        else:
            path, pattern = '', source
        if not path:
            path = '.'
        if not self.is_file(source):
            return [filename for filename in
                    self.list_files_in_dir(path, pattern, absolute=True)]
        else:
            return [source]

    def _get_get_file_destinations(self, source_files, destination):
        target_is_dir = destination.endswith(os.sep) or destination == '.'
        if not target_is_dir and len(source_files) > 1:
            raise SSHClientException('Cannot copy multiple source files to one '
                                     'destination file.')
        destination = os.path.abspath(destination.replace('/', os.sep))
        self._create_missing_local_dirs(destination, target_is_dir)
        if target_is_dir:
            return [os.path.join(destination, os.path.basename(name))
                    for name in source_files]
        return [destination]

    def _create_missing_local_dirs(self, destination, target_is_dir):
        if not target_is_dir:
            destination = os.path.dirname(destination)
        if not os.path.exists(destination):
            os.makedirs(destination)

    def _get_file(self, source, destination, scp_preserve_times):
        raise NotImplementedError

    def put_directory(self, source, destination, scp_preserve_times,mode, newline,
                      path_separator='/', recursive=False):
        r"""Uploads directory(-ies) from the local machine to the remote host,
        optionally with subdirectories included.

        :param str source: The path to the directory on the local machine.

        :param str destination: The target path on the remote host.
            The destination defaults to the user's home at the remote host.

        :param bool scp_preserve_times: preserve modification time and access time
        of transferred files and directories.

        :param str mode: The uploaded files on the remote host are created with
            these modes. The modes are given as traditional Unix octal
            permissions, such as '0600'.

        :param str newline: If given, the newline characters of the uploaded
            files on the remote host are converted to this.

        :param str path_separator: The path separator used for joining the
            paths on the remote host. On Windows, this must be set as `\`.
            The default is `/`, which is also the default on Linux-like systems.

        :param bool recursive: If `True`, the subdirectories in the `source`
            path are uploaded as well.

        :returns: A list of 2-tuples for all the uploaded files. These tuples
            contain the local path as the first value and the remote target
            path as the second.
        """
        self._verify_local_dir_exists(source)
        destination = self._remove_ending_path_separator(path_separator,
                                                         destination)
        if self.is_dir(destination):
            destination = destination + path_separator +\
                          source.rsplit(os.path.sep)[-1]
        return self._put_directory(source, destination, mode, newline,
                                   path_separator, recursive, scp_preserve_times)

    def _put_directory(self, source, destination, mode, newline,
                       path_separator, recursive, scp_preserve_times=False):
        files = []
        items = os.listdir(source)
        if items:
            for item in items:
                local_path = os.path.join(source, item)
                remote_path = destination + path_separator + item
                if os.path.isfile(local_path):
                    files += self.put_file(local_path, remote_path, scp_preserve_times,
                                           mode, newline, path_separator)
                elif recursive and os.path.isdir(local_path):
                    files += self._put_directory(local_path, remote_path,
                                                 mode, newline,
                                                 path_separator, recursive, scp_preserve_times)
        else:
            self._create_missing_remote_path(destination, mode)
            files.append((source, destination))
        return files

    def _verify_local_dir_exists(self, path):
        if not os.path.isdir(path):
            raise SSHClientException("There was no source path matching '%s'."
                                     % path)

    def put_file(self, sources, destination, scp_preserve_times, mode, newline, path_separator='/'):
        r"""Uploads the file(s) from the local machine to the remote host.

        :param str sources: Must be the path to an existing file on the remote
            machine or a glob pattern .
            Glob patterns, like '*' and '?', can be used in the source, in
            which case all the matching files are uploaded.

        :param str destination: The target path on the remote host.
            If multiple files are uploaded, e.g. patterns are used in the
            `source`, then this must be a path to an existing directory.
            The destination defaults to the user's home at the remote host.

        :param bool scp_preserve_times: preserve modification time and access time
        of transferred files and directories.

        :param str mode: The uploaded files on the remote host are created with
            these modes. The modes are given as traditional Unix octal
            permissions, such as '0600'. If 'None' value is provided,
            setting permissions will be skipped.

        :param str newline: If given, the newline characters of the uploaded
            files on the remote host are converted to this.

        :param str path_separator: The path separator used for joining the
            paths on the remote host. On Windows, this must be set as `\`.
            The default is `/`, which is also the default on Linux-like systems.

        :returns: A list of 2-tuples for all the uploaded files. These tuples
            contain the local path as the first value and the remote target
            path as the second.
        """
        if mode:
            mode = int(mode, 8)
        newline = {'CRLF': '\r\n', 'LF': '\n'}.get(newline.upper(), None)
        local_files = self._get_put_file_sources(sources)
        remote_files, remote_dir = self._get_put_file_destinations(local_files,
                                                                   destination,
                                                                   path_separator)
        self._create_missing_remote_path(remote_dir, mode)
        files = list(zip(local_files, remote_files))
        for source, destination in files:
            self._put_file(source, destination, mode, newline, path_separator, scp_preserve_times)
        return files

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

    def _get_put_file_destinations(self, sources, destination, path_separator):
        if destination[1:3] == ':' + path_separator:
            destination = path_separator + destination
        destination = self._format_destination_path(destination)
        if destination == '.':
            destination = self._homedir + '/'
        if len(sources) > 1 and destination[-1] != '/' and not self.is_dir(destination):
            raise ValueError('It is not possible to copy multiple source '
                             'files to one destination file.')
        dir_path, filename = self._parse_path_elements(destination,
                                                       path_separator)
        if filename:
            files = [path_separator.join([dir_path, filename])]
        else:
            files = [path_separator.join([dir_path, os.path.basename(path)])
                     for path in sources]
        return files, dir_path

    def _format_destination_path(self, destination):
        destination = destination.replace('\\', '/')
        destination = ntpath.splitdrive(destination)[-1]
        return destination

    def _parse_path_elements(self, destination, path_separator):
        def _isabs(path):
            if destination.startswith(path_separator):
                return True
            if path_separator == '\\' and path[1:3] == ':\\':
                return True
            return False
        if not _isabs(destination):
            destination = path_separator.join([self._homedir, destination])
        if self.is_dir(destination):
            return destination, ''
        return destination.rsplit(path_separator, 1)

    def _create_missing_remote_path(self, path, mode):
        if path.startswith(b'/'):
            current_dir = b'/'
        else:
            current_dir = self._absolute_path(b'.').encode(self._encoding)
        for dir_name in path.split(b'/'):
            if dir_name:
                current_dir = posixpath.join(current_dir, dir_name)
            try:
                self._client.stat(current_dir)
            except:
                if not isinstance(mode, int):
                    mode = int(mode, 8)
                self._client.mkdir(current_dir, mode)

    def _put_file(self, source, destination, mode, newline, path_separator, scp_preserve_times=False):
        remote_file = self._create_remote_file(destination, mode)
        with open(source, 'rb') as local_file:
            position = 0
            while True:
                data = local_file.read(4096)
                if not data:
                    break
                if newline:
                    data = re.sub(br'(\r\n|\r|\n)', newline.encode(self._encoding), data)
                self._write_to_remote_file(remote_file, data, position)
                position += len(data)
            self._close_remote_file(remote_file)

    def _create_remote_file(self, destination, mode):
        raise NotImplementedError

    def _write_to_remote_file(self, remote_file, data, position):
        raise NotImplementedError

    def _close_remote_file(self, remote_file):
        raise NotImplementedError

    def create_local_ssh_tunnel(self, local_port, remote_host, remote_port, client):
        raise NotImplementedError

    def _readlink(self, path):
        raise NotImplementedError


class AbstractCommand(object):
    """Base class for the remote command.

    Classes derived from this class (i.e. :py:class:`pythonclient.RemoteCommand`
    and :py:class:`javaclient.RemoteCommand`) provide the concrete and the
    language specific implementations for running the command on the remote
    host.
    """

    def __init__(self, command, encoding):
        self._command = command
        self._encoding = encoding
        self._shell = None

    def run_in(self, shell, sudo=False,  sudo_password=None, invoke_subsystem=False):
        """Runs this command in the given `shell`.

        :param shell: A shell in the already open connection.

        :param sudo
         and
        :param sudo_password are used for executing commands within a sudo session.

        :param invoke_subsystem will request a subsystem on the server.
        """
        self._shell = shell
        if invoke_subsystem:
            self._invoke()
        elif not sudo:
            self._execute()
        else:
            self._execute_with_sudo(sudo_password)

    def _execute(self):
        raise NotImplementedError

    def _invoke(self):
        raise NotImplementedError

    def _execute_with_sudo(self, sudo_password=None):
        raise NotImplementedError

    def read_outputs(self):
        """Returns the outputs of this command.

        :returns: A 3-tuple (stdout, stderr, return_code) with values
            `stdout` and `stderr` as strings and `return_code` as an integer.
        """
        raise NotImplementedError


class SFTPFileInfo(object):
    """Wrapper class for the language specific file information objects.

    Returned by the concrete SFTP client implementations.
    """

    def __init__(self, name, mode):
        self.name = name
        self.mode = mode

    def is_regular(self):
        """Checks if this file is a regular file.

        :returns: `True`, if the file is a regular file. False otherwise.
        """
        return stat.S_ISREG(self.mode)

    def is_directory(self):
        """Checks if this file is a directory.

        :returns: `True`, if the file is a regular file. False otherwise.
        """
        return stat.S_ISDIR(self.mode)

    def is_link(self):
        """Checks if this file is a symbolic link.

        :returns: `True`, if the file is a symlink file. False otherwise.
        """
        return stat.S_ISLNK(self.mode)
